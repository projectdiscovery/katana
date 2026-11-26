package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestCrawlerOptionsPropagation(t *testing.T) {
	c, err := New(Options{
		UseInstalledChrome: true,
		ChromiumPath:       "/mock/chrome",
		MaxBrowsers:        1,
	})
	require.NoError(t, err)
	require.True(t, c.options.UseInstalledChrome)
	require.Equal(t, "/mock/chrome", c.options.ChromiumPath)
}

func TestResolveHTTPLink(t *testing.T) {
	const page = "http://example.com/a/index.html"
	tests := []struct {
		href string
		want string
		ok   bool
	}{
		{href: "/hidden/alpha", want: "http://example.com/hidden/alpha", ok: true},
		{href: "beta", want: "http://example.com/a/beta", ok: true},
		{href: "https://other.com/x", want: "https://other.com/x", ok: true},
		{href: "javascript:void(0)"},
		{href: "mailto:a@example.com"},
		{href: ""},
	}
	for _, tt := range tests {
		got, ok := resolveHTTPLink(tt.href, page)
		require.Equal(t, tt.ok, ok, tt.href)
		require.Equal(t, tt.want, got, tt.href)
	}
}

func TestSimilarLinkFollowed(t *testing.T) {
	const page = "http://example.com/"
	link := func(href string) *types.Action {
		return &types.Action{Element: &types.HTMLElement{TagName: "A", Attributes: map[string]string{"href": href}}}
	}
	trie := utils.NewPathTrie(0)
	c := &Crawler{
		options:      Options{URLFingerprint: func(u string) string { return utils.FingerprintURL(u, trie) }},
		similarLinks: map[string]struct{}{},
	}
	require.False(t, c.similarLinkFollowed(link("/p/1"), page))
	require.True(t, c.similarLinkFollowed(link("/p/2"), page), "same pattern as /p/1")
	require.False(t, c.similarLinkFollowed(link("/about"), page))
	require.False(t, c.similarLinkFollowed(&types.Action{Type: types.ActionTypeLoadURL, Input: "http://example.com/p/3"}, page), "only links are filtered")

	off := &Crawler{similarLinks: map[string]struct{}{}}
	require.False(t, off.similarLinkFollowed(link("/p/1"), page))
	require.False(t, off.similarLinkFollowed(link("/p/2"), page), "-fsu off follows every link")
}

func TestMarkSimilarURL_Seed(t *testing.T) {
	trie := utils.NewPathTrie(0)
	c := &Crawler{
		options:      Options{URLFingerprint: func(u string) string { return utils.FingerprintURL(u, trie) }},
		similarLinks: map[string]struct{}{},
	}
	require.True(t, c.markSimilarURL("http://example.com/p/1"), "seed is new")
	link := &types.Action{Element: &types.HTMLElement{TagName: "A", Attributes: map[string]string{"href": "/p/2"}}}
	require.True(t, c.similarLinkFollowed(link, "http://example.com/"), "link similar to the seed is skipped")
}

func TestLinkBase_BaseHref(t *testing.T) {
	const pageURL = "http://example.com/"
	require.Equal(t, pageURL, baseOrPage("", pageURL))
	base := baseOrPage("http://example.com/docs/", pageURL)
	require.Equal(t, "http://example.com/docs/", base)

	trie := utils.NewPathTrie(0)
	c := &Crawler{
		options:      Options{URLFingerprint: func(u string) string { return utils.FingerprintURL(u, trie) }},
		similarLinks: map[string]struct{}{},
	}
	link := func(href string) *types.Action {
		return &types.Action{Element: &types.HTMLElement{TagName: "A", Attributes: map[string]string{"href": href}}}
	}
	// "guide" resolves to /docs/guide under the base, so it is not similar to /guide.
	require.False(t, c.similarLinkFollowed(link("/guide"), pageURL))
	require.False(t, c.similarLinkFollowed(link("guide"), base))
	require.True(t, c.similarLinkFollowed(link("guide"), base), "same resolved URL")
}

func TestLinkBase_FilterOffSkipsEval(t *testing.T) {
	c := &Crawler{}
	// A nil page would panic if linkBase evaluated in the browser.
	require.Equal(t, "http://example.com/a", c.linkBase(nil, "http://example.com/a"))
}
