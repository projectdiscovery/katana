package browser

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/require"
)

// startSPAServer spins up a test HTTP server whose index page opens an
// SSE stream that never closes, mimicking real-world SPAs with persistent
// connections (analytics, chat, live updates). Network-idle strategies
// can't finish on pages like this because there's always traffic in flight.
func startSPAServer(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>SPA Test</title></head>
<body>
  <a href="/page1">Page 1</a>
  <a href="/page2">Page 2</a>
  <a href="/api/data">API</a>
  <script>
    const evtSource = new EventSource("/stream");
    evtSource.onmessage = function(e) {};
  </script>
</body>
</html>`)
	})

	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(500 * time.Millisecond):
				_, _ = fmt.Fprintf(w, "data: keepalive\n\n")
				flusher.Flush()
			}
		}
	})

	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body><a href="/page3">Page 3</a></body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body>Page 2</body></html>`)
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"status":"ok"}`)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &http.Server{Handler: mux}
	go server.Serve(listener) //nolint:errcheck
	t.Cleanup(func() { server.Close() })

	return fmt.Sprintf("http://%s", listener.Addr().String())
}

func skipIfNoBrowser(t *testing.T) {
	t.Helper()
	path, _ := launcher.LookPath()
	if path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}
}

// TestPageLoadStrategyWithSPA hits a local SPA whose index keeps an SSE
// connection open forever. Each subtest picks a different page-load strategy
// and checks that the crawl finishes in a reasonable time with the right HTML.
func TestPageLoadStrategyWithSPA(t *testing.T) {
	skipIfNoBrowser(t)
	baseURL := startSPAServer(t)

	t.Run("none strategy returns immediately", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:      1,
			PageLoadStrategy: "none",
			NoSandbox:        true,
		})
		require.NoError(t, err)
		defer l.Close()

		bp, err := l.GetPageFromPool()
		require.NoError(t, err)

		start := time.Now()
		err = bp.Navigate(baseURL)
		require.NoError(t, err)
		err = bp.WaitPageLoadHeurisitics()
		require.NoError(t, err)
		elapsed := time.Since(start)

		require.Less(t, elapsed, 10*time.Second, "none strategy should return almost immediately")
		l.PutBrowserToPool(bp)
	})

	t.Run("domcontentloaded strategy completes without hanging on SSE", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:      1,
			PageLoadStrategy: "domcontentloaded",
			DOMWaitTime:      1,
			NoSandbox:        true,
		})
		require.NoError(t, err)
		defer l.Close()

		bp, err := l.GetPageFromPool()
		require.NoError(t, err)

		start := time.Now()
		err = bp.Navigate(baseURL)
		require.NoError(t, err)
		err = bp.WaitPageLoadHeurisitics()
		require.NoError(t, err)
		elapsed := time.Since(start)

		// Should complete in ~1-2s (DOMWaitTime=1), not hang on SSE stream.
		// Relaxed to 15s to accommodate slow CI runners (Windows).
		require.Less(t, elapsed, 15*time.Second, "domcontentloaded should not hang on continuous network activity")

		html, err := bp.HTML()
		require.NoError(t, err)
		require.Contains(t, html, "Page 1", "page content should be loaded")
		require.Contains(t, html, "Page 2", "page content should be loaded")

		l.PutBrowserToPool(bp)
	})

	t.Run("load strategy completes on SPA with SSE", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:      1,
			PageLoadStrategy: "load",
			NoSandbox:        true,
		})
		require.NoError(t, err)
		defer l.Close()

		bp, err := l.GetPageFromPool()
		require.NoError(t, err)

		start := time.Now()
		err = bp.Navigate(baseURL)
		require.NoError(t, err)
		err = bp.WaitPageLoadHeurisitics()
		require.NoError(t, err)
		elapsed := time.Since(start)

		require.Less(t, elapsed, 20*time.Second, "load strategy should complete within timeout")

		html, err := bp.HTML()
		require.NoError(t, err)
		require.Contains(t, html, "Page 1")

		l.PutBrowserToPool(bp)
	})

	t.Run("heuristic strategy completes on SPA with SSE", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:      1,
			PageLoadStrategy: "heuristic",
			NoSandbox:        true,
		})
		require.NoError(t, err)
		defer l.Close()

		bp, err := l.GetPageFromPool()
		require.NoError(t, err)

		start := time.Now()
		err = bp.Navigate(baseURL)
		require.NoError(t, err)
		err = bp.WaitPageLoadHeurisitics()
		require.NoError(t, err)
		elapsed := time.Since(start)

		require.Less(t, elapsed, 20*time.Second, "heuristic should complete within timeout")

		html, err := bp.HTML()
		require.NoError(t, err)
		require.Contains(t, html, "Page 1")

		l.PutBrowserToPool(bp)
	})
}

// TestDOMWaitTimeIsRespected makes sure a larger DOMWaitTime value
// actually makes the domcontentloaded strategy wait longer.
func TestDOMWaitTimeIsRespected(t *testing.T) {
	skipIfNoBrowser(t)
	baseURL := startSPAServer(t)

	t.Run("shorter DOMWaitTime finishes faster", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:      1,
			PageLoadStrategy: "domcontentloaded",
			DOMWaitTime:      1,
			NoSandbox:        true,
		})
		require.NoError(t, err)
		defer l.Close()

		bp, err := l.GetPageFromPool()
		require.NoError(t, err)

		start := time.Now()
		err = bp.Navigate(baseURL)
		require.NoError(t, err)
		err = bp.WaitPageLoadHeurisitics()
		require.NoError(t, err)
		shortElapsed := time.Since(start)

		l.PutBrowserToPool(bp)

		l2, err := NewLauncher(LauncherOptions{
			MaxBrowsers:      1,
			PageLoadStrategy: "domcontentloaded",
			DOMWaitTime:      4,
			NoSandbox:        true,
		})
		require.NoError(t, err)
		defer l2.Close()

		bp2, err := l2.GetPageFromPool()
		require.NoError(t, err)

		start = time.Now()
		err = bp2.Navigate(baseURL)
		require.NoError(t, err)
		err = bp2.WaitPageLoadHeurisitics()
		require.NoError(t, err)
		longElapsed := time.Since(start)

		l2.PutBrowserToPool(bp2)

		require.Greater(t, longElapsed, shortElapsed, "DOMWaitTime=4 should take longer than DOMWaitTime=1")
		require.Greater(t, longElapsed, 3*time.Second, "DOMWaitTime=4 should wait at least ~4 seconds")
	})
}
