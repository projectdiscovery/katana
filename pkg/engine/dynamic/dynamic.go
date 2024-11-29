package dynamic

import (
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/tfidf" // Import the new tfidf package
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
)

var (
	tfidfModel          *tfidf.TfIdf
	similarityThreshold float64 = 0.7
)

// Crawler is a dynamic crawler instance
type Crawler struct {
	*common.Shared
}

// New returns a new dynamic crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	shared, err := common.NewShared(options)
	if err != nil {
		return nil, errorutil.NewWithErr(err).WithTag("dynamic")
	}
	tfidfModel = tfidf.New()
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
		return errorutil.NewWithErr(err).WithTag("dynamic")
	}
	defer crawlSession.CancelFunc()
	gologger.Info().Msgf("Started dynamic crawling for => %v", rootURL)
	if err := c.Do(crawlSession, c.makeRequest); err != nil {
		return errorutil.NewWithErr(err).WithTag("dynamic")
	}
	return nil
}
