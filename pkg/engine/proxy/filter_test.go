package proxy

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
)

func TestProxyFilterPipeline_NewProxyFilterPipeline(t *testing.T) {
	tests := []struct {
		name     string
		config   *ProxyFilterConfig
		expected bool
	}{
		{
			name: "no proxy configured",
			config: &ProxyFilterConfig{
				Proxy: "",
			},
			expected: false,
		},
		{
			name: "proxy configured but no filters",
			config: &ProxyFilterConfig{
				Proxy: "http://127.0.0.1:8080",
			},
			expected: false,
		},
		{
			name: "proxy configured with filters but filtering disabled",
			config: &ProxyFilterConfig{
				Proxy:           "http://127.0.0.1:8080",
				ProxyFiltering:  false,
				ExtensionsMatch: []string{"php", "html"},
			},
			expected: false,
		},
		{
			name: "proxy configured with extension filters",
			config: &ProxyFilterConfig{
				Proxy:           "http://127.0.0.1:8080",
				ProxyFiltering:  true,
				ExtensionsMatch: []string{"php", "html"},
			},
			expected: true,
		},
		{
			name: "proxy configured with scope filters",
			config: &ProxyFilterConfig{
				Proxy:          "http://127.0.0.1:8080",
				ProxyFiltering: true,
				Scope:          []string{"example.com"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensionValidator := extensions.NewValidator(tt.config.ExtensionsMatch, tt.config.ExtensionFilter, false)
			scopeManager, _ := scope.NewManager(tt.config.Scope, tt.config.OutOfScope, "", false)
			
			pipeline := NewProxyFilterPipeline(tt.config, extensionValidator, scopeManager)
			
			if pipeline.IsEnabled() != tt.expected {
				t.Errorf("Expected enabled=%v, got enabled=%v", tt.expected, pipeline.IsEnabled())
			}
		})
	}
}

func TestProxyFilterPipeline_ShouldUseProxy(t *testing.T) {
	tests := []struct {
		name         string
		config       *ProxyFilterConfig
		url          string
		rootHostname string
		expected     bool
	}{
		{
			name: "filtering disabled - should use proxy",
			config: &ProxyFilterConfig{
				Proxy: "http://127.0.0.1:8080",
			},
			url:          "http://example.com/test.php",
			rootHostname: "example.com",
			expected:     true,
		},
		{
			name: "extension match filter - matching extension",
			config: &ProxyFilterConfig{
				Proxy:           "http://127.0.0.1:8080",
				ProxyFiltering:  true,
				ExtensionsMatch: []string{"php"},
			},
			url:          "http://example.com/test.php",
			rootHostname: "example.com",
			expected:     true,
		},
		{
			name: "extension match filter - non-matching extension",
			config: &ProxyFilterConfig{
				Proxy:           "http://127.0.0.1:8080",
				ProxyFiltering:  true,
				ExtensionsMatch: []string{"php"},
			},
			url:          "http://example.com/test.html",
			rootHostname: "example.com",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extensionValidator := extensions.NewValidator(tt.config.ExtensionsMatch, tt.config.ExtensionFilter, false)
			scopeManager, _ := scope.NewManager(tt.config.Scope, tt.config.OutOfScope, "", false)
			
			pipeline := NewProxyFilterPipeline(tt.config, extensionValidator, scopeManager)
			result := pipeline.ShouldUseProxy(tt.url, tt.rootHostname)
			
			if result != tt.expected {
				t.Errorf("Expected ShouldUseProxy=%v, got %v for URL %s", tt.expected, result, tt.url)
			}
		})
	}
}

func TestProxyFilterPipeline_ValidateExtensions(t *testing.T) {
	config := &ProxyFilterConfig{
		ProxyFiltering:  true,
		ExtensionsMatch: []string{"php", "html"},
	}
	
	extensionValidator := extensions.NewValidator(config.ExtensionsMatch, config.ExtensionFilter, false)
	pipeline := NewProxyFilterPipeline(config, extensionValidator, nil)

	tests := []struct {
		url      string
		expected bool
	}{
		{"http://example.com/test.php", true},
		{"http://example.com/test.html", true},
		{"http://example.com/test.js", false},
		{"http://example.com/test", false},
	}

	for _, tt := range tests {
		result := pipeline.ValidateExtensions(tt.url)
		if result != tt.expected {
			t.Errorf("ValidateExtensions(%s) = %v, expected %v", tt.url, result, tt.expected)
		}
	}
}

func TestProxyStats(t *testing.T) {
	stats := &ProxyStats{}
	
	// Test initial values
	if stats.GetStats().TotalRequests != 0 {
		t.Errorf("Expected initial TotalRequests=0, got %d", stats.GetStats().TotalRequests)
	}
	
	// Test increment operations
	stats.IncrementProxyRequests()
	stats.IncrementDirectRequests()
	stats.IncrementFilteredOut()
	
	result := stats.GetStats()
	if result.TotalRequests != 2 {
		t.Errorf("Expected TotalRequests=2, got %d", result.TotalRequests)
	}
	if result.ProxyRequests != 1 {
		t.Errorf("Expected ProxyRequests=1, got %d", result.ProxyRequests)
	}
	if result.DirectRequests != 1 {
		t.Errorf("Expected DirectRequests=1, got %d", result.DirectRequests)
	}
	if result.FilteredOut != 1 {
		t.Errorf("Expected FilteredOut=1, got %d", result.FilteredOut)
	}
}