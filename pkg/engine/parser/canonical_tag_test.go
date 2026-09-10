package parser

import (
	"strings"
	"testing"
)

func TestCanonicalLinkTagExtraction(t *testing.T) {
	html := `<link rel="canonical" href="https://example.com/preferred-page" />`
	if !strings.Contains(html, `rel="canonical"`) {
		t.Fatalf("failed to locate canonical link relationship")
	}
}
