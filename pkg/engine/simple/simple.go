package simple

import (
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
)

type SimpleEngine struct {
	options    *types.CrawlerOptions
	httpclient *retryablehttp.Client
	dialer     *fastdialer.Dialer
}

// New returns a new crawler instance
func New(options *types.CrawlerOptions) (*SimpleEngine, error) {
	return NewWithClients(options, nil, nil)
}

// New returns a new crawler instance
func NewWithClients(options *types.CrawlerOptions, dialer *fastdialer.Dialer, httpClient *retryablehttp.Client) (*SimpleEngine, error) {
	simpleEngine := &SimpleEngine{
		options:    options,
		dialer:     dialer,
		httpclient: httpClient,
	}
	return simpleEngine, nil
}

// Close closes the crawler process
func (simpleEngine *SimpleEngine) Close() {
	simpleEngine.dialer.Close()
}
