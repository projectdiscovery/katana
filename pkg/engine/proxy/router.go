package proxy

import (
	"github.com/projectdiscovery/retryablehttp-go"
)

// RequestRouter defines the interface for routing requests to appropriate clients
type RequestRouter interface {
	// RouteRequest determines which client to use for a given URL and returns the client and whether proxy was used
	RouteRequest(url string, rootHostname string) (*retryablehttp.Client, bool)
	// IsProxyEnabled returns true if proxy functionality is enabled
	IsProxyEnabled() bool
	// GetProxyStats returns current proxy usage statistics
	GetProxyStats() ProxyStats
}

// ProxyRouter implements the RequestRouter interface using ProxyConfig
type ProxyRouter struct {
	config *ProxyConfig
}

// NewProxyRouter creates a new proxy router instance
func NewProxyRouter(config *ProxyConfig) *ProxyRouter {
	return &ProxyRouter{
		config: config,
	}
}

// RouteRequest determines which client to use for the given URL
func (pr *ProxyRouter) RouteRequest(url string, rootHostname string) (*retryablehttp.Client, bool) {
	if pr.config == nil {
		// Fallback to direct client if no config
		return nil, false
	}

	client := pr.config.GetClient(url, rootHostname)
	
	// Determine if proxy was used by comparing client instances
	usingProxy := pr.config.IsProxyEnabled() && client == pr.config.ProxyClient
	
	return client, usingProxy
}

// IsProxyEnabled returns true if proxy is configured
func (pr *ProxyRouter) IsProxyEnabled() bool {
	if pr.config == nil {
		return false
	}
	return pr.config.IsProxyEnabled()
}

// GetProxyStats returns current proxy statistics
func (pr *ProxyRouter) GetProxyStats() ProxyStats {
	if pr.config == nil {
		return ProxyStats{}
	}
	return pr.config.GetProxyStats()
}