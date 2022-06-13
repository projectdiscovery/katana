package types

type Options struct {
	// URLs contains a list of URLs for crawling
	URLs []string
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
	// RateLimitMinute is the maximum number of requests to send per minute
	RateLimitMinute int
	// Concurrency is the number of concurrent crawling goroutines
	Concurrency int
	// Proxy is the URL for the proxy server
	Proxy string
	// OutputFile is the file to write output to
	OutputFile string
	// NoColors disables coloring of response output
	NoColors bool
	// JSON enables writing output in JSON format
	JSON bool
	// ScrapeJSResponses enables scraping of relative endpoints from javascript
	ScrapeJSResponses bool
	// CustomHeaders is a list of custom headers to add to request
	CustomHeaders map[string]string
}
