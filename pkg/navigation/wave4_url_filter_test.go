package navigation

import (
	"strings"
	"testing"
)

func TestWave4URLFilterSanitization(t *testing.T) {
	shouldCrawl := func(rawURL string, scopeDomain string) bool {
		lower := strings.ToLower(strings.TrimSpace(rawURL))
		if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") {
			return false
		}
		return strings.Contains(lower, scopeDomain)
	}

	if !shouldCrawl("https://app.example.com/dashboard", "example.com") {
		t.Error("expected true for in-scope URL")
	}
	if shouldCrawl("javascript:void(0)", "example.com") {
		t.Error("expected false for javascript pseudo-protocol")
	}
	if shouldCrawl("mailto:security@example.com", "example.com") {
		t.Error("expected false for mailto link")
	}
	if shouldCrawl("https://external-tracker.net/track", "example.com") {
		t.Error("expected false for out-of-scope domain")
	}
}

func TestWave4QueryParamDeduplication(t *testing.T) {
	stripTrackingParams := func(rawQuery string) string {
		params := strings.Split(rawQuery, "&")
		var retained []string
		for _, p := range params {
			if !strings.HasPrefix(p, "utm_") && !strings.HasPrefix(p, "fbclid=") {
				retained = append(retained, p)
			}
		}
		return strings.Join(retained, "&")
	}

	q := "page=2&utm_source=twitter&sort=desc&utm_medium=cpc"
	expected := "page=2&sort=desc"
	if stripTrackingParams(q) != expected {
		t.Errorf("stripTrackingParams(%q) = %q; want %q", q, stripTrackingParams(q), expected)
	}
}
