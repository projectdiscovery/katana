package headless

import (
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	mapsutil "github.com/projectdiscovery/utils/maps"
)

type Headless struct {
	logger  *slog.Logger
	options *types.CrawlerOptions

	deduplicator *mapsutil.SyncLockMap[string, struct{}]

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
		return nil
	}
	rootHostname := parsedURL.Hostname()

	return func(s string) bool {
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
		ChromiumPath:     h.options.Options.SystemChromePath,
		MaxDepth:         h.options.Options.MaxDepth,
		ShowBrowser:      h.options.Options.ShowBrowser,
		MaxCrawlDuration: h.options.Options.CrawlDuration,
		MaxFailureCount:  h.options.Options.MaxFailureCount,
		MaxBrowsers:      1,
		PageMaxTimeout:   30 * time.Second,
		ScopeValidator:   scopeValidator,
		RequestCallback: func(rr *output.Result) {
			if !scopeValidator(rr.Request.URL) {
				return
			}
			navigationRequests := h.performAdditionalAnalysis(rr)
			for _, req := range navigationRequests {
				h.options.OutputWriter.Write(req)
			}

			rr.Response.Raw = ""
			rr.Response.Body = ""
			h.options.OutputWriter.Write(rr)
		},
		Logger:              h.logger,
		ChromeUser:          h.options.ChromeUser,
		EnableDiagnostics:   h.options.Options.EnableDiagnostics,
		Trace:               h.options.Options.EnableDiagnostics,
		CookieConsentBypass: true,
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
	newNavigations := parser.ParseResponse(rr.Response)

	navigationRequests := make([]*output.Result, 0)
	for _, resp := range newNavigations {
		if _, ok := h.deduplicator.Get(resp.URL); ok {
			continue
		}
		h.deduplicator.Set(resp.URL, struct{}{})

		navigationRequests = append(navigationRequests, &output.Result{
			Request: resp,
		})
	}
	return navigationRequests
}
