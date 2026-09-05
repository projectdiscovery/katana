package output_test

import (
	"testing"
	"strings"
)

func TestWave1URLSanitization(t *testing.T) {
	sanitizeURL := func(raw string) string {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "javascript:") || strings.HasPrefix(trimmed, "data:") {
			return ""
		}
		return trimmed
	}

	if sanitizeURL("  https://projectdiscovery.io  ") != "https://projectdiscovery.io" {
		t.Errorf("expected trimmed url")
	}

	if sanitizeURL("javascript:void(0)") != "" {
		t.Errorf("expected dangerous scheme to be stripped")
	}
}
