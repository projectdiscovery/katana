package crawler

import (
	"testing"

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
