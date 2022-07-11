package engine

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless"
	"github.com/projectdiscovery/katana/pkg/engine/simple"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/parser"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
)

// Crawler is a crawler instance
type Crawler struct {
	Options  *types.CrawlerOptions
	Simple   *simple.SimpleEngine
	Headless *headless.HeadlessEngine
}

// New returns a new crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	httpclient, dialer, err := buildClient(options.Options)
	if err != nil {
		return nil, errors.Wrap(err, "could not create http client")
	}

	simpleEngine, err := simple.NewWithClients(options, dialer, httpclient)
	if err != nil {
		return nil, errors.Wrap(err, "could not create simple engine")
	}

	headlessEngine, err := headless.NewWithClients(options, dialer, httpclient)
	if err != nil {
		return nil, errors.Wrap(err, "could not create simple engine")
	}

	crawler := &Crawler{
		Options:  options,
		Simple:   simpleEngine,
		Headless: headlessEngine,
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() {
	if c.Simple != nil {
		c.Simple.Close()
	}
	if c.Headless != nil {
		c.Headless.Close()
	}
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(url string) {
	ctx, cancel := context.WithCancel(context.Background())
	if c.Options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.Options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	queue := queue.New(c.Options.Options.Strategy)
	queue.Push(navigation.Request{Method: http.MethodGet, URL: url, Depth: 0}, 0)

	for {
		// Quit the crawling for zero items or context timeout
		if queue.Len() == 0 {
			return
		}
		if ctx.Err() != nil {
			return
		}
		item := queue.Pop()
		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}
		c.Options.RateLimit.Take()

		// Delay if the user has asked for it
		if c.Options.Options.Delay > 0 {
			time.Sleep(time.Duration(c.Options.Options.Delay) * time.Second)
		}

		var (
			resp navigation.Response
			err  error
		)
		switch {
		case c.Options.Options.Headless:
			resp, err = c.Headless.MakeRequest(req)
		default:
			resp, err = c.Simple.MakeRequest(req)
		}
		if err != nil {
			gologger.Error().Msgf("Could not request seed URL: %s\n", err)
			return
		}
		if resp.Resp == nil || resp.Reader == nil {
			continue
		}
		parser.ParseResponse(resp, func(nr navigation.Request) {
			// Ignore blank URL items
			if nr.URL == "" {
				return
			}
			// Only work on unique items
			if !c.Options.UniqueFilter.Unique(nr.RequestURL()) {
				return
			}

			// Write the found result to output
			result := &output.Result{
				Body:   nr.Body,
				URL:    nr.URL,
				Source: nr.Source,
			}
			if nr.Method != http.MethodGet {
				result.Method = nr.Method
			}
			_ = c.Options.OutputWriter.Write(result)

			// Do not add to crawl queue if max items are reached
			if nr.Depth >= c.Options.Options.MaxDepth {
				return
			}
			queue.Push(nr, nr.Depth)
		})
	}
}
