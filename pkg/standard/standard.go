package standard

// Crawler is a standard crawler instance
type Crawler struct {
	options *Options
}

// Options contains the options for the standard crawler
type Options struct {
	// MaxDepth is the maximum depth to crawl
	MaxDepth int
	// BodyReadSize is the maximum size of response body to read
	BodyReadSize int
	// Timeout is the time to wait for request in seconds
	Timeout int
	// Concurrency is the number of concurrent crawling goroutines
	Concurrency int
	// Proxy is the URL for the proxy server
	Proxy string
	// CustomHeaders is a list of custom headers to add to request
	CustomHeaders map[string]string
}

// New returns a new standard crawler instance
func New(options *Options) *Crawler {
	return &Crawler{options: options}
}

func (c *Crawler) Crawl(url string) {

}
