package headless

import (
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
	_ "github.com/projectdiscovery/katana/pkg/engine/headless/captcha/capsolver"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	mapsutil "github.com/projectdiscovery/utils/maps"
)

type Headless struct {
	logger  *slog.Logger
	options *types.CrawlerOptions

	deduplicator *mapsutil.SyncLockMap[string, struct{}]
	pathTrie     *utils.PathTrie

	debugger *CrawlDebugger
}

// New returns a new headless crawler instance
func New(options *types.CrawlerOptions) (*Headless, error) {
	logger := newLogger(options)

	headless := &Headless{
		logger:  logger,
		options: options,

		deduplicator: mapsutil.NewSyncLockMap[string, struct{}](),
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

	scopeValidator := validateScopeFunc(h, URL)

	crawlOpts := crawler.Options{
		Context:           h.options.Options.Context,
		ChromiumPath:      h.options.Options.SystemChromePath,
		MaxDepth:          h.options.Options.MaxDepth,
		ShowBrowser:       h.options.Options.ShowBrowser,
		MaxCrawlDuration:  h.options.Options.CrawlDuration,
		MaxFailureCount:   h.options.Options.MaxFailureCount,
		NoSandbox:         h.options.Options.HeadlessNoSandbox,
		Proxy:             h.options.Options.Proxy,
		MaxBrowsers:       1,
		PageMaxTimeout:    30 * time.Second,
		ScopeValidator:    scopeValidator,
		AutomaticFormFill: h.options.Options.AutomaticFormFill,
		RequestCallback: func(rr *output.Result) {
			if rr == nil || rr.Request == nil {
				return
			}
			if scopeValidator != nil && !scopeValidator(rr.Request.URL) {
				return
			}
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

			if rr.Response != nil {
				rr.Response.KnowledgeBase = h.options.ClassifyPage(rr.Response.Body)
				rr.Response.Raw = ""
				rr.Response.Body = ""
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

func (h *Headless) performAdditionalAnalysis(rr *output.Result) []*output.Result {
	responseParser := parser.NewResponseParser()
	newNavigations := responseParser.ParseResponse(rr.Response)

	navigationRequests := make([]*output.Result, 0)
	for _, resp := range newNavigations {
		dedupKey := resp.URL
		if h.options.Options.FilterSimilar {
			dedupKey = utils.FingerprintURL(dedupKey, h.pathTrie)
		}
		if _, ok := h.deduplicator.Get(dedupKey); ok {
			continue
		}
		if err := h.deduplicator.Set(dedupKey, struct{}{}); err != nil {
			h.logger.Debug("deduplicator set failed",
				slog.String("url", resp.URL),
				slog.String("error", err.Error()),
			)
			continue
		}

		navigationRequests = append(navigationRequests, &output.Result{
			Request: resp,
		})
	}
	return navigationRequests
}
