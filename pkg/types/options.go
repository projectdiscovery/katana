package types

import (
	"strings"

	"github.com/projectdiscovery/goflags"
)

type Options struct {
	// URLs contains a list of URLs for crawling
	URLs goflags.StringSlice
	// Scope contains a list of regexes for in-scope URLS
	Scope goflags.StringSlice
	// OutOfScope contains a list of regexes for out-scope URLS
	OutOfScope goflags.StringSlice
	// NoScope disables host based default scope
	NoScope bool
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
	// FormConfig is the path to the form configuration file
	FormConfig string
	// Proxy is the URL for the proxy server
	Proxy string
	// Strategy is the crawling strategy. depth-first or breadth-first
	Strategy string
	// FieldScope is the scope field for default DNS scope
	FieldScope string
	// OutputFile is the file to write output to
	OutputFile string
	// KnownFiles enables crawling of knows files like robots.txt, sitemap.xml, etc
	KnownFiles string
	// Fields is the fields to format in output
	Fields string
	// StoreFields is the fields to store in separate per-host files
	StoreFields string
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
	// Headless enables headless scraping
	Headless bool
	// UseInstalledChrome skips chrome install and use local instance
	UseInstalledChrome bool
	// ShowBrowser specifies whether the show the browser in headless mode
	ShowBrowser bool
}

func (options *Options) ParseCustomHeaders() map[string]string {
	customHeaders := make(map[string]string)
	for _, v := range options.CustomHeaders {
		if headerParts := strings.SplitN(v, ":", 2); len(headerParts) >= 2 {
			customHeaders[strings.Trim(headerParts[0], " ")] = strings.Trim(headerParts[1], " ")
		}
	}
	return customHeaders
}
