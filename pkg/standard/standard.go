package standard

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/navigation"
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
func (c *Crawler) Crawl(URL string) error {
	ctx, cancel := context.WithCancel(context.Background())
	if c.options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	queue := queue.New(c.options.Options.Strategy)
	queue.Push(navigation.NavigationRequest{Method: http.MethodGet, URL: URL, Depth: 0}, 0)

	for {
		// Quit the crawling for zero items or context timeout
		if queue.Len() == 0 {
			return io.EOF
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		item := queue.Pop()
		req, ok := item.(navigation.NavigationRequest)
		if !ok {
			continue
		}

		// only visits vertexes once
		if ok, err := c.options.GraphDB.HasEndpoint(ctx, req.ToEphemeralEntity()); err == nil && ok {
			continue
		}

		// if we arrive here, we add the successful request/response to the graph
		node, err := c.options.GraphDB.GetOrCreate(ctx, req.ToEphemeralEntity())
		if err != nil {
			gologger.Error().Msgf("Could not create the node URL: %s\n", err)
		}

		c.options.RateLimit.Take()

		// Delay if the user has asked for it
		if c.options.Options.Delay > 0 {
			time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
		}
		resp, err := c.makeRequest(req)
		if err != nil {
			gologger.Error().Msgf("Could not request seed URL: %s\n", err)
		}
		if resp.Resp == nil || resp.Reader == nil {
			continue
		}

		// connect the endpoint with its ancestor
		if req.FromNode != nil {
			_, _ = c.options.GraphDB.ConnectEndpoints(context.Background(), req.FromNode, node)
		}

		parseResponse(resp, func(nr navigation.NavigationRequest) {
			// Ignore blank URL items
			if nr.URL == "" {
				return
			}
			// Only work on unique items
			if !c.options.UniqueFilter.Unique(nr.RequestURL()) {
				return
			}

			// back reference the original request
			nr.FromNode = node

			// Write the found result to output
			result := &output.Result{
				Body:   nr.Body,
				URL:    nr.URL,
				Source: nr.Source,
			}
			if nr.Method != http.MethodGet {
				result.Method = nr.Method
			}
			_ = c.options.OutputWriter.Write(result)

			// Do not add to crawl queue if max items are reached
			if nr.Depth >= c.options.Options.MaxDepth {
				return
			}
			queue.Push(nr, nr.Depth)
		})
	}
}
