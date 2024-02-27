package main

import (
	"math"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/levels"
	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
)

func main() {
	gologger.DefaultLogger.SetMaxLevel(levels.LevelSilent)

	options := &types.Options{
		MaxDepth:         3,           // Maximum depth to crawl
		FieldScope:       "rdn",       // Crawling Scope Field
		BodyReadSize:     math.MaxInt, // Maximum response size to read
		Timeout:          10,          // Timeout is the time to wait for request in seconds
		Concurrency:      10,          // Concurrency is the number of concurrent crawling goroutines
		OutputMatchRegex: []string{"policies"},
		Parallelism:      10,            // Parallelism is the number of urls processing goroutines
		Delay:            0,             // Delay is the delay between each crawl requests in seconds
		RateLimit:        150,           // Maximum requests to send per second
		Strategy:         "depth-first", // Visit strategy (depth-first, breadth-first)
		OnResult: func(result output.Result) { // Callback function to execute for result
			gologger.Info().Msg(result.Request.URL)
		},
	}
	crawlerOptions, err := types.NewCrawlerOptions(options)
	if err != nil {
		gologger.Fatal().Msg(err.Error())
	}
	defer crawlerOptions.Close()
	crawler, err := standard.New(crawlerOptions)
	if err != nil {
		gologger.Fatal().Msg(err.Error())
	}
	defer crawler.Close()
	var input = "https://www.hackerone.com"
	err = crawler.Crawl(input)
	if err != nil {
		gologger.Warning().Msgf("Could not crawl %s: %s", input, err.Error())
	}
}
