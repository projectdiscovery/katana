package files

import (
	"net/http"
	"strings"
	"testing"

	urlutil "github.com/projectdiscovery/utils/url"
	"github.com/stretchr/testify/require"
)

func TestSitemapXmlParseReader(t *testing.T) {
	requests := []string{}
	crawler := &sitemapXmlCrawler{}

	content := `<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
<sitemap>
  	<loc>
		http://security-crawl-maze.app/test/misc/known-files/sitemap.xml.found
	</loc>
	<lastmod>2019-06-19T12:00:00+00:00</lastmod>
</sitemap>
</sitemapindex>`
	parsed, err := urlutil.Parse("http://security-crawl-maze.app/sitemap.xml")
	require.Nil(t, err)
	navigationRequests, err := crawler.parseReader(strings.NewReader(content), &http.Response{Request: &http.Request{URL: parsed.URL}})
	require.Nil(t, err)
	for _, navReq := range navigationRequests {
		requests = append(requests, navReq.URL)
	}
	require.ElementsMatch(t, requests, []string{
		"http://security-crawl-maze.app/test/misc/known-files/sitemap.xml.found",
	}, "could not get correct elements")
}
