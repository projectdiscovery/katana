package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adrianbrad/queue"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/rod/lib/utils"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
	"github.com/projectdiscovery/katana/pkg/engine/headless/explorer"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/diagnostics"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer/simhash"
	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/projectdiscovery/katana/pkg/output"
	katanautils "github.com/projectdiscovery/katana/pkg/utils"
)

type Crawler struct {
	logger        *slog.Logger
	launcher      *browser.Launcher
	options       Options
	crawlQueue    *AffinityQueue
	lastPageHash  atomic.Value // stores string — page hash after last successful crawlFn
	crawlGraph    *graph.CrawlGraph
	simhashOracle *simhash.Oracle
	uniqueActions safeActionSet
	diagnostics   diagnostics.Writer

	crawlDeadline   time.Time        // when the crawl budget expires (zero if no limit)
	coverageTracker *CoverageTracker // nil when CoverageGuided is false

	// templateYield tracks how many new navigations each URL template has produced.
	// Uses utils.FingerprintURL (which recognizes UUIDs, hashes, numeric IDs, etc.)
	// to group similar URLs. After a template yields 0 new navs across multiple visits,
	// further instances are skipped to avoid thrashing on identical page templates.
	templateYield   map[string]*templateStats
	templateYieldMu sync.Mutex

	// originStrikes tracks consecutive zero-yield actions per origin page.
	// After 3 consecutive zero-yield actions, the origin is marked exhausted
	// and its remaining actions get deprioritized (+50) in the queue.
	originStrikes    map[string]int
	exhaustedOrigins map[string]bool
	originStrikesMu  sync.Mutex
}

// templateStats tracks crawl yield for a URL template.
type templateStats struct {
	visits       int
	totalNewNavs int
}

// urlFingerprint returns a structural fingerprint of the URL using the existing
// pattern recognition (UUIDs, hashes, numeric IDs, slugs, base64 tokens, etc.).
func urlFingerprint(rawURL string) string {
	return katanautils.FingerprintURL(rawURL, nil)
}

// isTemplatedURL returns true if the URL contains variable segments
// (i.e. the fingerprint differs from the original URL).
func isTemplatedURL(rawURL string) bool {
	return urlFingerprint(rawURL) != rawURL
}

// shouldSkipTemplate returns true if this URL template has been visited enough
// times with diminishing returns (0 new navigations found).
func (c *Crawler) shouldSkipTemplate(rawURL string) bool {
	if !isTemplatedURL(rawURL) {
		return false
	}
	tmpl := urlFingerprint(rawURL)

	c.templateYieldMu.Lock()
	defer c.templateYieldMu.Unlock()

	stats := c.templateYield[tmpl]
	if stats == nil {
		return false
	}
	// Skip after first zero-yield visit — template pages are structurally identical
	if stats.visits >= 1 && stats.totalNewNavs == 0 {
		return true
	}
	// Also skip if yield rate is very low (< 1 new nav per visit after 2+ visits)
	if stats.visits >= 2 && float64(stats.totalNewNavs)/float64(stats.visits) < 1.0 {
		return true
	}
	// Coverage-guided exhaustion: if last 2+ visits triggered zero new JS code
	if c.coverageTracker != nil && c.coverageTracker.IsTemplateCoverageExhausted(rawURL) {
		c.logger.Info("[coverage] skipping template, zero coverage gain",
			slog.String("template", urlFingerprint(rawURL)),
		)
		return true
	}
	return false
}

// recordTemplateYield records how many new navigations a page visit produced.
func (c *Crawler) recordTemplateYield(rawURL string, newNavs int) {
	if !isTemplatedURL(rawURL) {
		return
	}
	tmpl := urlFingerprint(rawURL)

	c.templateYieldMu.Lock()
	defer c.templateYieldMu.Unlock()

	stats := c.templateYield[tmpl]
	if stats == nil {
		stats = &templateStats{}
		c.templateYield[tmpl] = stats
	}
	stats.visits++
	stats.totalNewNavs += newNavs
}

const originExhaustionThreshold = 3

// recordOriginYield records yield for an action from a given origin page.
// An action is productive if it discovered new navigations (DOM elements)
// OR triggered new HTTP requests (API calls captured by RequestCallback).
// Coverage gain is excluded — UI interactions trigger JS without real discovery.
func (c *Crawler) recordOriginYield(originID string, newNavs int, newRequests int) {
	c.originStrikesMu.Lock()
	defer c.originStrikesMu.Unlock()

	if newNavs > 0 || newRequests > 0 {
		c.originStrikes[originID] = 0
		delete(c.exhaustedOrigins, originID)
		return
	}

	c.originStrikes[originID]++
	if c.originStrikes[originID] >= originExhaustionThreshold {
		c.exhaustedOrigins[originID] = true
	}
}

// isOriginExhausted returns true if the origin has had 3+ consecutive
// zero-yield actions and should be deprioritized.
func (c *Crawler) isOriginExhausted(originID string) bool {
	c.originStrikesMu.Lock()
	defer c.originStrikesMu.Unlock()

	return c.exhaustedOrigins[originID]
}

// safeActionSet is a thread-safe set for deduplicating crawl actions.
type safeActionSet struct {
	mu sync.Mutex
	m  map[string]struct{}
}

// Add returns true if the hash was new (added), false if already seen.
func (s *safeActionSet) Add(hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[hash]; ok {
		return false
	}
	s.m[hash] = struct{}{}
	return true
}

// Len returns the number of unique actions seen.
func (s *safeActionSet) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

type Options struct {
	ChromiumPath        string
	MaxBrowsers         int
	MaxDepth            int
	PageMaxTimeout      time.Duration
	NoSandbox           bool
	NoIncognito         bool
	ShowBrowser         bool
	SlowMotion          bool
	MaxCrawlDuration    time.Duration
	MaxFailureCount     int
	Trace               bool
	CookieConsentBypass bool
	AutomaticFormFill   bool
	PageLoadStrategy    string
	ChromeWSUrl         string
	DOMWaitTime         int
	CoverageGuided      bool
	UserDataDir         string

	// EnableDiagnostics enables the diagnostics mode
	// which writes diagnostic information to a directory
	// specified by the DiagnosticsDir optionally.
	EnableDiagnostics bool
	DiagnosticsDir    string

	Proxy           string
	Logger          *slog.Logger
	ScopeValidator  browser.ScopeValidator
	RequestCallback func(*output.Result)
	ChromeUser      *user.User
	CaptchaHandler  *captcha.Handler

	// Authentication
	SessionState     *auth.SessionState
	AuthConfig       *auth.AuthConfig
	AuthOrchestrator *auth.Orchestrator

	// RequestYieldCounter is incremented by the RequestCallback for each new
	// unique URL seen. crawlFn reads the delta before/after action execution
	// to measure request-level yield (API calls triggered by page visits).
	RequestYieldCounter *atomic.Int64

	// AI Exploration
	ExplorationPlanner *explorer.Planner
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
		NoIncognito:         opts.NoIncognito,
		PageLoadStrategy:    opts.PageLoadStrategy,
		ChromeWSUrl:         opts.ChromeWSUrl,
		DOMWaitTime:         opts.DOMWaitTime,
		UserDataDir:         opts.UserDataDir,
		Proxy:               opts.Proxy,
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
		launcher:         launcher,
		options:          opts,
		logger:           opts.Logger,
		uniqueActions:    safeActionSet{m: make(map[string]struct{})},
		diagnostics:      diagnosticsWriter,
		simhashOracle:    simhash.NewOracle(),
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}
	if opts.CoverageGuided {
		crawler.coverageTracker = NewCoverageTracker()
		opts.Logger.Info("[crawl] coverage-guided mode enabled")
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

// GetPageFromLauncher borrows a browser page from the crawler's own launcher pool.
// Used for authentication on the same browser instance the crawler will use.
func (c *Crawler) GetPageFromLauncher() (*browser.BrowserPage, error) {
	return c.launcher.GetPageFromPool()
}

// ReturnPageToLauncher returns a borrowed page back to the launcher pool.
func (c *Crawler) ReturnPageToLauncher(page *browser.BrowserPage) {
	c.launcher.PutBrowserToPool(page)
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

	c.templateYield = make(map[string]*templateStats)
	c.originStrikes = make(map[string]int)
	c.exhaustedOrigins = make(map[string]bool)

	actions := []*types.Action{{
		Type:     types.ActionTypeLoadURL,
		Input:    URL,
		Depth:    0,
		OriginID: emptyPageHash,
	}}

	priorityQueue := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})
	c.crawlQueue = NewAffinityQueue(priorityQueue, c.shouldSkipTemplate, c.isOriginExhausted)

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

	// Note: When auth is configured, authentication happens on this crawler's
	// browser BEFORE Crawl() is called (in headless.go). Cookies are already
	// in the browser context — no CDP cookie transfer needed.

	// Create a master context that will automatically cancel all page operations
	// once the per-URL crawl deadline is reached.
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if c.options.MaxCrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), c.options.MaxCrawlDuration)
		c.crawlDeadline = time.Now().Add(c.options.MaxCrawlDuration)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	numWorkers := c.options.MaxBrowsers
	if numWorkers <= 0 {
		numWorkers = 1
	}

	if numWorkers == 1 {
		return c.crawlSingleThreaded(ctx)
	}
	return c.crawlParallel(ctx, numWorkers)
}

// crawlSingleThreaded is the legacy single-threaded crawl loop.
// Used when headless-concurrency is 1 (default).
func (c *Crawler) crawlSingleThreaded(ctx context.Context) error {
	var crawlTimeout <-chan time.Time
	if c.options.MaxCrawlDuration > 0 {
		crawlTimeout = time.After(c.options.MaxCrawlDuration)
	}

	consecutiveFailures := 0
	for {
		select {
		case <-crawlTimeout:
			c.logger.Info("[crawl] max crawl duration reached, stopping",
				slog.Int("total_unique_actions", c.uniqueActions.Len()),
				slog.Int("queue_remaining", c.crawlQueue.Size()),
			)
			return nil
		default:
			if c.options.MaxFailureCount > 0 && consecutiveFailures >= c.options.MaxFailureCount {
				c.logger.Warn("Too many consecutive failures, stopping crawl",
					slog.Int("failures", consecutiveFailures),
					slog.Int("max_allowed", c.options.MaxFailureCount),
					slog.Int("remaining_actions", c.crawlQueue.Size()),
				)
				return nil
			}

			// Use origin-affinity: prefer actions from the current page to avoid navigate-back
			lastHash, _ := c.lastPageHash.Load().(string)
			action, err := c.crawlQueue.GetPreferring(lastHash, 20)
			if err == queue.ErrNoElementsAvailable {
				c.logger.Info("[crawl] queue empty, crawl complete",
					slog.Int("total_unique_actions", c.uniqueActions.Len()),
				)
				return nil
			}
			if err != nil {
				return err
			}

			if c.options.MaxDepth > 0 && action.Depth > c.options.MaxDepth {
				c.logger.Debug("[crawl] depth limit exceeded, skipping",
					slog.Int("action_depth", action.Depth),
					slog.Int("max_depth", c.options.MaxDepth),
					slog.String("action", action.String()),
				)
				continue
			}

			page, err := c.launcher.GetPageFromPool()
			if err != nil {
				return err
			}

			page.Page = page.Context(ctx)

			c.logger.Info("[crawl] processing action",
				slog.Int("depth", action.Depth),
				slog.String("type", string(action.Type)),
				slog.String("action", action.String()),
				slog.Int("queue_remaining", c.crawlQueue.Size()),
			)

			if err := c.crawlFn(ctx, action, page); err != nil {
				if err == ErrNoCrawlingAction {
					c.logger.Info("[crawl] no more actions to crawl, stopping",
						slog.Int("total_unique_actions", c.uniqueActions.Len()),
					)
					return nil
				}
				consecutiveFailures += c.handleCrawlError(err, action)
				continue
			}
			consecutiveFailures = 0
		}
	}
}

// crawlParallel runs N concurrent workers, each processing actions from the shared queue.
func (c *Crawler) crawlParallel(ctx context.Context, numWorkers int) error {
	var (
		wg              sync.WaitGroup
		activeWorkers   atomic.Int32
		failures        atomic.Int32
	)

	c.logger.Info("Starting parallel crawl",
		slog.Int("workers", numWorkers),
	)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			c.crawlWorker(ctx, workerID, &activeWorkers, &failures)
		}(i)
	}

	wg.Wait()
	return nil
}

func (c *Crawler) crawlWorker(ctx context.Context, id int, activeWorkers *atomic.Int32, failures *atomic.Int32) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if c.options.MaxFailureCount > 0 && int(failures.Load()) >= c.options.MaxFailureCount {
			return
		}

		lastHash, _ := c.lastPageHash.Load().(string)
		action, err := c.crawlQueue.GetPreferring(lastHash, 20)
		if err == queue.ErrNoElementsAvailable {
			// No work — check if other workers are still active
			if activeWorkers.Load() == 0 && c.crawlQueue.Size() == 0 {
				return
			}
			time.Sleep(300 * time.Millisecond)
			continue
		}
		if err != nil {
			return
		}

		if c.options.MaxDepth > 0 && action.Depth > c.options.MaxDepth {
			c.logger.Debug("[crawl] depth limit exceeded, skipping",
				slog.Int("worker", id),
				slog.Int("action_depth", action.Depth),
				slog.Int("max_depth", c.options.MaxDepth),
			)
			continue
		}

		activeWorkers.Add(1)

		page, err := c.launcher.GetPageFromPool()
		if err != nil {
			activeWorkers.Add(-1)
			c.logger.Debug("Worker failed to get page",
				slog.Int("worker", id),
				slog.String("error", err.Error()),
			)
			continue
		}

		page.Page = page.Context(ctx)

		c.logger.Info("[crawl] processing action",
			slog.Int("worker", id),
			slog.Int("depth", action.Depth),
			slog.String("type", string(action.Type)),
			slog.String("action", action.String()),
			slog.Int("queue_remaining", c.crawlQueue.Size()),
		)

		if err := c.crawlFn(ctx, action, page); err != nil {
			if err == ErrNoCrawlingAction {
				activeWorkers.Add(-1)
				return
			}
			c.handleCrawlError(err, action)
			failures.Add(1)
			activeWorkers.Add(-1)
			continue
		}

		failures.Store(0)
		activeWorkers.Add(-1)
	}
}

// handleCrawlError logs the error and returns 1 (failure count increment).
func (c *Crawler) handleCrawlError(err error, action *types.Action) int {
	if errors.Is(err, ErrElementNotVisible) {
		c.logger.Warn("[crawl] action failed: element not visible",
			slog.String("action", action.String()),
		)
		return 1
	}
	var npe *rod.NoPointerEventsError
	var ish *rod.InvisibleShapeError
	if errors.As(err, &npe) || errors.As(err, &ish) {
		c.logger.Warn("[crawl] action failed: not interactable",
			slog.String("action", action.String()),
			slog.String("error", err.Error()),
		)
		return 1
	}
	var ne *rod.NavigationError
	if errors.As(err, &ne) {
		c.logger.Warn("[crawl] action failed: navigation error",
			slog.String("action", action.String()),
			slog.String("error", err.Error()),
		)
		return 1
	}
	if errors.Is(err, ErrNoNavigationPossible) {
		c.logger.Warn("[crawl] action failed: no navigation possible", slog.String("action", action.String()))
		return 1
	}
	var msce *utils.MaxSleepCountError
	if errors.As(err, &msce) {
		c.logger.Warn("[crawl] action failed: timeout (max sleep count)",
			slog.String("action", action.String()),
		)
		return 1
	}
	c.logger.Warn("[crawl] action failed",
		slog.String("error", err.Error()),
		slog.String("action", action.String()),
	)
	return 1
}

var ErrNoCrawlingAction = errors.New("no more actions to crawl")

func (c *Crawler) crawlFn(ctx context.Context, action *types.Action, page *browser.BrowserPage) error {
	crawlStart := time.Now()
	defer func() {
		c.launcher.PutBrowserToPool(page)
	}()

	// Template dedup: skip actions targeting URLs whose template has been visited
	// enough times with 0 new discoveries (e.g. /task/{uuid} after 3 visits with no new navs).
	actionURL := action.Input
	if actionURL == "" && action.Element != nil {
		actionURL = action.Element.Attributes["href"]
	}
	if actionURL != "" && c.shouldSkipTemplate(actionURL) {
		c.logger.Info("[crawl] skipping exhausted template",
			slog.String("template", urlFingerprint(actionURL)),
		)
		return nil
	}

	currentPageHash, _, err := getPageHash(page)
	if err != nil {
		c.logger.Warn("[crawl] getPageHash failed", slog.String("error", err.Error()))
		return err
	}

	c.logger.Debug("Processing action - current state",
		slog.String("current_page_hash", currentPageHash),
		slog.String("action_origin_id", action.OriginID),
		slog.String("action", action.String()),
	)

	// LoadURL actions navigate directly to a URL — they don't need to be on any
	// particular origin page, so skip the navigate-back check entirely.
	needsNavigateBack := action.OriginID != "" && action.OriginID != currentPageHash && action.Type != types.ActionTypeLoadURL
	if needsNavigateBack {
		// Skip cross-origin actions if we're running low on time budget.
		// Navigate-back typically costs 5-10s; use a tight threshold to avoid
		// skipping too many actions near the end.
		if !c.crawlDeadline.IsZero() && time.Until(c.crawlDeadline) < 8*time.Second {
			c.logger.Info("[crawl] skipping action, low time budget",
				slog.String("remaining", time.Until(c.crawlDeadline).Round(time.Second).String()),
				slog.String("action", action.String()),
			)
			return nil
		}

		c.logger.Debug("Need to navigate back to origin",
			slog.String("from", currentPageHash),
			slog.String("to", action.OriginID),
		)
		navBackStart := time.Now()
		newPageHash, err := c.navigateBackToStateOrigin(action, page, currentPageHash)
		if err != nil {
			c.logger.Warn("[crawl] navigate-back failed",
				slog.String("error", err.Error()),
				slog.String("action", action.String()),
			)
			return err
		}
		c.logger.Info("[crawl] navigate-back",
			slog.String("duration", time.Since(navBackStart).Round(time.Millisecond).String()),
		)
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
	// Snapshot request yield counter before action execution so we can measure
	// how many new unique HTTP requests this action triggered (API calls, etc.).
	var requestsBefore int64
	if c.options.RequestYieldCounter != nil {
		requestsBefore = c.options.RequestYieldCounter.Load()
	}

	// Enable JS coverage profiler before action execution (coverage-guided mode only).
	// Profiler is scoped to action execution to avoid counting framework code from DOM traversal.
	if c.coverageTracker != nil {
		_ = proto.ProfilerEnable{}.Call(page.Page)
		// Detailed: false uses function-level granularity (not byte-level).
		// Detailed: true instruments every byte range and causes severe JS slowdown
		// that breaks SPA navigation timing — auth middleware race conditions cause
		// redirects when the SPA router is delayed by profiler overhead.
		_, _ = proto.ProfilerStartPreciseCoverage{Detailed: false}.Call(page.Page)
	}

	actionStart := time.Now()
	if err := c.executeCrawlStateAction(action, page); err != nil {
		c.logger.Warn("[crawl] executeCrawlStateAction failed",
			slog.String("action", action.String()),
			slog.String("error", err.Error()),
			slog.String("duration", time.Since(actionStart).Round(time.Millisecond).String()),
		)
		if c.coverageTracker != nil {
			_ = proto.ProfilerDisable{}.Call(page.Page)
		}
		return err
	}
	actionDuration := time.Since(actionStart)

	// Collect coverage delta after action execution.
	var coverageGain int
	if c.coverageTracker != nil {
		result, covErr := proto.ProfilerTakePreciseCoverage{}.Call(page.Page)
		if covErr == nil {
			coverageGain = c.coverageTracker.RecordAndDiff(result.Result)
			if coverageGain > 0 {
				c.logger.Info("[coverage] new JS code paths triggered",
					slog.Int("new_bytes", coverageGain),
					slog.Int("total_ranges", c.coverageTracker.TotalRanges()),
					slog.String("action", action.String()),
				)
			} else {
				c.logger.Debug("[coverage] no new code paths",
					slog.String("action", action.String()),
				)
			}
		} else {
			c.logger.Debug("[coverage] profiler error", slog.String("error", covErr.Error()))
		}
		_ = proto.ProfilerDisable{}.Call(page.Page)
	}

	// Check for captcha pages after navigation and attempt to solve them.
	// On success, wait for the page to settle and re-enter crawlFn so navigation
	// discovery runs on the post-solve page instead of the captcha page.
	if c.options.CaptchaHandler != nil {
		html, htmlErr := page.HTML()
		if htmlErr == nil {
			handled, solveErr := c.options.CaptchaHandler.HandleIfCaptcha(ctx, page.Page, html)
			if solveErr != nil {
				gologger.Warning().Msgf("captcha solving failed: %s", solveErr)
			}
			if handled && solveErr == nil {
				_ = page.WaitPageLoadHeurisitics()
			}
			if handled {
				// Skip navigation discovery on captcha pages — the discovered
				// links/forms belong to the captcha widget, not the real page.
				return nil
			}
		}
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

	// AI Exploration: if enabled, let the AI planner analyze the page and convert
	// its planned actions into crawl queue entries. Each planned action (click link,
	// fill form, expand menu) becomes a standard crawl action executed independently.
	explorerStart := time.Now()
	if c.options.ExplorationPlanner != nil && c.options.ExplorationPlanner.HasAPIKey() {
		graphCtx := &explorer.GraphContext{
			CurrentURL:      pageState.URL,
			PageTitle:       pageState.Title,
			PagesVisited:    c.uniqueActions.Len(),
			BudgetRemaining: c.options.MaxDepth*10 - c.uniqueActions.Len(),
		}
		for _, vid := range c.crawlGraph.GetVertices() {
			if ps, psErr := c.crawlGraph.GetPageState(vid); psErr == nil && ps.URL != "about:blank" {
				graphCtx.VisitedPaths = append(graphCtx.VisitedPaths, ps.URL)
			}
		}

		// Give the explorer its own timeout context — the crawl's master ctx
		// may be nearly expired after navigation + page load.
		explorerCtx, explorerCancel := context.WithTimeout(context.Background(), 45*time.Second)
		explorerPage := page.Page.Timeout(30 * time.Second)
		origPage := page.Page
		page.Page = explorerPage

		plan, planErr := c.options.ExplorationPlanner.Plan(explorerCtx, page, graphCtx)
		if planErr != nil {
			c.logger.Debug("[explorer] Planning failed, using standard discovery",
				slog.String("error", planErr.Error()))
		} else {
			snapshot := c.options.ExplorationPlanner.LastSnapshot()
			if snapshot != nil {
				explorer.ExecuteDirectActions(plan, snapshot)
			}
		}
		// Restore original page for the rest of the crawl loop
		page.Page = origPage
		explorerCancel()
	}

	explorerDuration := time.Since(explorerStart)

	navDiscoveryStart := time.Now()
	navigations, err := page.FindNavigations()
	if err != nil {
		return err
	}
	navDiscoveryDuration := time.Since(navDiscoveryStart)

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

	// Coverage boost for child actions: pages that triggered new code paths
	// get their discovered actions boosted in the priority queue.
	var covBoost int
	if c.coverageTracker != nil && coverageGain > 0 {
		covBoost = c.coverageTracker.CoverageBoost(pageState.URL)
		if covBoost != 0 {
			c.logger.Info("[coverage] boosting child actions",
				slog.Int("boost", covBoost),
				slog.Int("coverage_gain", coverageGain),
				slog.String("url", pageState.URL),
			)
		}
	}

	var queuedCount, dupCount, logoutCount int
	for _, nav := range navigations {
		actionHash := nav.Hash()
		if !c.uniqueActions.Add(actionHash) {
			dupCount++
			continue
		}

		// Check if the element we have is a logout page
		if nav.Element != nil && isLogoutPage(nav.Element) {
			c.logger.Debug("Skipping Found logout page",
				slog.String("url", nav.Element.Attributes["href"]),
			)
			logoutCount++
			continue
		}

		// Skip Chrome error/interstitial page elements
		if nav.Element != nil && isChromeErrorPage(nav.Element) {
			continue
		}
		nav.OriginID = pageState.UniqueID
		nav.OriginURL = pageState.URL
		nav.Depth = action.Depth + 1
		nav.CoverageBoost = covBoost

		c.logger.Debug("Got new navigation",
			slog.Any("navigation", nav),
		)
		if err := c.crawlQueue.Offer(nav); err != nil {
			return err
		}
		queuedCount++
	}

	// Record template yield for future dedup decisions
	c.recordTemplateYield(pageState.URL, queuedCount)
	if c.coverageTracker != nil {
		c.coverageTracker.RecordTemplateGain(pageState.URL, coverageGain)
	}

	// Measure request-level yield: new unique HTTP requests triggered during action.
	var requestDelta int
	if c.options.RequestYieldCounter != nil {
		requestDelta = int(c.options.RequestYieldCounter.Load() - requestsBefore)
	}

	// Record origin yield for page exhaustion tracking.
	// Productive = new navigations OR new HTTP requests (API calls).
	c.recordOriginYield(action.OriginID, queuedCount, requestDelta)

	totalDuration := time.Since(crawlStart)
	c.logger.Info("[crawl] page complete",
		slog.String("url", pageState.URL),
		slog.Int("depth", action.Depth),
		slog.String("action_time", actionDuration.Round(time.Millisecond).String()),
		slog.String("explorer_time", explorerDuration.Round(time.Millisecond).String()),
		slog.String("discovery_time", navDiscoveryDuration.Round(time.Millisecond).String()),
		slog.String("total_time", totalDuration.Round(time.Millisecond).String()),
		slog.Int("navs_found", len(navigations)),
		slog.Int("navs_queued", queuedCount),
		slog.Int("navs_dup", dupCount),
		slog.Int("navs_logout", logoutCount),
		slog.Int("new_requests", requestDelta),
		slog.Int("queue_size", c.crawlQueue.Size()),
		slog.Int("pages_crawled", c.uniqueActions.Len()),
		slog.Int("coverage_gain", coverageGain),
		slog.Bool("origin_exhausted", c.isOriginExhausted(action.OriginID)),
	)

	// Store the current page hash for origin-affinity dequeuing —
	// the next iteration will prefer actions from this same page.
	c.lastPageHash.Store(pageState.UniqueID)

	err = c.crawlGraph.AddPageState(*pageState)
	if err != nil {
		return err
	}

	// TODO: Check if the page opened new sub pages and if so capture their
	// navigation as well as close them so the state change can work.

	if len(navigations) == 0 && c.crawlQueue.Size() == 0 {
		c.logger.Info("[crawl] stopping: no navigations found and queue empty",
			slog.String("url", pageState.URL),
			slog.Int("depth", action.Depth),
			slog.Int("total_pages", c.uniqueActions.Len()),
		)
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
		// Use quick wait for non-link elements (buttons, divs) that typically
		// don't trigger full page navigation — saves ~2s per click.
		if action.Element != nil && !strings.EqualFold(action.Element.TagName, "A") {
			err = page.WaitPageLoadHeuristicsQuick()
		} else {
			err = page.WaitPageLoadHeurisitics()
		}
		if err != nil {
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

// Chrome error page element IDs that should never be crawled.
var chromeErrorPageIDs = map[string]bool{
	"download-link":            true,
	"download-button":          true,
	"reload-button":            true,
	"details-button":           true,
	"error-information-button": true,
	"sub-frame-error":          true,
	"main-frame-error":         true,
}

// isChromeErrorPage returns true if the element belongs to a Chrome error/interstitial page.
func isChromeErrorPage(element *types.HTMLElement) bool {
	if element == nil {
		return false
	}
	return chromeErrorPageIDs[element.ID]
}

// actionPriority returns a numeric priority for a crawl action.
// Lower number = higher priority (processed first).
// Priority = base (action type) + depth penalty + template penalty + chrome penalty.
// This ensures breadth-first exploration: all depth-0 actions before depth-1, etc.
func actionPriority(a *types.Action) int {
	base := 0
	switch {
	case a.Type == types.ActionTypeLoadURL:
		base = 0
	case a.Type == types.ActionTypeLeftClick && a.Element != nil &&
		strings.EqualFold(a.Element.TagName, "A"):
		base = 1
	case a.Type == types.ActionTypeFillForm:
		base = 2
	case a.Type == types.ActionTypeLeftClick && a.Element != nil &&
		strings.EqualFold(a.Element.TagName, "BUTTON"):
		if isUIChrome(a.Element) {
			base = 8 // UI chrome buttons (toggles, icon buttons) — nearly last
		} else {
			base = 3
		}
	default:
		base = 4
	}

	// Depth penalty: each depth level adds 10 to priority.
	// Ensures ALL depth-N actions before ANY depth-N+1 actions.
	depthPenalty := a.Depth * 10

	// Template penalty: known template URLs (containing UUIDs, IDs, etc.)
	// get deprioritized since they're likely similar to already-visited pages.
	templatePenalty := 0
	targetURL := a.Input
	if targetURL == "" && a.Element != nil {
		targetURL = a.Element.Attributes["href"]
	}
	if targetURL != "" && isTemplatedURL(targetURL) {
		templatePenalty = 5
	}

	return base + depthPenalty + templatePenalty + a.CoverageBoost + classifyAction(a)
}

// classifyAction returns a priority adjustment based on semantic analysis of
// the action's element attributes. Negative = boost (higher priority),
// positive = penalty (lower priority).
func classifyAction(a *types.Action) int {
	if a.Element == nil {
		return 0
	}

	tag := strings.ToUpper(a.Element.TagName)
	attrs := a.Element.Attributes
	text := strings.TrimSpace(a.Element.TextContent)

	switch tag {
	case "A":
		return classifyLink(a, attrs)
	case "BUTTON":
		return classifyButton(attrs, text)
	case "DIV", "SPAN":
		return classifyCursorInteractive(attrs)
	}
	return 0
}

// classifyLink returns a priority adjustment for link elements.
// No boost for nav links — they're already priority 1 (highest for clicks),
// and boosting them causes them to fire before the SPA auth session settles.
// The penalty on dialog triggers (+3) and toggles (+2) effectively moves
// nav links up relative to buttons without causing timing issues.
func classifyLink(a *types.Action, attrs map[string]string) int {
	return 0
}

// classifyButton penalizes dialog triggers and toggles.
func classifyButton(attrs map[string]string, text string) int {
	// Dialog/popup triggers
	if attrs["aria-haspopup"] != "" {
		return 3
	}
	// Keyboard shortcut indicators in text → likely command palette trigger
	if dialogTriggerPattern.MatchString(text) {
		return 3
	}
	// Toggle/dropdown buttons
	if _, ok := attrs["aria-expanded"]; ok {
		return 2
	}
	return 0
}

// classifyCursorInteractive penalizes dropdowns and tab switches.
func classifyCursorInteractive(attrs map[string]string) int {
	if _, ok := attrs["aria-expanded"]; ok {
		return 2
	}
	role := attrs["role"]
	if role == "tab" || role == "combobox" || role == "listbox" || role == "menuitem" {
		return 2
	}
	return 0
}

// firstPathSegment extracts the first path segment from a URL or path.
// "/agents/foo" → "agents", "https://example.com/task/123" → "task"
func firstPathSegment(rawURL string) string {
	path := rawURL
	// Strip scheme+host if present
	if idx := strings.Index(path, "://"); idx >= 0 {
		path = path[idx+3:]
		if sl := strings.Index(path, "/"); sl >= 0 {
			path = path[sl:]
		} else {
			return ""
		}
	}
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return ""
	}
	seg := strings.SplitN(path, "/", 2)[0]
	// Strip query/fragment
	if idx := strings.IndexAny(seg, "?#"); idx >= 0 {
		seg = seg[:idx]
	}
	return strings.ToLower(seg)
}

// navClassPattern matches CSS classes indicating navigation/sidebar context.
var navClassPattern = regexp.MustCompile(`(?i)(?:^|[\s/])(?:nav|sidebar|menu|navigation)(?:[-_/]|\s|$)`)

// dialogTriggerPattern matches keyboard shortcut indicators in button text.
var dialogTriggerPattern = regexp.MustCompile(`[⌘⌃⇧⌥]|Ctrl\+|Alt\+|Cmd\+`)

// uiChromeClassPattern matches CSS classes typical of tiny UI chrome widgets
// (expand/collapse toggles, icon-only buttons, sidebar collapse buttons).
// These are micro-interactions that rarely lead to new pages.
var uiChromeClassPattern = regexp.MustCompile(`(?:^|\s)(?:size-[4-7]|h-[6-8]\s+w-[6-8]|w-[6-8]\s+h-[6-8])(?:\s|$)`)

// isUIChrome returns true if the element looks like a UI chrome widget
// (expand/collapse toggle, icon-only button, sidebar collapse) rather than
// a meaningful navigation button ("New Task", "Submit", "Search").
func isUIChrome(el *types.HTMLElement) bool {
	if el == nil {
		return false
	}

	// Radix UI toggle/trigger IDs (e.g., #radix-_r_1q_)
	if strings.HasPrefix(el.ID, "radix-") {
		// Radix buttons with no meaningful text are UI chrome
		text := strings.TrimSpace(el.TextContent)
		if text == "" || len(text) <= 2 {
			return true
		}
	}

	// Small icon-only buttons: classes like "size-6", "h-8 w-8"
	if uiChromeClassPattern.MatchString(el.Classes) {
		text := strings.TrimSpace(el.TextContent)
		if text == "" || len(text) <= 2 {
			return true
		}
	}

	return false
}
