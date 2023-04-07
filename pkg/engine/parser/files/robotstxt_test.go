package files

import (
	"net/http"

	"strings"
	"testing"

	urlutil "github.com/projectdiscovery/utils/url"
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
	parsed, err := urlutil.Parse("http://localhost/robots.txt")
	require.Nil(t, err)
	navigationRequests, err := crawler.parseReader(strings.NewReader(content), &http.Response{Request: &http.Request{URL: parsed.URL}})
	require.Nil(t, err)

	for _, navReq := range navigationRequests {
		requests = append(requests, navReq.URL)
	}
	require.ElementsMatch(t, requests, []string{
		"http://localhost/test/includes/",
		"http://localhost/test/misc/known-files/robots.txt.found",
	}, "could not get correct elements")
}
