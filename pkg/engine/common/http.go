package common

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/fastdialer/fastdialer/ja3/impersonate"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/utils/errkit"
	proxyutil "github.com/projectdiscovery/utils/proxy"
)

type RedirectCallback func(resp *http.Response, depth int)

// RedirectScope reports whether a redirect target is in the crawl scope.
type RedirectScope func(u *url.URL) bool

// BuildHttpClient builds a http client based on a profile.
// Custom -H headers are removed from a redirect hop whose target fails
// inScope; with a nil inScope they stay on the origin of the first request.
func BuildHttpClient(dialer *fastdialer.Dialer, options *types.Options, redirectCallback RedirectCallback, inScope RedirectScope) (*retryablehttp.Client, *fastdialer.Dialer, error) {
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

	customHeaders := options.ParseCustomHeaders()

	client := retryablehttp.NewWithHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if options.DisableRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) == 10 {
				return errkit.New("stopped after 10 redirects")
			}
			// Go copies every header of the first request to each hop, so
			// remove the operator's -H values before they reach a host
			// outside the crawl scope.
			if !redirectHeadersAllowed(req.URL, via, inScope) {
				stripCustomHeaders(req.Header, customHeaders)
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

// redirectHeadersAllowed reports whether custom headers may go with a
// redirect to target.
func redirectHeadersAllowed(target *url.URL, via []*http.Request, inScope RedirectScope) bool {
	if inScope != nil {
		return inScope(target)
	}
	return len(via) > 0 && sameOrigin(via[0].URL, target)
}

// sameOrigin compares scheme, hostname and effective port.
func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}

// stripCustomHeaders removes every custom header name from h.
func stripCustomHeaders(h http.Header, customHeaders map[string]string) {
	for name := range customHeaders {
		h.Del(name)
	}
}
