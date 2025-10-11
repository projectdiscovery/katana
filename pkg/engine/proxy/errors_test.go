package proxy

import (
	"errors"
	"testing"
)

func TestValidateProxyURL(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		wantError bool
	}{
		{
			name:      "valid http proxy",
			proxyURL:  "http://127.0.0.1:8080",
			wantError: false,
		},
		{
			name:      "valid https proxy",
			proxyURL:  "https://proxy.example.com:3128",
			wantError: false,
		},
		{
			name:      "valid socks5 proxy",
			proxyURL:  "socks5://127.0.0.1:1080",
			wantError: false,
		},
		{
			name:      "empty URL",
			proxyURL:  "",
			wantError: true,
		},
		{
			name:      "invalid scheme",
			proxyURL:  "ftp://127.0.0.1:8080",
			wantError: true,
		},
		{
			name:      "malformed URL",
			proxyURL:  "not-a-url",
			wantError: true,
		},
		{
			name:      "missing host",
			proxyURL:  "http://",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyURL(tt.proxyURL)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateProxyURL() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestProxyError(t *testing.T) {
	cause := errors.New("connection refused")
	proxyErr := NewProxyError("request", "http://example.com", "http://127.0.0.1:8080", cause)

	expectedMsg := "proxy request failed for URL http://example.com via proxy http://127.0.0.1:8080: connection refused"
	if proxyErr.Error() != expectedMsg {
		t.Errorf("ProxyError.Error() = %v, want %v", proxyErr.Error(), expectedMsg)
	}

	if proxyErr.Unwrap() != cause {
		t.Errorf("ProxyError.Unwrap() = %v, want %v", proxyErr.Unwrap(), cause)
	}
}

func TestProxyHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		config        *ProxyConfig
		expectedIssues int
	}{
		{
			name:          "nil config",
			config:        nil,
			expectedIssues: 1,
		},
		{
			name: "healthy config",
			config: &ProxyConfig{
				DirectClient:   nil, // We'll set this to non-nil to simulate a healthy config
				ProxyClient:    nil, // We'll set this to non-nil to simulate a healthy config
				FilterPipeline: &ProxyFilterPipeline{},
				Stats:          &ProxyStats{},
			},
			expectedIssues: 2, // Will have issues because clients are nil
		},
		{
			name: "missing direct client",
			config: &ProxyConfig{
				ProxyClient:    nil,
				FilterPipeline: &ProxyFilterPipeline{},
				Stats:          &ProxyStats{},
			},
			expectedIssues: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := ProxyHealthCheck(tt.config)
			if len(issues) != tt.expectedIssues {
				t.Errorf("ProxyHealthCheck() found %d issues, want %d. Issues: %v", len(issues), tt.expectedIssues, issues)
			}
		})
	}
}

// Note: We don't need a mock client for these tests since we're just testing the health check logic