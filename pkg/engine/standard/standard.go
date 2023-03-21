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
	*common.Shared
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	shared, err := common.NewShared(options)
	if err != nil {
		return nil, errorutil.NewWithErr(err).WithTag("standard")
	}
	return &Crawler{Shared: shared}, nil
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
	if c.Options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.Options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	queue, err := queue.New(c.Options.Options.Strategy, c.Options.Options.Timeout)
	if err != nil {
		return err
	}
	queue.Push(navigation.Request{Method: http.MethodGet, URL: rootURL, Depth: 0}, 0)

	if c.KnownFiles != nil {
		navigationRequests, err := c.KnownFiles.Request(rootURL)
		if err != nil {
			gologger.Warning().Msgf("Could not parse known files for %s: %s\n", rootURL, err)
		}
		c.Enqueue(queue, navigationRequests...)
	}
	httpclient, _, err := common.BuildHttpClient(c.Options.Dialer, c.Options.Options, func(resp *http.Response, depth int) {
		body, _ := io.ReadAll(resp.Body)
		reader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		technologies := c.Options.Wappalyzer.Fingerprint(resp.Header, body)
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
		c.Enqueue(queue, navigationRequests...)
	})
	if err != nil {
		return errorutil.NewWithTag("standard", "could not create http client").Wrap(err)
	}

	wg := sizedwaitgroup.New(c.Options.Options.Concurrency)
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

		if ok, err := c.Options.ValidateScope(req.URL, hostname); err != nil || !ok {
			continue
		}
		if !c.Options.ValidatePath(req.URL) {
			continue
		}

		wg.Add()

		go func() {
			defer wg.Done()

			c.Options.RateLimit.Take()

			// Delay if the user has asked for it
			if c.Options.Options.Delay > 0 {
				time.Sleep(time.Duration(c.Options.Options.Delay) * time.Second)
			}
			resp, err := c.makeRequest(ctx, &req, hostname, req.Depth, httpclient)

			c.Output(req, &resp, err)

			if err != nil {
				gologger.Warning().Msgf("Could not request seed URL %s: %s\n", req.URL, err)
				outputError := &output.Error{
					Timestamp: time.Now(),
					Endpoint:  req.RequestURL(),
					Source:    req.Source,
					Error:     err.Error(),
				}
				_ = c.Options.OutputWriter.WriteErr(outputError)
				return
			}
			if resp.Resp == nil || resp.Reader == nil {
				return
			}

			navigationRequests := parser.ParseResponse(resp)
			c.Enqueue(queue, navigationRequests...)
		}()
	}
	wg.Wait()

	return nil
}
