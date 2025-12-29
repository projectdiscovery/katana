package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/adrianbrad/queue"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/rod/lib/utils"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/diagnostics"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer/simhash"
	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/projectdiscovery/katana/pkg/output"
)

type Crawler struct {
	logger        *slog.Logger
	launcher      *browser.Launcher
	options       Options
	crawlQueue    queue.Queue[*types.Action]
	crawlGraph    *graph.CrawlGraph
	simhashOracle *simhash.Oracle
	uniqueActions map[string]struct{}
	diagnostics   diagnostics.Writer
}

type Options struct {
	ChromiumPath        string
	MaxBrowsers         int
	MaxDepth            int
	PageMaxTimeout      time.Duration
	NoSandbox           bool
	ShowBrowser         bool
	SlowMotion          bool
	MaxCrawlDuration    time.Duration
	MaxFailureCount     int
	Trace               bool
	CookieConsentBypass bool
	AutomaticFormFill   bool

	// EnableDiagnostics enables the diagnostics mode
	// which writes diagnostic information to a directory
	// specified by the DiagnosticsDir optionally.
	EnableDiagnostics bool
	DiagnosticsDir    string

	Logger          *slog.Logger
	ScopeValidator  browser.ScopeValidator
	RequestCallback func(*output.Result)
	ChromeUser      *user.User
}

var domNormalizer *normalizer.Normalizer
var initOnce sync.Once
var initError error

func init() {
	initOnce.Do(func() {
		var err error
		domNormalizer, err = normalizer.New()
		if err != nil {
			initError = errors.Wrap(err, "failed to create domnormalizer")
		}
	})
}

func New(opts Options) (*Crawler, error) {
	if initError != nil {
		return nil, initError
	}

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	launcher, err := browser.NewLauncher(browser.LauncherOptions{
		ChromiumPath:        opts.ChromiumPath,
		MaxBrowsers:         opts.MaxBrowsers,
		PageMaxTimeout:      opts.PageMaxTimeout,
		ShowBrowser:         opts.ShowBrowser,
		RequestCallback:     opts.RequestCallback,
		SlowMotion:          opts.SlowMotion,
		ScopeValidator:      opts.ScopeValidator,
		ChromeUser:          opts.ChromeUser,
		Trace:               opts.Trace,
		CookieConsentBypass: opts.CookieConsentBypass,
		NoSandbox:           opts.NoSandbox,
	})
	if err != nil {
		return nil, err
	}

	var diagnosticsWriter diagnostics.Writer
	if opts.EnableDiagnostics {
		directory := opts.DiagnosticsDir
		if directory == "" {
			cwd, _ := os.Getwd()
			directory = filepath.Join(cwd, fmt.Sprintf("katana-diagnostics-%s", time.Now().Format(time.RFC3339)))
		}

		writer, err := diagnostics.NewWriter(directory)
		if err != nil {
			return nil, err
		}
		diagnosticsWriter = writer
		opts.DiagnosticsDir = directory
		opts.Logger.Info("Diagnostics enabled", slog.String("directory", directory))
	}

	crawler := &Crawler{
		launcher:      launcher,
		options:       opts,
		logger:        opts.Logger,
		uniqueActions: make(map[string]struct{}),
		diagnostics:   diagnosticsWriter,
		simhashOracle: simhash.NewOracle(),
	}
	return crawler, nil
}

func (c *Crawler) Close() {
	c.launcher.Close()
	if c.diagnostics != nil {
		if err := c.diagnostics.Close(); err != nil {
			c.logger.Warn("Failed to close diagnostics", slog.String("error", err.Error()))
		}
	}
}

func (c *Crawler) GetCrawlGraph() *graph.CrawlGraph {
	return c.crawlGraph
}

func (c *Crawler) Crawl(URL string) error {
	defer func() {
		if c.diagnostics == nil {
			return
		}
		err := c.crawlGraph.DrawGraph(filepath.Join(c.options.DiagnosticsDir, "crawl-graph.dot"))
		if err != nil {
			c.logger.Error("Failed to draw crawl graph", slog.String("error", err.Error()))
		}
	}()

	actions := []*types.Action{{
		Type:     types.ActionTypeLoadURL,
		Input:    URL,
		Depth:    0,
		OriginID: emptyPageHash,
	}}

	crawlQueue := queue.NewLinked(actions)
	c.crawlQueue = crawlQueue

	crawlGraph := graph.NewCrawlGraph()
	c.crawlGraph = crawlGraph

	// Add the initial blank state
	err := crawlGraph.AddPageState(types.PageState{
		UniqueID: emptyPageHash,
		URL:      "about:blank",
		Depth:    0,
	})
	if err != nil {
		return err
	}

	// Create a master context that will automatically cancel all page operations
	// once the per-URL crawl deadline is reached.
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if c.options.MaxCrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), c.options.MaxCrawlDuration)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// Retain the legacy time.After guard as a secondary fail-safe but the
	// context cancellation is what actually stops in-flight rod calls.
	var crawlTimeout <-chan time.Time
	if c.options.MaxCrawlDuration > 0 {
		crawlTimeout = time.After(c.options.MaxCrawlDuration)
	}

	consecutiveFailures := 0

	for {
		select {
		case <-crawlTimeout:
			c.logger.Debug("Max crawl duration reached, stopping crawl")
			return nil
		default:
			// Check for too many failures
			if c.options.MaxFailureCount > 0 && consecutiveFailures >= c.options.MaxFailureCount {
				c.logger.Warn("Too many consecutive failures, stopping crawl",
					slog.Int("failures", consecutiveFailures),
					slog.Int("max_allowed", c.options.MaxFailureCount),
					slog.Int("remaining_actions", c.crawlQueue.Size()),
				)
				return nil
			}

			action, err := crawlQueue.Get()
			if err == queue.ErrNoElementsAvailable {
				c.logger.Debug("No more actions to process")
				return nil
			}
			if err != nil {
				return err
			}

			if c.options.MaxDepth > 0 && action.Depth > c.options.MaxDepth {
				continue
			}

			page, err := c.launcher.GetPageFromPool()
			if err != nil {
				return err
			}

			page.Page = page.Context(ctx)

			c.logger.Debug("Processing action",
				slog.String("action", action.String()),
			)

			if err := c.crawlFn(action, page); err != nil {
				if err == ErrNoCrawlingAction {
					return nil
				}
				if errors.Is(err, ErrElementNotVisible) {
					consecutiveFailures++
					continue
				}
				var npe *rod.NoPointerEventsError
				var ish *rod.InvisibleShapeError
				if errors.As(err, &npe) || errors.As(err, &ish) {
					c.logger.Debug("Skipping action as it is not visible",
						slog.String("action", action.String()),
						slog.String("error", err.Error()),
					)
					consecutiveFailures++
					continue
				}
				var ne *rod.NavigationError
				if errors.As(err, &ne) {
					c.logger.Debug("Skipping action as navigation failed",
						slog.String("action", action.String()),
						slog.String("error", err.Error()),
					)
					consecutiveFailures++
					continue
				}
				if errors.Is(err, ErrNoNavigationPossible) {
					c.logger.Debug("Skipping action as no navigation possible", slog.String("action", action.String()))
					consecutiveFailures++
					continue
				}
				var msce *utils.MaxSleepCountError
				if errors.As(err, &msce) {
					c.logger.Debug("Skipping action as it is taking too long", slog.String("action", action.String()))
					consecutiveFailures++
					continue
				}

				c.logger.Debug("Skipping action due to site-specific error",
					slog.String("error", err.Error()),
					slog.String("action", action.String()),
				)
				consecutiveFailures++
				continue
			}

			consecutiveFailures = 0
		}
	}
}

var ErrNoCrawlingAction = errors.New("no more actions to crawl")

func (c *Crawler) crawlFn(action *types.Action, page *browser.BrowserPage) error {
	defer func() {
		c.launcher.PutBrowserToPool(page)
	}()

	currentPageHash, _, err := getPageHash(page)
	if err != nil {
		return err
	}

	c.logger.Debug("Processing action - current state",
		slog.String("current_page_hash", currentPageHash),
		slog.String("action_origin_id", action.OriginID),
		slog.String("action", action.String()),
	)

	if action.OriginID != "" && action.OriginID != currentPageHash {
		c.logger.Debug("Need to navigate back to origin",
			slog.String("from", currentPageHash),
			slog.String("to", action.OriginID),
		)
		newPageHash, err := c.navigateBackToStateOrigin(action, page, currentPageHash)
		if err != nil {
			return err
		}
		// Refresh the page hash
		currentPageHash = newPageHash
	}

	// FIXME: TODO: Restrict the navigation using scope manager and only
	// proceed with actions if the scope is allowed

	// Check the action and do actions based on action type
	if c.diagnostics != nil {
		if err := c.diagnostics.LogAction(action); err != nil {
			return err
		}
	}
	if err := c.executeCrawlStateAction(action, page); err != nil {
		return err
	}

	pageState, err := newPageState(page, action)
	if err != nil {
		return err
	}
	if c.diagnostics != nil {
		if err := c.diagnostics.LogPageState(pageState, diagnostics.PostActionPageState); err != nil {
			return err
		}
	}
	pageState.OriginID = currentPageHash

	if c.options.ScopeValidator != nil {
		if !c.options.ScopeValidator(pageState.URL) {
			c.logger.Debug("Skipping navigation collection - current page is out of scope",
				slog.String("url", pageState.URL),
			)
			if c.crawlQueue.Size() == 0 {
				return ErrNoCrawlingAction
			}
			return nil
		}
	}

	navigations, err := page.FindNavigations()
	if err != nil {
		return err
	}

	// Log navigations for diagnostics
	if c.diagnostics != nil {
		screenshotState, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
			Format: proto.PageCaptureScreenshotFormatPng,
		})
		if err != nil {
			c.logger.Error("Failed to take screenshot", slog.String("error", err.Error()))
		}
		if err := c.diagnostics.LogPageStateScreenshot(pageState.UniqueID, screenshotState); err != nil {
			c.logger.Error("Failed to log page state screenshot", slog.String("error", err.Error()))
		}
		if err := c.diagnostics.LogNavigations(pageState.UniqueID, navigations); err != nil {
			c.logger.Error("Failed to log navigations", slog.String("error", err.Error()))
		}
	}

	for _, nav := range navigations {
		actionHash := nav.Hash()
		if _, ok := c.uniqueActions[actionHash]; ok {
			continue
		}
		c.uniqueActions[actionHash] = struct{}{}

		// Check if the element we have is a logout page
		if nav.Element != nil && isLogoutPage(nav.Element) {
			c.logger.Debug("Skipping Found logout page",
				slog.String("url", nav.Element.Attributes["href"]),
			)
			continue
		}
		nav.OriginID = pageState.UniqueID

		c.logger.Debug("Got new navigation",
			slog.Any("navigation", nav),
		)
		if err := c.crawlQueue.Offer(nav); err != nil {
			return err
		}
	}

	err = c.crawlGraph.AddPageState(*pageState)
	if err != nil {
		return err
	}

	// TODO: Check if the page opened new sub pages and if so capture their
	// navigation as well as close them so the state change can work.

	if len(navigations) == 0 && c.crawlQueue.Size() == 0 {
		return ErrNoCrawlingAction
	}
	return nil
}

var ErrElementNotVisible = errors.New("element not visible")

func (c *Crawler) executeCrawlStateAction(action *types.Action, page *browser.BrowserPage) error {
	var err error
	switch action.Type {
	case types.ActionTypeLoadURL:
		// Apply a timeout to every critical Rod call.
		pTimeout := page.Timeout(c.options.PageMaxTimeout)

		if err := pTimeout.Navigate(action.Input); err != nil {
			return err
		}
		if err = page.WaitPageLoadHeurisitics(); err != nil {
			return err
		}
	case types.ActionTypeFillForm:
		if err := c.processForm(page, action.Form); err != nil {
			return err
		}
	case types.ActionTypeLeftClick, types.ActionTypeLeftClickDown:
		pTimeout := page.Timeout(c.options.PageMaxTimeout)
		element, err := pTimeout.ElementX(action.Element.XPath)
		if err != nil {
			return err
		}

		elementTimeout := element.Timeout(c.options.PageMaxTimeout)
		if err := elementTimeout.ScrollIntoView(); err != nil {
			return err
		}
		visible, err := element.Visible()
		if err != nil {
			return err
		}
		if !visible {
			return ErrElementNotVisible
		}

		// Check if element is interactable (not blocked by overlays)
		interactable, err := element.Interactable()
		if err != nil {
			var ce *rod.CoveredError
			if errors.As(err, &ce) {
				return ErrElementNotVisible
			}
			return err
		}
		if interactable == nil {
			return ErrElementNotVisible
		}

		if err := element.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return err
		}
		if err = page.WaitPageLoadHeurisitics(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown action type: %v", action.Type)
	}
	return nil
}

var logoutPattern = regexp.MustCompile(`(?i)(log[\s-]?out|sign[\s-]?out|signout|deconnexion|cerrar[\s-]?sesion|sair|abmelden|uitloggen|ausloggen|exit|disconnect|terminate|end[\s-]?session|salir|desconectar|afmelden|wyloguj|logout|sign[\s-]?off)`)

func isLogoutPage(element *types.HTMLElement) bool {
	return logoutPattern.MatchString(element.TextContent) ||
		logoutPattern.MatchString(element.Attributes["href"])
}
