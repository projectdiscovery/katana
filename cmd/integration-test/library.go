package main

import (
	"fmt"
	"math"

	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/output"
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
	var crawledURLs []string

	options := &types.Options{
		MaxDepth:     1,
		FieldScope:   "rdn",
		BodyReadSize: math.MaxInt,
		RateLimit:    150,
		Verbose:      debug,
		Strategy:     queue.DepthFirst.String(),
		OnResult: func(r output.Result) {
			crawledURLs = append(crawledURLs, r.Request.URL)
		},
	}
	crawlerOptions, err := types.NewCrawlerOptions(options)
	if err != nil {
		return err
	}
	defer func() {
		if err := crawlerOptions.Close(); err != nil {
			fmt.Printf("Error closing crawler options: %v\n", err)
		}
	}()
	crawler, err := standard.New(crawlerOptions)
	if err != nil {
		return err
	}
	defer func() {
		if err := crawler.Close(); err != nil {
			fmt.Printf("Error closing crawler: %v\n", err)
		}
	}()
	var input = "https://public-firing-range.appspot.com"
	err = crawler.Crawl(input)
	if err != nil {
		return err
	}
	if len(crawledURLs) == 0 {
		return fmt.Errorf("no URLs crawled")
	}
	return nil
}
