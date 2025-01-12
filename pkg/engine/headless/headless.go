package headless

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
)

type Headless struct {
	options *types.CrawlerOptions
}

// New returns a new headless crawler instance
func New(options *types.CrawlerOptions) (*Headless, error) {
	// create a new logger
	writer := os.Stderr

	// set global logger with custom options
	level := slog.LevelInfo
	if options.Options.Debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(
		tint.NewHandler(writer, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
		}),
	))

	return &Headless{
		options: options,
	}, nil
}

// Crawl executes the headless crawling on a given URL
func (h *Headless) Crawl(URL string) error {
	crawlOpts := crawler.Options{
		ChromiumPath:     h.options.Options.SystemChromePath,
		MaxDepth:         h.options.Options.MaxDepth,
		ShowBrowser:      h.options.Options.ShowBrowser,
		MaxCrawlDuration: h.options.Options.CrawlDuration,
		MaxBrowsers:      1,
		PageMaxTimeout:   30 * time.Second,
		RequestCallback: func(rr *output.Result) {
			h.options.OutputWriter.Write(rr)
		},
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
	return nil
}
