package navigation

import (
	"net/url"
	"strings"
	"testing"
)

func NormalizeCrawlURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	parsed.Fragment = "" // Strip hash fragments for deduplication
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String(), nil
}

func TestWave16NormalizeCrawlURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HTTPS://Example.COM/page#section", "https://example.com/page"},
		{"http://test.org:8080/api?id=10#top", "http://test.org:8080/api?id=10"},
		{"https://app.io", "https://app.io/"},
	}

	for _, tt := range tests {
		got, err := NormalizeCrawlURL(tt.input)
		if err != nil {
			t.Fatalf("unexpected error parsing %s: %v", tt.input, err)
		}
		if got != tt.expected {
			t.Errorf("NormalizeCrawlURL(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}
