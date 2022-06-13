package standard

import "github.com/projectdiscovery/katana/pkg/types"

// Crawler is a standard crawler instance
type Crawler struct {
	options *types.Options
}

// New returns a new standard crawler instance
func New(options *types.Options) *Crawler {
	return &Crawler{options: options}
}

func (c *Crawler) Crawl(url string) {

}
