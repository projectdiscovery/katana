package main

import (
	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/types"
)

var libraryTestcases = map[string]TestCase{
	"katana as library": &goIntegrationTest{},
}

type goIntegrationTest struct{}

// Execute executes a test case and returns an error if occurred
// Execute the docs at ../README.md if the code stops working for integration.
func (h *goIntegrationTest) Execute() error {
	options := &types.Options{
		MaxDepth:     1,
		FieldScope:   "rdn",
		BodyReadSize: 2 * 1024 * 1024,
		RateLimit:    150,
		Verbose:      debug,
	}
	crawlerOptions, err := types.NewCrawlerOptions(options)
	if err != nil {
		return err
	}
	defer crawlerOptions.Close()
	crawler, err := standard.New(crawlerOptions)
	if err != nil {
		return err
	}
	defer crawler.Close()
	var input = "https://tesla.com"
	return crawler.Crawl(input)
}
