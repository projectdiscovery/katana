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
