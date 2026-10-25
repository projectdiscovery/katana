package headless_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/require"
)

func TestInteractiveCrawlFourLevelsWithScopedHeaders(t *testing.T) {
	skipIfNoBrowser(t)
	var thirdPartyAuth atomic.Bool
	thirdParty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			thirdPartyAuth.Store(true)
		}
		w.Header().Set("Content-Type", "application/javascript")
		_, _ = fmt.Fprint(w, `window.thirdParty = true;`)
	}))
	defer thirdParty.Close()

	var mu sync.Mutex
	served := map[string]bool{}
	authed := map[string]bool{}
	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		served[r.URL.Path] = true
		if r.Header.Get("Authorization") == "Bearer secret" {
			authed[r.URL.Path] = true
		}
		mu.Unlock()
		if r.URL.Path == "/" {
			if cookie, err := r.Cookie("session"); err != nil || cookie.Value != "ok" {
				http.Error(w, "cookie missing", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<html><body><button id="next" onclick="advance()">next 0</button><button id="sibling" onclick="showSibling()">sibling</button><button id="popup" onclick="window.open('/popup')">popup</button><script src="%s/lib.js"></script><script type="module" src="/main.js"></script></body></html>`, thirdParty.URL)
			return
		}
		if r.URL.Path == "/popup" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><button id="popup-next" onclick="openNested()">open nested</button><script type="module" src="/popup.js"></script></body></html>`)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		switch r.URL.Path {
		case "/main.js":
			_, _ = fmt.Fprint(w, `window.advance = async () => { const d = Number(document.body.dataset.depth || 0) + 1; await import('/chunk' + d + '.js'); document.body.dataset.depth = d; document.querySelector('#next').textContent = 'next ' + d; document.querySelector('#sibling')?.remove(); document.querySelector('#popup')?.remove(); }; window.showSibling = async () => { await import('/sibling.js'); document.body.dataset.sibling = 'opened'; };`)
		case "/popup.js":
			_, _ = fmt.Fprint(w, `window.openNested = async () => { await import('/popup-chunk.js'); document.querySelector('#popup-next').textContent = 'opened'; };`)
		default:
			if strings.HasSuffix(r.URL.Path, ".js") {
				_, _ = fmt.Fprint(w, `export const chunk = true;`)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer lab.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	c, err := crawler.New(crawler.Options{
		Context: ctx, MaxBrowsers: 1, MaxDepth: 1, PageMaxTimeout: 15 * time.Second,
		RootURL:       lab.URL,
		CustomHeaders: map[string]string{"Cookie": "session=ok", "Authorization": "Bearer secret"},
		Interactive:   true, InteractionDepth: 4,
		InteractionSelector: "button", InteractionMaxStates: 20,
		InteractionMaxActions: 20, InteractionSettle: time.Second,
		ScopeValidator: func(u string) bool { return strings.HasPrefix(u, lab.URL) },
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Crawl(lab.URL))

	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/main.js", "/chunk1.js", "/chunk2.js", "/chunk3.js", "/chunk4.js", "/sibling.js", "/popup.js", "/popup-chunk.js"} {
		require.True(t, served[path], "interactive crawl did not load %s", path)
		// A popup's first fetches can leave before its tab is attached; the
		// re-navigation after adoption must carry the header.
		require.True(t, authed[path], "custom header missing on %s", path)
	}
	require.False(t, thirdPartyAuth.Load(), "custom header leaked to out-of-scope host")
}

func TestInteractiveDurationContinuesQueuedCrawl(t *testing.T) {
	skipIfNoBrowser(t)
	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<html><body>queued crawl</body></html>`)
	}))
	defer lab.Close()
	var observed atomic.Bool
	c, err := crawler.New(crawler.Options{
		Context: context.Background(), MaxBrowsers: 1, MaxDepth: 1,
		MaxCrawlDuration: 20 * time.Second, PageMaxTimeout: 5 * time.Second,
		RootURL: lab.URL, Interactive: true, InteractionDepth: 4,
		InteractionSelector: "button", InteractionDuration: time.Millisecond,
		RequestCallback: func(result *output.Result) {
			if result != nil && result.Request != nil && result.Request.URL == lab.URL+"/" {
				observed.Store(true)
			}
		},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Crawl(lab.URL+"/"))
	require.True(t, observed.Load(), "ordinary crawl did not run after UI time limit")
}
