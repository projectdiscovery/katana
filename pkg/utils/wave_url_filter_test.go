package utils

import (
	"strings"
	"testing"
)

func TestWaveKatanaURLScopeFilter(t *testing.T) {
	isInScope := func(targetURL, rootDomain string) bool {
		return strings.HasSuffix(targetURL, rootDomain) || strings.Contains(targetURL, "."+rootDomain)
	}

	root := "example.com"
	if !isInScope("https://sub.example.com/api", root) {
		t.Errorf("expected sub.example.com to be in scope for %s", root)
	}
	if isInScope("https://malicious-example.org/test", root) {
		t.Errorf("expected unrelated domain to be out of scope")
	}
}

func TestWaveKatanaFileExtensionExclusion(t *testing.T) {
	ignoredExtensions := map[string]bool{".png": true, ".jpg": true, ".svg": true, ".css": true}
	isCrawlable := func(path string) bool {
		for ext := range ignoredExtensions {
			if strings.HasSuffix(strings.ToLower(path), ext) {
				return false
			}
		}
		return true
	}

	if isCrawlable("https://app.example.com/static/logo.png") {
		t.Errorf("expected .png asset to be skipped by crawler")
	}
	if !isCrawlable("https://app.example.com/api/v1/auth") {
		t.Errorf("expected API endpoint to be crawlable")
	}
}
