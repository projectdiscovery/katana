package files

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
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
	parsed, _ := url.Parse("http://security-crawl-maze.app/sitemap.xml")
	_ = crawler.parseReader(strings.NewReader(content), &http.Response{Request: &http.Request{URL: parsed}}, func(r navigation.Request) {
		requests = append(requests, r.URL)
	})
	require.ElementsMatch(t, requests, []string{
		"http://security-crawl-maze.app/test/misc/known-files/sitemap.xml.found",
	}, "could not get correct elements")
}
