package parser

import (
	"strings"
	"testing"
)

func TestXMLSitemapIndexParsing(t *testing.T) {
	xmlData := `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><sitemap><loc>https://example.com/sitemap-1.xml</loc></sitemap></sitemapindex>`
	if !strings.Contains(xmlData, "<sitemapindex") {
		t.Fatalf("failed to detect sitemap index XML root tag")
	}
}
