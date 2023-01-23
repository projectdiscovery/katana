package standard

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/parser/files"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	errorutil "github.com/projectdiscovery/utils/errors"
	"github.com/remeh/sizedwaitgroup"
)

// Crawler is a standard crawler instance
type Crawler struct {
	headers    map[string]string
	knownFiles *files.KnownFiles
	options    *types.CrawlerOptions
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	crawler := &Crawler{
		headers: options.Options.ParseCustomHeaders(),
		options: options,
	}
	if options.Options.KnownFiles != "" {
		httpclient, _, err := common.BuildClient(options.Dialer, options.Options, nil)
		if err != nil {
			return nil, errorutil.NewWithTag("standard", "could not create http client").Wrap(err)
		}
		crawler.knownFiles = files.New(httpclient, options.Options.KnownFiles)
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	return nil
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	parsed, err := url.Parse(rootURL)
	if err != nil {
		return errorutil.NewWithTag("standard", "could not parse root URL").Wrap(err)
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

	if c.knownFiles != nil {
		if err := c.knownFiles.Request(rootURL, func(nr navigation.Request) {
			parseResponseCallback(nr)
		}); err != nil {
			gologger.Warning().Msgf("Could not parse known files for %s: %s\n", rootURL, err)
		}
	}
	httpclient, _, err := common.BuildClient(c.options.Dialer, c.options.Options, func(resp *http.Response, depth int) {
		body, _ := io.ReadAll(resp.Body)
		reader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		parser.ParseResponse(navigation.Response{Depth: depth + 1, Options: c.options, RootHostname: hostname, Resp: resp, Body: body, Reader: reader}, parseResponseCallback)
	})
	if err != nil {
		return errorutil.NewWithTag("standard", "could not create http client").Wrap(err)
	}

	wg := sizedwaitgroup.New(c.options.Options.Concurrency)
	running := int32(0)
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		// Quit the crawling for zero items or context timeout
		if !(atomic.LoadInt32(&running) > 0) && (queue.Len() == 0) {
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
			resp, err := c.makeRequest(ctx, req, hostname, req.Depth, httpclient)
			if err != nil {
				gologger.Warning().Msgf("Could not request seed URL %s: %s\n", req.URL, err)
				outputError := &output.Error{
					Timestamp: time.Now(),
					Endpoint:  req.RequestURL(),
					Source:    req.Source,
					Error:     err.Error(),
				}
				_ = c.options.OutputWriter.WriteErr(outputError)
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
		if nr.URL == "" || !utils.IsURL(nr.URL) {
			return
		}
		parsed, err := url.Parse(nr.URL)
		if err != nil {
			return
		}
		// Ignore blank URL items and only work on unique items
		if !c.options.UniqueFilter.UniqueURL(nr.RequestURL()) && len(nr.CustomFields) == 0 {
			return
		}
		// - URLs stuck in a loop
		if c.options.UniqueFilter.IsCycle(nr.RequestURL()) {
			return
		}

		// Write the found result to output
		result := &output.Result{
			Timestamp:    time.Now(),
			Body:         nr.Body,
			URL:          nr.URL,
			Source:       nr.Source,
			Tag:          nr.Tag,
			Attribute:    nr.Attribute,
			CustomFields: nr.CustomFields,
		}
		if nr.Method != http.MethodGet {
			result.Method = nr.Method
		}
		scopeValidated, err := c.options.ScopeManager.Validate(parsed, nr.RootHostname)
		if err != nil {
			return
		}
		if scopeValidated || c.options.Options.DisplayOutScope {
			_ = c.options.OutputWriter.Write(result, nil)
		}
		if c.options.Options.OnResult != nil {
			c.options.Options.OnResult(*result)
		}
		// Do not add to crawl queue if max items are reached
		if nr.Depth >= c.options.Options.MaxDepth || !scopeValidated {
			return
		}
		queue.Push(nr, nr.Depth)
	}
}
