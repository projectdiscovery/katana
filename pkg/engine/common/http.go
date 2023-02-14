package common

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
)

type RedirectCallback func(resp *http.Response, depth int)

// BuildClient builds a http client based on a profile
func BuildHttpClient(dialer *fastdialer.Dialer, options *types.Options, redirectCallback RedirectCallback) (*retryablehttp.Client, *fastdialer.Dialer, error) {
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

	if options.Proxy != "" {
		if proxyURL, err := url.Parse(options.Proxy); err != nil {
			return nil, nil, err
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	client := retryablehttp.NewWithHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 10 {
				return errorutil.New("stopped after 10 redirects")
			}
			depth, ok := req.Context().Value(navigation.Depth{}).(int)
			if !ok {
				depth = 2
			}
			if redirectCallback != nil {
				redirectCallback(req.Response, depth)
			}
			return nil
		},
	}, retryablehttpOptions)
	client.CheckRetry = retryablehttp.HostSprayRetryPolicy()
	return client, dialer, nil
}
