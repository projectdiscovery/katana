package navigation

import (
	"strings"
	"testing"
)

func TestWave6ResourceExtensionFiltering(t *testing.T) {
	shouldSkipAsset := func(path string) bool {
		lower := strings.ToLower(strings.TrimSpace(path))
		skipExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".mp4"}
		for _, ext := range skipExtensions {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
		return false
	}

	if !shouldSkipAsset("/static/images/logo.PNG") {
		t.Error("expected true for image asset")
	}
	if !shouldSkipAsset("/assets/fonts/inter.woff2") {
		t.Error("expected true for font asset")
	}
	if shouldSkipAsset("/api/v1/user/profile.json") {
		t.Error("expected false for JSON API endpoint")
	}
	if shouldSkipAsset("/dashboard/index.html") {
		t.Error("expected false for HTML page")
	}
}

func TestWave6MaxCrawledURLLimit(t *testing.T) {
	isWithinBudget := func(crawledCount int, maxBudget int) bool {
		return crawledCount < maxBudget
	}

	if !isWithinBudget(99, 100) {
		t.Error("count 99 should be within budget 100")
	}
	if isWithinBudget(100, 100) {
		t.Error("count 100 should meet the budget limit")
	}
}
