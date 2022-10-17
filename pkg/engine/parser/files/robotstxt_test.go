package files

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/stretchr/testify/require"
)

func TestRobotsTxtParseReader(t *testing.T) {
	requests := []string{}
	crawler := &robotsTxtCrawler{}

	content := `User-agent: *
Disallow: /test/misc/known-files/robots.txt.found

User-agent: *
Disallow: /test/includes/

# User-agent: Googlebot
# Allow: /random/

Sitemap: https://example.com/sitemap.xml`
	parsed, _ := url.Parse("http://localhost/robots.txt")
	crawler.parseReader(strings.NewReader(content), &http.Response{Request: &http.Request{URL: parsed}}, func(r navigation.Request) {
		requests = append(requests, r.URL)
	})
	require.ElementsMatch(t, requests, []string{
		"http://localhost/test/includes/",
		"http://localhost/test/misc/known-files/robots.txt.found",
	}, "could not get correct elements")
}
