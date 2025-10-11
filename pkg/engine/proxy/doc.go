// Package proxy provides enhanced proxy support with filtering capabilities for katana.
//
// This package implements a proxy filter pipeline that allows selective routing of HTTP requests
// through a proxy server based on various filtering criteria including:
//
// - Extension filtering (match/filter specific file extensions)
// - Scope filtering (in-scope/out-of-scope URL patterns)
// - Regex filtering (match/filter URL patterns using regular expressions)
// - Condition filtering (DSL-based conditional filtering)
//
// The package maintains backward compatibility with existing proxy functionality while
// adding intelligent filtering that reuses katana's existing filter logic.
//
// Key Components:
//
// - ProxyFilterPipeline: Core filtering logic that determines proxy usage
// - ProxyConfig: Manages both direct and proxy HTTP clients
// - RequestRouter: Interface for routing requests to appropriate clients
// - ProxyStats: Statistics tracking for proxy vs direct requests
//
// Usage:
//
//	// Create filter pipeline
//	pipeline := NewProxyFilterPipeline(options, extensionValidator, scopeManager)
//	
//	// Create proxy configuration
//	config, err := BuildHttpClientWithProxyFilter(dialer, options, pipeline, redirectCallback)
//	if err != nil {
//		return err
//	}
//	
//	// Route requests
//	client := config.GetClient(url, rootHostname)
//	resp, err := client.Do(request)
package proxy