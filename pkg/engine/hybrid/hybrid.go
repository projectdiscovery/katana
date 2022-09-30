package hybrid

import (
	"errors"

	"github.com/projectdiscovery/katana/pkg/types"
)

type Crawler struct{}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	return &Crawler{}, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	return errors.New("not implemented")
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(url string) error {
	return errors.New("not implemented")
}
