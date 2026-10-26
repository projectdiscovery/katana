package headless

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	_ "github.com/projectdiscovery/katana/pkg/engine/headless/captcha/capsolver"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
)

type Headless struct {
	logger  *slog.Logger
	options *types.CrawlerOptions

	pathTrie *utils.PathTrie

	debugger *CrawlDebugger

	hooks Hooks
}

// New returns a new headless crawler instance
func New(options *types.CrawlerOptions) (*Headless, error) {
	logger := newLogger(options)

	headless := &Headless{
		logger:  logger,
		options: options,
	}
	if options.Options.FilterSimilar {
		headless.pathTrie = utils.NewPathTrie(options.Options.FilterSimilarThreshold)
	}

	// Show crawl debugger if verbose is enabled
	if options.Options.Verbose {
		headless.debugger = NewCrawlDebugger(8089)
	}

	return headless, nil
}

func newLogger(options *types.CrawlerOptions) *slog.Logger {
	if options.Logger != nil {
		return options.Logger
	}

	writer := os.Stderr

	// set global logger with custom options
	level := slog.LevelInfo
	if options.Options.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(
		tint.NewHandler(writer, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
		}),
	)
	return logger
}

func validateScopeFunc(h *Headless, URL string) browser.ScopeValidator {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return func(string) bool { return true }
	}
	rootHostname := parsedURL.Hostname()

	return func(s string) bool {
		if h.options.ScopeManager == nil {
			return true
		}
		parsed, err := url.Parse(s)
		if err != nil {
			return false
		}
		validated, err := h.options.ScopeManager.Validate(parsed, rootHostname)
		if err != nil {
			return false
		}
		return validated
	}
}

// Crawl executes the headless crawling on a given URL
func (h *Headless) Crawl(URL string) error {
	if h.debugger != nil {
		h.debugger.StartURL(URL, 0)
	}
	defer func() {
		if h.debugger != nil {
			h.debugger.EndURL(URL)
		}
	}()

	pool := buildAgentPool(h.options.Options)
	// A remote browser (-cwu) ignores user-data-dir, so per-agent temp jars are
	// pointless there; only isolate data dirs for locally launched browsers.
	remoteBrowser := h.options.Options.ChromeWSUrl != ""
	var firstErr error
	for _, agent := range pool.All() {
		cleanup := func() {}
		if !remoteBrowser {
			c, err := ensureAgentDataDir(agent)
			if err != nil {
				return err
			}
			cleanup = c
		}
		err := h.crawlWithAgent(URL, agent)
		cleanup()
		if err != nil {
			gologger.Warning().Msgf("headless agent %s failed to crawl %s: %s", agent.ID, URL, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (h *Headless) crawlWithAgent(URL string, agent *cartography.Agent) error {
	scopeValidator := validateScopeFunc(h, URL)

	userDataDir := h.options.Options.ChromeDataDir
	if agent != nil && agent.DataDir != "" {
		userDataDir = agent.DataDir
	}

	crawlOpts := crawler.Options{
		Context:               h.options.Options.Context,
		UseInstalledChrome:    h.options.Options.UseInstalledChrome,
		ChromiumPath:          h.options.Options.SystemChromePath,
		MaxDepth:              h.options.Options.MaxDepth,
		ShowBrowser:           h.options.Options.ShowBrowser,
		MaxCrawlDuration:      h.options.Options.CrawlDuration,
		MaxFailureCount:       h.options.Options.MaxFailureCount,
		NoSandbox:             h.options.Options.HeadlessNoSandbox,
		NoIncognito:           h.options.Options.HeadlessNoIncognito,
		UserDataDir:           userDataDir,
		Proxy:                 h.options.Options.Proxy,
		MaxBrowsers:           1,
		PageMaxTimeout:        30 * time.Second,
		ScopeValidator:        scopeValidator,
		AutomaticFormFill:     h.options.Options.AutomaticFormFill,
		CustomHeaders:         h.options.Options.ParseCustomHeaders(),
		RootURL:               URL,
		Interactive:           h.options.Options.Interactive,
		InteractionDepth:      h.options.Options.InteractionDepth,
		InteractionSelector:   h.options.Options.InteractionSelector,
		InteractionExclude:    h.options.Options.InteractionExclude,
		InteractionMaxStates:  h.options.Options.InteractionMaxStates,
		InteractionMaxActions: h.options.Options.InteractionMaxActions,
		InteractionSettle:     h.options.Options.InteractionSettle,
		InteractionDuration:   h.options.Options.InteractionDuration,
		PageLoadStrategy:      h.options.Options.PageLoadStrategy,
		ChromeWSUrl:           h.options.Options.ChromeWSUrl,
		DOMWaitTime:           h.options.Options.DOMWaitTime,
		MaxScrollSteps:        h.options.Options.MaxScrollSteps,
		RewalkSample:          h.options.Options.RewalkSample,
		LinkIdentityBudget:    h.options.Options.LinkIdentityBudget,
		RequestCallback: func(rr *output.Result) {
			if rr == nil || rr.Request == nil {
				return
			}
			if scopeValidator != nil && !scopeValidator(rr.Request.URL) {
				return
			}

			// Register the real (intercepted) request URL before parsing the
			// response body for additional discoveries. This ensures that real
			// results with full response data always take priority over
			// synthetic Request-only entries produced by performAdditionalAnalysis.
			isUnique := h.isUniqueURL(rr.Request.URL)

			// Always run additional analysis regardless of uniqueness so we
			// don't miss URL discoveries embedded in a response body that the
			// browser happened to fetch more than once.
			navigationRequests := h.performAdditionalAnalysis(rr)
			for _, req := range navigationRequests {
				if err := h.options.OutputWriter.Write(req); err != nil {
					h.logger.Debug("failed to write navigation result",
						slog.String("url", func() string {
							if req != nil && req.Request != nil {
								return req.Request.URL
							}
							return ""
						}()),
						slog.String("error", err.Error()),
					)
				}
			}

			if !isUnique {
				return
			}

			if rr.Response != nil {
				var req *http.Request
				if rr.Response.Resp != nil {
					req = rr.Response.Resp.Request
				}
				rr.Response.KnowledgeBase = h.options.BuildKnowledgeBase(rr.Response.Body, req, rr.Response.Resp)
				if h.options.Options.OmitRaw {
					rr.Response.Raw = ""
				}
				if h.options.Options.OmitBody {
					rr.Response.Body = ""
				}
			}
			if err := h.options.OutputWriter.Write(rr); err != nil {
				h.logger.Debug("failed to write result",
					slog.String("error", err.Error()),
				)
			}
		},
		Logger:              h.logger,
		ChromeUser:          h.options.ChromeUser,
		EnableDiagnostics:   h.options.Options.EnableDiagnostics,
		Trace:               h.options.Options.EnableDiagnostics,
		CookieConsentBypass: true,
		UserArguments:       h.options.Options.ParseHeadlessOptionalArguments(),
		DitClassifier:       h.options.DitClassifier,
		Hooks:               h.hooks,
	}

	creds := h.options.Options.AuthCredentials
	if agent != nil && agent.Credentials != "" {
		creds = agent.Credentials
	}
	if creds != "" {
		crawlOpts.AuthUsername, crawlOpts.AuthPassword = splitCredentials(creds)
	}

	if path := h.options.Options.RecordedFlow; path != "" {
		steps, err := auth.StepsFromFile(path, crawlOpts.AuthUsername, crawlOpts.AuthPassword)
		if err != nil {
			return err
		}
		crawlOpts.AuthSteps = steps
	}

	if provider := h.options.Options.CaptchaSolverProvider; provider != "" {
		gologger.Debug().Msgf("captcha solver enabled: provider=%s", provider)
		handler, err := captcha.NewHandler(provider, h.options.Options.CaptchaSolverAPIKey)
		if err != nil {
			gologger.Warning().Msgf("captcha handler init failed: %s", err)
		} else {
			crawlOpts.CaptchaHandler = handler
		}
	}

	// TODO: Make the crawling multi-threaded. Right now concurrency is hardcoded to 1.

	if capturer, ok := h.options.OutputWriter.(interface{ Capture(*output.Result) }); ok {
		crawlOpts.TrafficCallback = capturer.Capture
	}

	headlessCrawler, err := crawler.New(crawlOpts)
	if err != nil {
		return err
	}
	defer headlessCrawler.Close()

	if err = headlessCrawler.Crawl(URL); err != nil {
		return err
	}
	return nil
}

func (h *Headless) Close() error {
	if h.debugger != nil {
		h.debugger.Close()
	}
	return nil
}

func (h *Headless) isUniqueURL(rawURL string) bool {
	dedupKey := rawURL
	if h.options.Options.IgnoreQueryParams {
		dedupKey = utils.ReplaceAllQueryParam(dedupKey, "")
	}
	if h.options.Options.FilterSimilar {
		dedupKey = utils.FingerprintURL(dedupKey, h.pathTrie)
	}
	return h.options.UniqueFilter.UniqueURL(dedupKey)
}

func (h *Headless) performAdditionalAnalysis(rr *output.Result) []*output.Result {
	newNavigations := h.options.Parser.ParseResponse(rr.Response)

	navigationRequests := make([]*output.Result, 0)
	for _, resp := range newNavigations {
		if !h.isUniqueURL(resp.URL) {
			continue
		}
		navigationRequests = append(navigationRequests, &output.Result{
			Request: resp,
		})
	}
	return navigationRequests
}
