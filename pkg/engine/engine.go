package engine

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/engine/headless"
	"github.com/projectdiscovery/katana/pkg/engine/simple"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/parser"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/stringsutil"
)

// Crawler is a crawler instance
type Crawler struct {
	Options    *types.CrawlerOptions
	httpclient *retryablehttp.Client
	dialer     *fastdialer.Dialer
}

// New returns a new crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	httpclient, dialer, err := buildClient(options.Options)
	if err != nil {
		return nil, errors.Wrap(err, "could not create http client")
	}

	crawler := &Crawler{
		Options:    options,
		dialer:     dialer,
		httpclient: httpclient,
	}

	return crawler, nil
}

// Close closes the crawler process
func (crawler *Crawler) Close() {
	crawler.dialer.Close()
}

// Crawl crawls a URL with the specified options
func (crawler *Crawler) Crawl(url string) error {
	ctx, cancel := context.WithCancel(context.Background())
	if crawler.Options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(crawler.Options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	simpleEngine, err := simple.NewWithClients(crawler.Options, crawler.dialer, crawler.httpclient)
	if err != nil {
		return errors.Wrap(err, "could not create simple engine")
	}
	defer simpleEngine.Close()

	var headlessEngine *headless.HeadlessEngine
	if crawler.Options.Options.Headless {
		// we use different browsers instances with shared context
		var err error
		headlessEngine, err = headless.NewWithClients(crawler.Options, crawler.dialer, crawler.httpclient)
		if err != nil {
			return errors.Wrap(err, "could not create simple engine")
		}
		defer headlessEngine.Close()
	}

	queue := queue.New(crawler.Options.Options.Strategy)
	queue.Push(navigation.Request{Method: http.MethodGet, URL: url, Depth: 0}, 0)

	for {
		// Quit the crawling for zero items or context timeout
		if queue.Len() == 0 {
			return io.EOF
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		item := queue.Pop()
		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}
		crawler.Options.RateLimit.Take()

		// Delay if the user has asked for it
		if crawler.Options.Options.Delay > 0 {
			time.Sleep(time.Duration(crawler.Options.Options.Delay) * time.Second)
		}

		var responses []navigation.Response

		respSimple, errSimple := simpleEngine.MakeRequest(req)
		if errSimple != nil {
			return errors.Wrap(errSimple, "Could not request seed URL: %s\n")
		}
		if !crawler.shouldSkipResponse(respSimple) {
			responses = append(responses, respSimple)
		}

		// additionally crawls headless if requested
		canUseHeadless := stringsutil.EqualFoldAny(req.Method, http.MethodGet)
		if crawler.Options.Options.Headless && canUseHeadless {
			respHeadless, err := headlessEngine.MakeRequest(req)
			if err != nil {
				return errors.Wrap(err, "Could not request headless seed URL: %s\n")
			}
			if !crawler.shouldSkipResponse(respHeadless) {
				responses = append(responses, respHeadless)
			}
		}

		crawler.parseResponses(queue, req, responses...)
	}
}

func (crawler *Crawler) shouldSkipResponse(response navigation.Response) bool {
	return response.Resp == nil || response.Reader == nil
}

func (crawler *Crawler) parseResponses(queue *queue.VarietyQueue, request navigation.Request, responses ...navigation.Response) {
	for _, response := range responses {
		parser.ParseResponse(response, func(nr navigation.Request) {
			// Ignore blank URL items
			if nr.URL == "" {
				return
			}
			// Only work on unique items
			if !crawler.Options.UniqueFilter.Unique(nr.RequestURL()) {
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
			_ = crawler.Options.OutputWriter.Write(result)

			// Do not add to crawl queue if max items are reached
			if nr.Depth >= crawler.Options.Options.MaxDepth {
				return
			}
			queue.Push(nr, nr.Depth)
		})
	}
}
