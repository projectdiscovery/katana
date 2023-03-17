package standard

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
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
	mapsutil "github.com/projectdiscovery/utils/maps"
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
		httpclient, _, err := common.BuildHttpClient(options.Dialer, options.Options, nil)
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

	queue, err := queue.New(c.options.Options.Strategy, c.options.Options.Timeout)
	if err != nil {
		return err
	}
	queue.Push(navigation.Request{Method: http.MethodGet, URL: rootURL, Depth: 0}, 0)

	if c.knownFiles != nil {
		navigationRequests, err := c.knownFiles.Request(rootURL)
		if err != nil {
			gologger.Warning().Msgf("Could not parse known files for %s: %s\n", rootURL, err)
		}
		c.enqueue(queue, navigationRequests...)
	}
	httpclient, _, err := common.BuildHttpClient(c.options.Dialer, c.options.Options, func(resp *http.Response, depth int) {
		body, _ := io.ReadAll(resp.Body)
		reader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		technologies := c.options.Wappalyzer.Fingerprint(resp.Header, body)
		navigationResponse := navigation.Response{
			Depth:        depth + 1,
			RootHostname: hostname,
			Resp:         resp,
			Body:         string(body),
			Reader:       reader,
			Technologies: mapsutil.GetKeys(technologies),
			StatusCode:   resp.StatusCode,
			Headers:      utils.FlattenHeaders(resp.Header),
		}
		navigationRequests := parser.ParseResponse(navigationResponse)
		c.enqueue(queue, navigationRequests...)
	})
	if err != nil {
		return errorutil.NewWithTag("standard", "could not create http client").Wrap(err)
	}

	wg := sizedwaitgroup.New(c.options.Options.Concurrency)
	for item := range queue.Pop() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}

		if !utils.IsURL(req.URL) {
			continue
		}

		if ok, err := c.options.ValidateScope(req.URL, hostname); err != nil || !ok {
			continue
		}
		if !c.options.ValidatePath(req.URL) {
			continue
		}

		wg.Add()

		go func() {
			defer wg.Done()

			c.options.RateLimit.Take()

			// Delay if the user has asked for it
			if c.options.Options.Delay > 0 {
				time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
			}
			resp, err := c.makeRequest(ctx, &req, hostname, req.Depth, httpclient)

			c.output(req, &resp, err)

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

			navigationRequests := parser.ParseResponse(resp)
			c.enqueue(queue, navigationRequests...)
		}()
	}
	wg.Wait()

	return nil
}

// makeParseResponseCallback returns a parse response function callback
func (c *Crawler) enqueue(queue *queue.Queue, navigationRequests ...navigation.Request) {
	for _, nr := range navigationRequests {
		if nr.URL == "" || !utils.IsURL(nr.URL) {
			continue
		}

		// Ignore blank URL items and only work on unique items
		if !c.options.UniqueFilter.UniqueURL(nr.RequestURL()) && len(nr.CustomFields) == 0 {
			continue
		}
		// - URLs stuck in a loop
		if c.options.UniqueFilter.IsCycle(nr.RequestURL()) {
			continue
		}

		scopeValidated := c.validateScope(nr.URL, nr.RootHostname)

		// Do not add to crawl queue if max items are reached
		if nr.Depth >= c.options.Options.MaxDepth || !scopeValidated {
			continue
		}
		queue.Push(nr, nr.Depth)
	}
}

func (c *Crawler) validateScope(URL string, root string) bool {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return false
	}
	scopeValidated, err := c.options.ScopeManager.Validate(parsedURL, root)
	return err == nil && scopeValidated
}

func (c *Crawler) output(navigationRequest navigation.Request, navigationResponse *navigation.Response, err error) {
	var errData string
	if err != nil {
		errData = err.Error()
	}
	// Write the found result to output
	result := &output.Result{
		Timestamp: time.Now(),
		Request:   navigationRequest,
		Response:  navigationResponse,
		Error:     errData,
	}

	_ = c.options.OutputWriter.Write(result)

	if c.options.Options.OnResult != nil {
		c.options.Options.OnResult(*result)
	}
}
