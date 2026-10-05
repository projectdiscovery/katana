package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
)

func TestPutBrowserToPoolWithChromeWSUrl(t *testing.T) {
	skipIfNoBrowser(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, "<html><body>%s</body></html>", r.URL.Path)
	}))
	defer server.Close()

	tests := []struct {
		name        string
		features    string
		popupClosed bool
	}{
		{name: "popup", popupClosed: true},
		// noopener popups report no opener, so they cannot be told apart from
		// the user's tabs; only katana's page coming back to front is checked.
		{name: "noopener popup", features: "noopener"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chrome := launcher.New().Headless(true).NoSandbox(true)
			wsURL, err := chrome.Launch()
			require.NoError(t, err)
			defer chrome.Kill()

			user := rod.New().ControlURL(wsURL)
			require.NoError(t, user.Connect())
			defer func() { _ = user.Close() }()
			_, err = user.Page(proto.TargetCreateTarget{URL: server.URL + "/user"})
			require.NoError(t, err)

			l, err := NewLauncher(LauncherOptions{MaxBrowsers: 1, NoSandbox: true, ChromeWSUrl: wsURL})
			require.NoError(t, err)
			defer l.Close()

			page, err := l.GetPageFromPool()
			require.NoError(t, err)
			require.NoError(t, page.Navigate(server.URL+"/crawl"))
			require.NoError(t, page.WaitLoad())
			_, err = page.Eval(`(url, features) => { window.open(url, "_blank", features) }`, server.URL+"/popup", tt.features)
			require.NoError(t, err)
			require.Eventually(t, func() bool { return hasPage(t, user, "/popup") }, 5*time.Second, 100*time.Millisecond)

			l.PutBrowserToPool(page)

			require.True(t, hasPage(t, user, "/user"), "user tab must survive")
			require.True(t, hasPage(t, user, "/crawl"), "katana's pooled page must survive")
			if tt.popupClosed {
				require.False(t, hasPage(t, user, "/popup"), "popup opened by katana must be closed")
			}
			// visibilityState updates asynchronously after BringToFront returns.
			require.Eventually(t, func() bool {
				visibility, err := page.Eval(`() => document.visibilityState`)
				return err == nil && visibility.Value.Str() == "visible"
			}, 2*time.Second, 50*time.Millisecond, "a backgrounded page stalls repaint waits")
		})
	}
}

func hasPage(t *testing.T, b *rod.Browser, path string) bool {
	t.Helper()
	targets, err := proto.TargetGetTargets{}.Call(b)
	require.NoError(t, err)
	for _, target := range targets.TargetInfos {
		if target.Type == proto.TargetTargetInfoTypePage && strings.HasSuffix(target.URL, path) {
			return true
		}
	}
	return false
}
