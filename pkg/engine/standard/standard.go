package standard

import (
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/parser"
	"github.com/projectdiscovery/katana/pkg/types"
)

// Crawler is the standard HTTP crawler
type Crawler struct {
	options *types.Options
	parser  parser.Parser
}

// New creates a new standard crawler
func New(options *types.Options) (*Crawler, error) {
	return &Crawler{
		options: options,
		parser:  parser.GetParser(),
	}, nil
}

// Crawl starts the crawling process
func (c *Crawler) Crawl(rootURL string) (<-chan navigation.Result, error) {
	out := make(chan navigation.Result)
	
	go func() {
		defer close(out)
		
		// Perform standard HTTP crawling
		resp := &navigation.Response{
			URL: rootURL,
		}
		
		// Parse response for links
		parsedResp, err := c.parser.Parse(resp)
		if err != nil {
			return
		}
		
		// Process and output results
		for _, link := range parsedResp.Links {
			out <- navigation.Result{
				Request:  &navigation.Request{URL: rootURL},
				Response: parsedResp,
				Link:     link,
			}
		}
	}()
	
	return out, nil
}
