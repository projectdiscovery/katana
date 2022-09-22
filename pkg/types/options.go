package types

import "github.com/projectdiscovery/goflags"

type Options struct {
	// URLs contains a list of URLs for crawling
	URLs goflags.StringSlice
	// Scope contains a list of regexes for in-scope hosts
	Scope goflags.StringSlice
	// OutOfScope contains a list of regexes for out-scope hosts
	OutOfScope goflags.StringSlice
	// IncludeSubdomains specifies if we if want to include subdomains for scope
	IncludeSubdomains bool
	// Extensions is a list of extensions to be allowed. Can be * for all extensions.
	Extensions goflags.StringSlice
	// ExtensionsAllowList contains any extensions to allow from default deny list
	ExtensionsAllowList goflags.StringSlice
	// ExtensionDenyList contains additional items for deny list
	ExtensionDenyList goflags.StringSlice
	// MaxDepth is the maximum depth to crawl
	MaxDepth int
	// BodyReadSize is the maximum size of response body to read
	BodyReadSize int
	// Timeout is the time to wait for request in seconds
	Timeout int
	// CrawlDuration is the duration in seconds to crawl target from
	CrawlDuration int
	// Delay is the delay between each crawl requests in seconds
	Delay int
	// RateLimit is the maximum number of requests to send per second
	RateLimit int
	// Retries is the number of retries to do for request
	Retries int
	// RateLimitMinute is the maximum number of requests to send per minute
	RateLimitMinute int
	// Concurrency is the number of concurrent crawling goroutines
	Concurrency int
	// Parallelism is the number of urls processing goroutines
	Parallelism int
	// Proxy is the URL for the proxy server
	Proxy string
	// Strategy is the crawling strategy. depth-first or breadth-first
	Strategy string
	// OutputFile is the file to write output to
	OutputFile string
	// NoColors disables coloring of response output
	NoColors bool
	// JSON enables writing output in JSON format
	JSON bool
	// Silent shows only output
	Silent bool
	// Verbose specifies showing verbose output
	Verbose bool
	// Version enables showing of crawler version
	Version bool
	// ScrapeJSResponses enables scraping of relative endpoints from javascript
	ScrapeJSResponses bool
	// CustomHeaders is a list of custom headers to add to request
	CustomHeaders goflags.StringSlice
}
