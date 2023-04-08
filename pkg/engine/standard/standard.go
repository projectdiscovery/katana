package standard

import (
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
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
	defer crawlSession.CancelFunc()
	gologger.Info().Msgf("Started standard crawling for => %v", rootURL)
	if err := c.Do(crawlSession, c.makeRequest); err != nil {
		return errorutil.NewWithErr(err).WithTag("standard")
	}
	return nil
}
