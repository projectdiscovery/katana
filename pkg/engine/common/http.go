package common

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/fastdialer/fastdialer/ja3/impersonate"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	proxyutil "github.com/projectdiscovery/utils/proxy"
)

type RedirectCallback func(resp *http.Response, depth int)

// BuildClient builds a http client based on a profile
func BuildHttpClient(dialer *fastdialer.Dialer, options *types.Options, redirectCallback RedirectCallback) (*retryablehttp.Client, *fastdialer.Dialer, error) {
	// Single Host
	retryablehttpOptions := retryablehttp.DefaultOptionsSingle
	retryablehttpOptions.RetryMax = options.Retries
	transport := &http.Transport{
		DialContext: dialer.Dial,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if options.TlsImpersonate {
				return dialer.DialTLSWithConfigImpersonate(ctx, network, addr, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10}, impersonate.Random, nil)
			}
			return dialer.DialTLS(ctx, network, addr)
		},
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
	if proxyURL, err := url.Parse(options.Proxy); options.Proxy != "" && err == nil {
		if ok, err := proxyutil.IsBurp(options.Proxy); err == nil && ok {
			transport.TLSClientConfig.MaxVersion = tls.VersionTLS12
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := retryablehttp.NewWithHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if options.DisableRedirects {
				return http.ErrUseLastResponse
			}
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
