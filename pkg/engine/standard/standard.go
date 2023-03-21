package standard

import (
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	errorutil "github.com/projectdiscovery/utils/errors"
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
	crawlSession, err := c.NewCrawlSessionWithURL(rootURL)
	if err != nil {
		return errorutil.NewWithErr(err).WithTag("standard")
	}

	wg := sizedwaitgroup.New(c.Options.Options.Concurrency)
	for item := range crawlSession.Queue.Pop() {
		if ctxErr := crawlSession.Ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}

		if !utils.IsURL(req.URL) {
			continue
		}

		if ok, err := c.Options.ValidateScope(req.URL, crawlSession.Hostname); err != nil || !ok {
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
			resp, err := c.makeRequest(crawlSession.Ctx, &req, crawlSession.Hostname, req.Depth, crawlSession.HttpClient)

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
			c.Enqueue(crawlSession.Queue, navigationRequests...)
		}()
	}
	wg.Wait()

	return nil
}
