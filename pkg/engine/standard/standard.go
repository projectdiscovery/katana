package standard

import (
	"context"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/remeh/sizedwaitgroup"
)

// Crawler is a standard crawler instance
type Crawler struct {
	headers map[string]string

	options    *types.CrawlerOptions
	httpclient *retryablehttp.Client
	dialer     *fastdialer.Dialer
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	httpclient, dialer, err := common.BuildClient(options.Options)
	if err != nil {
		return nil, errors.Wrap(err, "could not create http client")
	}
	crawler := &Crawler{
		headers:    options.Options.ParseCustomHeaders(),
		options:    options,
		dialer:     dialer,
		httpclient: httpclient,
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	c.dialer.Close()
	return nil
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	parsed, err := url.Parse(rootURL)
	if err != nil {
		return errors.Wrap(err, "could not parse root URL")
	}
	hostname := parsed.Hostname()

	ctx, cancel := context.WithCancel(context.Background())
	if c.options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	queue := queue.New(c.options.Options.Strategy)
	queue.Push(navigation.Request{Method: http.MethodGet, URL: rootURL, Depth: 0}, 0)
	parseResponseCallback := c.makeParseResponseCallback(queue)

	wg := sizedwaitgroup.New(c.options.Options.Concurrency)
	running := int32(0)
	for {
		// Quit the crawling for zero items or context timeout
		if !(atomic.LoadInt32(&running) > 0) && (queue.Len() == 0 || ctx.Err() != nil) {
			break
		}
		item := queue.Pop()
		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}
		if !utils.IsURL(req.URL) {
			continue
		}
		wg.Add()
		atomic.AddInt32(&running, 1)

		go func() {
			defer wg.Done()
			defer atomic.AddInt32(&running, -1)

			c.options.RateLimit.Take()

			// Delay if the user has asked for it
			if c.options.Options.Delay > 0 {
				time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
			}
			resp, err := c.makeRequest(ctx, req, hostname)
			if err != nil {
				gologger.Warning().Msgf("Could not request seed URL: %s\n", err)
				return
			}
			if resp.Resp == nil || resp.Reader == nil {
				return
			}
			parser.ParseResponse(resp, parseResponseCallback)
		}()
	}
	wg.Wait()

	return nil
}

// makeParseResponseCallback returns a parse response function callback
func (c *Crawler) makeParseResponseCallback(queue *queue.VarietyQueue) func(nr navigation.Request) {
	return func(nr navigation.Request) {
		if !utils.IsURL(nr.URL) {
			return
		}
		// Ignore blank URL items and only work on unique items
		if nr.URL == "" || !c.options.UniqueFilter.Unique(nr.RequestURL()) {
			return
		}

		// Write the found result to output
		result := &output.Result{
			Timestamp: time.Now(),
			Body:      nr.Body,
			URL:       nr.URL,
			Source:    nr.Source,
			Tag:       nr.Tag,
			Attribute: nr.Attribute,
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
	}
}
