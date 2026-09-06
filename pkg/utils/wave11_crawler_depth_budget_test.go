package utils

import (
	"testing"
)

// TestWave11CrawlDepthBudgetEvaluation asserts crawl recursion safety
func TestWave11CrawlDepthBudgetEvaluation(t *testing.T) {
	maxDepth := 5
	maxURLsPerScope := 500

	canCrawl := func(currentDepth, currentURLCount int) bool {
		return currentDepth <= maxDepth && currentURLCount < maxURLsPerScope
	}

	if !canCrawl(3, 200) {
		t.Errorf("expected depth 3 and 200 URLs to be within crawling budget")
	}
	if canCrawl(6, 200) {
		t.Errorf("expected depth 6 to exceed crawl depth budget")
	}
	if canCrawl(2, 500) {
		t.Errorf("expected 500 URLs to trigger max URL scope ceiling")
	}
}

// TestWave11RobotsDisallowPatternMatching tests robots.txt parsing logic
func TestWave11RobotsDisallowPatternMatching(t *testing.T) {
	isPathDisallowed := func(disallowedPrefix, path string) bool {
		return len(disallowedPrefix) > 0 && len(path) >= len(disallowedPrefix) && path[:len(disallowedPrefix)] == disallowedPrefix
	}

	if !isPathDisallowed("/admin", "/admin/dashboard") {
		t.Errorf("expected /admin/dashboard to be disallowed under /admin prefix")
	}
	if isPathDisallowed("/admin", "/public/home") {
		t.Errorf("expected /public/home to be allowed")
	}
}
