package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
)

func TestClearSessionNoop(t *testing.T) {
	t.Parallel()
	require.NoError(t, ClearSession(context.Background(), nil, []string{"https://example.com"}))
	require.NoError(t, ClearSession(context.Background(), &rod.Page{}, nil))
}

// TestClearSessionScopedToOrigins checks that only the crawled origin loses
// its cookies and storage. The browser can be the user's own, and a cookie of
// another site must survive.
func TestClearSessionScopedToOrigins(t *testing.T) {
	path, found := launcher.LookPath()
	if !found || path == "" {
		t.Skip("chrome/chromium not found")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body>ok</body></html>`))
	}))
	defer srv.Close()

	controlURL, err := launcher.New().Bin(path).Leakless(true).Launch()
	if err != nil {
		t.Skipf("could not launch browser: %v", err)
	}
	b := rod.New().ControlURL(controlURL).MustConnect()
	t.Cleanup(func() { _ = b.Close() })

	page := b.MustPage(srv.URL)
	t.Cleanup(func() { _ = page.Close() })
	page.MustEval(`() => { localStorage.setItem("k", "v"); sessionStorage.setItem("k", "v"); }`)
	require.NoError(t, page.SetCookies([]*proto.NetworkCookieParam{
		{Name: "crawl", Value: "1", URL: srv.URL},
		{Name: "other", Value: "1", URL: "https://other.example/"},
	}))

	require.NoError(t, ClearSession(context.Background(), page, []string{srv.URL}))

	cookies, err := proto.NetworkGetAllCookies{}.Call(page)
	require.NoError(t, err)
	var names []string
	for _, c := range cookies.Cookies {
		names = append(names, c.Name)
	}
	require.NotContains(t, names, "crawl")
	require.Contains(t, names, "other")
	require.Equal(t, "", page.MustEval(`() => (localStorage.getItem("k") || "") + (sessionStorage.getItem("k") || "")`).Str())
}
