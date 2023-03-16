package main

import (
	"math"

	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
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
		BodyReadSize: math.MaxInt,
		RateLimit:    150,
		Verbose:      debug,
		Strategy:     queue.DepthFirst.String(),
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
	var input = "https://public-firing-range.appspot.com"
	return crawler.Crawl(input)
}
