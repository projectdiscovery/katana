package types

import (
	"testing"

	"github.com/projectdiscovery/goflags"
)

func TestNewCrawlerOptions_WithProxyFiltering(t *testing.T) {
	tests := []struct {
		name    string
		options *Options
		wantErr bool
	}{
		{
			name: "basic options without proxy",
			options: &Options{
				MaxDepth:    2,
				Concurrency: 10,
				Timeout:     10,
			},
			wantErr: false,
		},
		{
			name: "options with proxy and extension filters",
			options: &Options{
				MaxDepth:        2,
				Concurrency:     10,
				Timeout:         10,
				Proxy:           "http://127.0.0.1:8080",
				ProxyFiltering:  true,
				ExtensionsMatch: goflags.StringSlice{"php", "html"},
			},
			wantErr: false,
		},
		{
			name: "options with proxy and scope filters",
			options: &Options{
				MaxDepth:       2,
				Concurrency:    10,
				Timeout:        10,
				Proxy:          "http://127.0.0.1:8080",
				ProxyFiltering: true,
				Scope:          goflags.StringSlice{"example.com"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crawlerOptions, err := NewCrawlerOptions(tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCrawlerOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if err == nil {
				// Verify that proxy components are initialized
				if crawlerOptions.ProxyFilterPipeline == nil {
					t.Error("ProxyFilterPipeline should be initialized")
				}
				if crawlerOptions.ProxyConfig == nil {
					t.Error("ProxyConfig should be initialized")
				}
				
				// Test GetHttpClient method
				client := crawlerOptions.GetHttpClient("http://example.com/test.php", "example.com")
				if client == nil {
					t.Error("GetHttpClient should return a valid client")
				}
				
				// Clean up
				crawlerOptions.Close()
			}
		})
	}
}

func TestCrawlerOptions_GetHttpClient(t *testing.T) {
	options := &Options{
		MaxDepth:        2,
		Concurrency:     10,
		Timeout:         10,
		Proxy:           "http://127.0.0.1:8080",
		ExtensionsMatch: goflags.StringSlice{"php"},
	}
	
	crawlerOptions, err := NewCrawlerOptions(options)
	if err != nil {
		t.Fatalf("Failed to create crawler options: %v", err)
	}
	defer crawlerOptions.Close()
	
	tests := []struct {
		name         string
		url          string
		rootHostname string
	}{
		{
			name:         "matching extension",
			url:          "http://example.com/test.php",
			rootHostname: "example.com",
		},
		{
			name:         "non-matching extension",
			url:          "http://example.com/test.html",
			rootHostname: "example.com",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := crawlerOptions.GetHttpClient(tt.url, tt.rootHostname)
			if client == nil {
				t.Error("GetHttpClient should always return a valid client")
			}
		})
	}
}