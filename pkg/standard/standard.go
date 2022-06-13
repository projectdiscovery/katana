package standard

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/retryablehttp-go"
)

// Crawler is a standard crawler instance
type Crawler struct {
	options    *types.CrawlerOptions
	httpclient *retryablehttp.Client
	dialer     *fastdialer.Dialer
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	httpclient, dialer, err := buildClient(options.Options)
	if err != nil {
		return nil, errors.Wrap(err, "could not create http client")
	}

	crawler := &Crawler{
		options:    options,
		dialer:     dialer,
		httpclient: httpclient,
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() {
	c.dialer.Close()
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(url string) {
	ctx, cancel := context.WithCancel(context.Background())
	if c.options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	queue := queue.New(c.options.Options.Strategy)
	queue.Push(navigationRequest{Method: http.MethodGet, URL: url, Depth: 0}, 0)

	for {
		// Quit the crawling for zero items or context timeout
		if queue.Len() == 0 {
			return
		}
		if ctx.Err() != nil {
			return
		}
		item := queue.Pop()
		req, ok := item.(navigationRequest)
		if !ok {
			continue
		}
		c.options.RateLimit.Take()

		// Delay if the user has asked for it
		if c.options.Options.Delay > 0 {
			time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
		}
		resp, err := c.makeRequest(req)
		if err != nil {
			gologger.Error().Msgf("Could not request seed URL: %s\n", err)
			return
		}
		if resp.Resp == nil || resp.Reader == nil {
			continue
		}
		parseResponse(resp, func(nr navigationRequest) {
			// Ignore blank URL items
			if nr.URL == "" {
				return
			}
			// Only work on unique items
			if !c.options.UniqueFilter.Unique(nr.RequestURL()) {
				return
			}

			// Write the found result to output
			c.options.OutputWriter.Write(&output.Result{URL: nr.URL, Source: nr.Source})

			// Do not add to crawl queue if max items are reached
			if nr.Depth >= c.options.Options.MaxDepth {
				return
			}
			queue.Push(nr, nr.Depth)
		})
	}
}
