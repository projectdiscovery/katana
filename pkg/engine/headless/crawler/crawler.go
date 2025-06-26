package crawler

import (
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
	ShowBrowser         bool
	SlowMotion          bool
	MaxCrawlDuration    time.Duration
	MaxFailureCount     int
	Trace               bool
	CookieConsentBypass bool

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

func init() {
	initOnce.Do(func() {
		var err error
		domNormalizer, err = normalizer.New()
		if err != nil {
			panic(err)
		}
	})
}

func New(opts Options) (*Crawler, error) {
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
		c.diagnostics.Close()
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

			c.logger.Debug("Processing action",
				slog.String("action", action.String()),
			)

			if err := c.crawlFn(action, page); err != nil {
				consecutiveFailures++
				if err == ErrNoCrawlingAction {
					return nil
				}
				if errors.Is(err, ErrElementNotVisible) {
					continue
				}
				if errors.Is(err, &rod.NoPointerEventsError{}) || errors.Is(err, &rod.InvisibleShapeError{}) {
					c.logger.Debug("Skipping action as it is not visible",
						slog.String("action", action.String()),
						slog.String("error", err.Error()),
					)
					continue
				}
				if errors.Is(err, &rod.NavigationError{}) {
					c.logger.Debug("Skipping action as navigation failed",
						slog.String("action", action.String()),
						slog.String("error", err.Error()),
					)
					continue
				}
				if errors.Is(err, ErrNoNavigationPossible) {
					c.logger.Debug("Skipping action as no navigation possible", slog.String("action", action.String()))
					continue
				}
				if errors.Is(err, &utils.MaxSleepCountError{}) {
					c.logger.Debug("Skipping action as it is taking too long", slog.String("action", action.String()))
					continue
				}
				c.logger.Error("Error processing action",
					slog.String("error", err.Error()),
					slog.String("action", action.String()),
				)
				return err
			}

			// Reset consecutive failures on success
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
		if err := page.Navigate(action.Input); err != nil {
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
		element, err := page.ElementX(action.Element.XPath)
		if err != nil {
			return err
		}
		if err := element.ScrollIntoView(); err != nil {
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
			if errors.Is(err, &rod.CoveredError{}) {
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

var logoutPattern = regexp.MustCompile(`(?i)(log[\s-]?out|sign[\s-]?out|signout|deconnexion|cerrar[\s-]?sesion|sair|abmelden|uitloggen|exit|disconnect|terminate|end[\s-]?session|salir|desconectar|auc.loggergen|afmelden|wyloguj|logout|sign[\s-]?off)`)

func isLogoutPage(element *types.HTMLElement) bool {
	return logoutPattern.MatchString(element.TextContent) ||
		logoutPattern.MatchString(element.Attributes["href"])
}

var formFillingData = map[string]string{
	"text":     "test",
	"number":   "5",
	"password": "test",
	"email":    "test@test.com",
}

func (c *Crawler) processForm(page *browser.BrowserPage, form *types.HTMLForm) error {
	var err error

	var submitButtonFinal *rod.Element
	for _, field := range form.Elements {
		var element *rod.Element
		if field.XPath != "" {
			if element, err = page.ElementX(field.XPath); err != nil {
				return err
			}
		}

		switch field.TagName {
		case "INPUT":
			var inputValue string
			switch field.Type {
			case "text":
				inputValue = formFillingData["text"]
			case "number":
				inputValue = formFillingData["number"]
			case "password":
				inputValue = formFillingData["password"]
			case "email":
				inputValue = formFillingData["email"]
			}
			if err := element.Input(inputValue); err != nil {
				return err
			}
		case "TEXTAREA":

		case "BUTTON":
			if submitButtonFinal == nil && field.Type == "submit" {
				submitButtonFinal = element
			}
		}
	}
	if submitButtonFinal != nil {
		if err := submitButtonFinal.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return err
		}
	}
	return nil
}
