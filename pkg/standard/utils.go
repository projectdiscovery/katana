package standard

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
)

// buildClient builds a http client based on a profile
func buildClient(options *types.Options) (*retryablehttp.Client, *fastdialer.Dialer, error) {
	dialer, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not create dialer")
	}

	var proxyURL *url.URL
	if options.Proxy != "" {
		proxyURL, err = url.Parse(options.Proxy)
	}
	if err != nil {
		return nil, nil, err
	}

	// Single Host
	retryablehttpOptions := retryablehttp.DefaultOptionsSingle
	retryablehttpOptions.RetryMax = options.Retries
	transport := &http.Transport{
		DialContext:         dialer.Dial,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     100,
		TLSClientConfig: &tls.Config{
			Renegotiation:      tls.RenegotiateOnceAsClient,
			InsecureSkipVerify: true,
		},
		DisableKeepAlives: false,
	}

	// Attempts to overwrite the dial function with the socks proxied version
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := retryablehttp.NewWithHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
	}, retryablehttpOptions)
	client.CheckRetry = retryablehttp.HostSprayRetryPolicy()
	return client, dialer, nil
}
