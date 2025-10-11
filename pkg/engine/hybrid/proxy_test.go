package hybrid

import (
	"testing"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/katana/pkg/types"
)

func TestHybridProxyFiltering(t *testing.T) {
	tests := []struct {
		name           string
		options        *types.Options
		expectFiltering bool
	}{
		{
			name: "hybrid without proxy filtering",
			options: &types.Options{
				Headless:    true,
				Proxy:       "http://127.0.0.1:8080",
				MaxDepth:    1,
				Concurrency: 1,
				Timeout:     10,
			},
			expectFiltering: false,
		},
		{
			name: "hybrid with proxy filtering enabled",
			options: &types.Options{
				Headless:        true,
				Proxy:           "http://127.0.0.1:8080",
				ProxyFiltering:  true,
				ExtensionsMatch: goflags.StringSlice{"php", "html"},
				MaxDepth:        1,
				Concurrency:     1,
				Timeout:         10,
			},
			expectFiltering: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create crawler options
			crawlerOptions, err := types.NewCrawlerOptions(tt.options)
			if err != nil {
				t.Fatalf("Failed to create crawler options: %v", err)
			}
			defer crawlerOptions.Close()

			// Verify proxy filtering configuration
			if tt.expectFiltering {
				if crawlerOptions.ProxyFilterPipeline == nil {
					t.Error("ProxyFilterPipeline should be initialized when proxy filtering is enabled")
				}
				if !crawlerOptions.ProxyFilterPipeline.IsEnabled() {
					t.Error("ProxyFilterPipeline should be enabled when proxy filtering is configured")
				}
			}

			// Note: We can't easily test the actual hybrid crawler creation without Chrome
			// but we can verify the configuration is set up correctly
			t.Logf("Hybrid proxy filtering configuration verified for: %s", tt.name)
		})
	}
}

func TestHybridProxyDecision(t *testing.T) {
	options := &types.Options{
		Headless:        true,
		Proxy:           "http://127.0.0.1:8080",
		ProxyFiltering:  true,
		ExtensionsMatch: goflags.StringSlice{"php"},
		MaxDepth:        1,
		Concurrency:     1,
		Timeout:         10,
		Debug:           true,
	}

	crawlerOptions, err := types.NewCrawlerOptions(options)
	if err != nil {
		t.Fatalf("Failed to create crawler options: %v", err)
	}
	defer crawlerOptions.Close()

	// Test proxy decision logic
	tests := []struct {
		url      string
		expected bool
	}{
		{"http://example.com/test.php", true},  // Should use proxy
		{"http://example.com/test.html", false}, // Should not use proxy
		{"http://example.com/style.css", false}, // Should not use proxy
	}

	for _, tt := range tests {
		client := crawlerOptions.ProxyConfig.GetClient(tt.url, "example.com")
		usingProxy := client == crawlerOptions.ProxyConfig.ProxyClient
		
		if usingProxy != tt.expected {
			t.Errorf("URL %s: expected proxy=%v, got proxy=%v", tt.url, tt.expected, usingProxy)
		}
	}
}