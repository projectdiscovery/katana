package types

import (
	"strings"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/katana/pkg/output"
)

// OnResultCallback (output.Result)
type OnResultCallback func(output.Result)

type Options struct {
	// URLs contains a list of URLs for crawling
	URLs goflags.StringSlice
	// Scope contains a list of regexes for in-scope URLS
	Scope goflags.StringSlice
	// OutOfScope contains a list of regexes for out-scope URLS
	OutOfScope goflags.StringSlice
	// NoScope disables host based default scope
	NoScope bool
	// DisplayOutScope displays out of scope items in results
	DisplayOutScope bool
	// ExtensionsMatch contains extensions to match explicitly
	ExtensionsMatch goflags.StringSlice
	// ExtensionFilter contains additional items for filter list
	ExtensionFilter goflags.StringSlice
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
	// FieldConfig is the path to the custom field configuration file
	FieldConfig string
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
	// AutomaticFormFill enables optional automatic form filling and submission
	AutomaticFormFill bool
	// UseInstalledChrome skips chrome install and use local instance
	UseInstalledChrome bool
	// ShowBrowser specifies whether the show the browser in headless mode
	ShowBrowser bool
	// HeadlessOptionalArguments specifies optional arguments to pass to Chrome
	HeadlessOptionalArguments goflags.StringSlice
	// HeadlessNoSandbox specifies if chrome should be start in --no-sandbox mode
	HeadlessNoSandbox bool
	// SystemChromePath : Specify the chrome binary path for headless crawling
	SystemChromePath string
	// OnResult allows callback function on a result
	OnResult OnResultCallback
	// StoreResponse specifies if katana should store http requests/responses
	StoreResponse bool
	// StoreResponseDir specifies if katana should use a custom directory to store http requests/responses
	StoreResponseDir string
	// HeadlessNoIncognito specifies if chrome should be started without incognito mode
	HeadlessNoIncognito bool
	// HealthCheck determines if a self-healthcheck should be performed
	HealthCheck bool
	// ErrorLogFile specifies a file to write with the errors of all requests
	ErrorLogFile string
	// OpenBrowser opens visible browser window to login
	NewProject string
	// CrawlProject uses saved session data while crawling
	CrawlProject string
	// ListProject lists all project
	ListProject bool
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

func (options *Options) ParseHeadlessOptionalArguments() map[string]string {
	optionalArguments := make(map[string]string)
	for _, v := range options.HeadlessOptionalArguments {
		if argParts := strings.SplitN(v, "=", 2); len(argParts) >= 2 {
			key := strings.TrimSpace(argParts[0])
			value := strings.TrimSpace(argParts[1])
			if key != "" && value != "" {
				optionalArguments[key] = value
			}
		}
	}
	return optionalArguments
}
