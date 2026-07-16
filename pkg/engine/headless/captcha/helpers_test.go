package captcha

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

func setupBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	path, found := launcher.LookPath()
	if !found {
		t.Skip("chromium not found, skipping browser tests")
	}
	u := launcher.New().Bin(path).Headless(true).Leakless(true).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	t.Cleanup(func() { browser.MustClose() })
	return browser
}

func servePage(t *testing.T, html string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, html)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
