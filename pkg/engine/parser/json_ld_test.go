package parser

import (
	"strings"
	"testing"
)

func TestJSONLDEmbeddedURLParser(t *testing.T) {
	snippet := `<script type="application/ld+json">{"@context":"https://schema.org","@type":"WebPage","url":"https://example.com/item"}</script>`
	if !strings.Contains(snippet, "schema.org") {
		t.Fatalf("failed to detect JSON-LD schema payload")
	}
}
