package headless

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/lmittmann/tint"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
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

	agents := buildAgents(h.options.Options)
	if len(agents) == 1 {
		return h.crawlAs(URL, agents[0], len(agents))
	}

	// Identities run at the same time, so -ct bounds the whole crawl rather
	// than each identity in turn.
	errs := make([]error, len(agents))
	var wg sync.WaitGroup
	for i, a := range agents {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.logger.Info("headless agent starting", slog.String("agent", a.label))
			if err := h.crawlAs(URL, a, len(agents)); err != nil {
				gologger.Warning().Msgf("headless agent %s failed to crawl %s: %s", a.label, URL, err)
				errs[i] = err
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

// crawlAs crawls URL as identity a, one of agents identities.
func (h *Headless) crawlAs(URL string, a agent, agents int) error {
	dedupScope := a.dedupScope(agents)
	scopeValidator := validateScopeFunc(h, URL)
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
		UserDataDir:           h.options.Options.ChromeDataDir,
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
		ExplosionBudget:       h.options.Options.ExplosionBudget,
		ExplosionHamming:      h.options.Options.ExplosionHamming,
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
			isUnique := h.isUniqueURL(dedupScope, rr.Request.URL)

			// Always run additional analysis regardless of uniqueness so we
			// don't miss URL discoveries embedded in a response body that the
			// browser happened to fetch more than once.
			navigationRequests := h.performAdditionalAnalysis(rr, dedupScope)
			for _, req := range navigationRequests {
				if agents > 1 {
					labelAgent(req, a.label)
				}
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
			// Name the identity so results of several agents can be told apart.
			if agents > 1 {
				labelAgent(rr, a.label)
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

	if a.credentials != "" {
		crawlOpts.AuthUsername, crawlOpts.AuthPassword = splitCredentials(a.credentials)
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

// isUniqueURL reports whether rawURL is new within dedupScope, the prefix that
// keeps the results of each crawl identity apart.
func (h *Headless) isUniqueURL(dedupScope, rawURL string) bool {
	dedupKey := rawURL
	if h.options.Options.IgnoreQueryParams {
		dedupKey = utils.ReplaceAllQueryParam(dedupKey, "")
	}
	if h.options.Options.FilterSimilar {
		dedupKey = utils.FingerprintURL(dedupKey, h.pathTrie)
	}
	return h.options.UniqueFilter.UniqueURL(dedupScope + dedupKey)
}

// labelAgent names identity label on result: in request.agent for every
// result, and also in response.extra.agent when the result has a response.
func labelAgent(rr *output.Result, label string) {
	if rr == nil {
		return
	}
	if rr.Request != nil {
		rr.Request.Agent = label
	}
	if rr.Response != nil {
		if rr.Response.Extra == nil {
			rr.Response.Extra = map[string]any{}
		}
		rr.Response.Extra["agent"] = label
	}
}

func (h *Headless) performAdditionalAnalysis(rr *output.Result, dedupScope string) []*output.Result {
	newNavigations := h.options.Parser.ParseResponse(rr.Response)

	navigationRequests := make([]*output.Result, 0)
	for _, resp := range newNavigations {
		if !h.isUniqueURL(dedupScope, resp.URL) {
			continue
		}
		navigationRequests = append(navigationRequests, &output.Result{
			Request: resp,
		})
	}
	return navigationRequests
}
