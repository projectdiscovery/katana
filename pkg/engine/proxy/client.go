package proxy

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/fastdialer/fastdialer/ja3/impersonate"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/utils/errkit"
	proxyutil "github.com/projectdiscovery/utils/proxy"
)

// ProxyConfig manages both direct and proxy HTTP clients
type ProxyConfig struct {
	ProxyURL        *url.URL
	FilterPipeline  *ProxyFilterPipeline
	DirectClient    *retryablehttp.Client
	ProxyClient     *retryablehttp.Client
	Stats           *ProxyStats
}

// RedirectCallback defines the callback function for handling redirects
type RedirectCallback func(resp *http.Response, depth int)

// HttpClientConfig contains configuration for HTTP clients
type HttpClientConfig struct {
	Proxy               string
	Timeout             int
	Retries             int
	TlsImpersonate      bool
	DisableRedirects    bool
}

// BuildHttpClientWithProxyFilter creates HTTP clients with proxy filtering support
func BuildHttpClientWithProxyFilter(dialer *fastdialer.Dialer, config *HttpClientConfig, filterPipeline *ProxyFilterPipeline, redirectCallback RedirectCallback) (*ProxyConfig, error) {
	// Validate input parameters
	if dialer == nil {
		return nil, errkit.New("dialer cannot be nil")
	}
	if config == nil {
		return nil, errkit.New("config cannot be nil")
	}

	proxyConfig := &ProxyConfig{
		FilterPipeline: filterPipeline,
		Stats:          &ProxyStats{},
	}

	// Parse proxy URL if provided
	if config.Proxy != "" {
		proxyURL, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, errkit.Wrap(err, "invalid proxy URL format")
		}
		
		// Validate proxy URL scheme
		if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" && proxyURL.Scheme != "socks5" {
			return nil, errkit.New("proxy URL must use http, https, or socks5 scheme")
		}
		
		proxyConfig.ProxyURL = proxyURL
		gologger.Debug().Msgf("Proxy client: Configured proxy URL: %s", proxyURL.String())
	}

	// Create direct client (no proxy) - this is critical and must succeed
	directClient, err := buildClient(dialer, config, nil, redirectCallback)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create direct HTTP client - this is required for fallback")
	}
	proxyConfig.DirectClient = directClient
	gologger.Debug().Msgf("Proxy client: Direct HTTP client created successfully")

	// Create proxy client if proxy is configured
	if proxyConfig.ProxyURL != nil {
		proxyClient, err := buildClient(dialer, config, proxyConfig.ProxyURL, redirectCallback)
		if err != nil {
			gologger.Warning().Msgf("Failed to create proxy HTTP client: %v", err)
			gologger.Warning().Msgf("All requests will be sent directly (no proxy) due to proxy client creation failure")
			// Fall back to direct client for all requests
			proxyConfig.ProxyClient = directClient
		} else {
			proxyConfig.ProxyClient = proxyClient
			gologger.Debug().Msgf("Proxy client: Proxy HTTP client created successfully")
		}
	} else {
		// No proxy configured, use direct client for both
		proxyConfig.ProxyClient = directClient
		gologger.Debug().Msgf("Proxy client: No proxy configured, using direct client for all requests")
	}

	return proxyConfig, nil
}

// GetClient returns the appropriate HTTP client based on filter results
func (pc *ProxyConfig) GetClient(requestURL string, rootHostname string) *retryablehttp.Client {
	// Validate input parameters
	if requestURL == "" {
		gologger.Warning().Msgf("Proxy client: Empty URL provided, using direct client")
		pc.Stats.IncrementDirectRequests()
		return pc.DirectClient
	}

	// If no proxy is configured, always use direct client
	if pc.ProxyURL == nil {
		pc.Stats.IncrementDirectRequests()
		return pc.DirectClient
	}

	// Ensure we have a valid direct client as fallback
	if pc.DirectClient == nil {
		gologger.Error().Msgf("Proxy client: Direct client is nil, this should not happen")
		// This is a critical error, but we'll try to use proxy client as fallback
		if pc.ProxyClient != nil {
			pc.Stats.IncrementProxyRequests()
			return pc.ProxyClient
		}
		// Both clients are nil - this is a serious configuration error
		gologger.Fatal().Msgf("Proxy client: Both direct and proxy clients are nil")
		return nil
	}

	// If filtering is disabled or no filter pipeline, use proxy for all requests
	if pc.FilterPipeline == nil || !pc.FilterPipeline.IsEnabled() {
		// Ensure proxy client is available
		if pc.ProxyClient == nil {
			gologger.Warning().Msgf("Proxy client: Proxy client is nil, falling back to direct client")
			pc.Stats.IncrementDirectRequests()
			return pc.DirectClient
		}
		pc.Stats.IncrementProxyRequests()
		return pc.ProxyClient
	}

	// Apply filtering logic with error handling
	shouldUseProxy := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				gologger.Error().Msgf("Proxy filter: Panic during filtering for URL '%s': %v", requestURL, r)
				shouldUseProxy = false // Default to direct on panic
			}
		}()
		shouldUseProxy = pc.FilterPipeline.ShouldUseProxy(requestURL, rootHostname)
	}()

	if shouldUseProxy {
		// Ensure proxy client is available
		if pc.ProxyClient == nil {
			gologger.Warning().Msgf("Proxy client: Proxy client is nil, falling back to direct client for URL '%s'", requestURL)
			pc.Stats.IncrementDirectRequests()
			return pc.DirectClient
		}
		pc.Stats.IncrementProxyRequests()
		return pc.ProxyClient
	} else {
		pc.Stats.IncrementDirectRequests()
		return pc.DirectClient
	}
}

// IsProxyEnabled returns true if proxy is configured
func (pc *ProxyConfig) IsProxyEnabled() bool {
	return pc.ProxyURL != nil
}

// GetProxyStats returns current proxy statistics
func (pc *ProxyConfig) GetProxyStats() ProxyStats {
	return pc.Stats.GetStats()
}

// buildClient creates a retryable HTTP client with the specified configuration
func buildClient(dialer *fastdialer.Dialer, config *HttpClientConfig, proxyURL *url.URL, redirectCallback RedirectCallback) (*retryablehttp.Client, error) {
	// Single Host
	retryablehttpOptions := retryablehttp.DefaultOptionsSingle
	retryablehttpOptions.RetryMax = config.Retries

	transport := &http.Transport{
		DialContext: dialer.Dial,
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if config.TlsImpersonate {
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

	// Configure proxy if provided
	if proxyURL != nil {
		if ok, err := proxyutil.IsBurp(proxyURL.String()); err == nil && ok {
			transport.TLSClientConfig.MaxVersion = tls.VersionTLS12
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := retryablehttp.NewWithHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   time.Duration(config.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if config.DisableRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) == 10 {
				return errkit.New("stopped after 10 redirects")
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

	return client, nil
}