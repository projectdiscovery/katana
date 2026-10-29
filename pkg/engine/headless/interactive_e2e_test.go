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
		Context: ctx, MaxBrowsers: 1, MaxDepth: 1, PageMaxTimeout: 15 * time.Second, NoSandbox: true,
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
		Context: context.Background(), MaxBrowsers: 1, MaxDepth: 1, NoSandbox: true,
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

func TestInteractiveAssetExpansionEnqueuesPagesScriptsAndPrefixes(t *testing.T) {
	skipIfNoBrowser(t)

	var mu sync.Mutex
	served := map[string]bool{}
	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		served[r.URL.Path] = true
		mu.Unlock()
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body>
				<a id="go-admin" href="/admin/users">admin users</a>
				<button id="noop">noop</button>
				<script src="/boot.js"></script>
			</body></html>`)
		case "/admin/users":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body>
				<h1>users</h1>
				<script type="module" src="/admin/users.chunk.js"></script>
				<link rel="modulepreload" href="/admin/preload.js">
			</body></html>`)
		case "/admin", "/admin/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><h1>admin</h1></body></html>`)
		case "/boot.js", "/admin/users.chunk.js", "/admin/preload.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, `export const ok = true;`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer lab.Close()

	var seenMu sync.Mutex
	seen := map[string]bool{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c, err := crawler.New(crawler.Options{
		Context: ctx, MaxBrowsers: 1, MaxDepth: 3, PageMaxTimeout: 10 * time.Second, NoSandbox: true,
		RootURL: lab.URL, Interactive: true, InteractionDepth: 2,
		InteractionSelector: "a, button", InteractionMaxStates: 30,
		InteractionMaxActions: 30, InteractionSettle: time.Second,
		ScopeValidator: func(u string) bool { return strings.HasPrefix(u, lab.URL) },
		RequestCallback: func(result *output.Result) {
			if result == nil || result.Request == nil {
				return
			}
			seenMu.Lock()
			seen[result.Request.URL] = true
			seenMu.Unlock()
		},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Crawl(lab.URL+"/"))

	mu.Lock()
	defer mu.Unlock()
	require.True(t, served["/admin/users"], "click did not open /admin/users")
	require.True(t, served["/admin/users.chunk.js"] || served["/admin/preload.js"] || served["/boot.js"], "expected interactive JS assets to load")
	// Parent prefix discovered via interaction should be visited by ordinary crawl.
	require.True(t, served["/admin"] || served["/admin/"], "parent path /admin was not crawled")

	seenMu.Lock()
	defer seenMu.Unlock()
	require.True(t, seen[lab.URL+"/admin/users"] || seen[lab.URL+"/admin/users/"], "admin users URL missing from output")
}

func TestInteractiveRestoreFailureDoesNotHang(t *testing.T) {
	skipIfNoBrowser(t)

	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body>
				<button id="leave" onclick="location.href='/gone'">leave</button>
				<button id="sibling">sibling</button>
			</body></html>`)
		case "/gone":
			w.Header().Set("Content-Type", "text/html")
			// Replace history so back cannot restore the parent document cheaply
			// in some browsers; explorer should abandon siblings instead of hanging.
			_, _ = fmt.Fprint(w, `<html><body><script>history.replaceState(null,'','/gone');</script><p>gone</p></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer lab.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	start := time.Now()
	c, err := crawler.New(crawler.Options{
		Context: ctx, MaxBrowsers: 1, MaxDepth: 2, PageMaxTimeout: 5 * time.Second, NoSandbox: true,
		RootURL: lab.URL, Interactive: true, InteractionDepth: 2,
		InteractionSelector: "button", InteractionMaxStates: 10,
		InteractionMaxActions: 10, InteractionSettle: 500 * time.Millisecond,
		ScopeValidator: func(u string) bool { return strings.HasPrefix(u, lab.URL) },
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Crawl(lab.URL+"/"))
	require.Less(t, time.Since(start), 40*time.Second, "interactive crawl hung after restore difficulty")
}

func TestInteractiveSameURLSiblingRestoresClickPath(t *testing.T) {
	skipIfNoBrowser(t)

	var mu sync.Mutex
	served := map[string]bool{}
	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		served[r.URL.Path] = true
		mu.Unlock()
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			// Root exposes a menu. Opening it reveals two same-URL siblings.
			// Clicking A removes B, so restoring the menu panel (via click-path
			// replay) is required before B can load branch-b.js.
			_, _ = fmt.Fprint(w, `<html><body>
				<button id="menu" onclick="openMenu()">menu</button>
				<script src="/main.js"></script>
			</body></html>`)
		case "/main.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, `
window.openMenu = () => {
  document.body.innerHTML = '<div id="panel"><button id="a" onclick="openA()">A</button><button id="b" onclick="openB()">B</button></div>';
};
window.openA = async () => {
  await import('/branch-a.js');
  document.getElementById('panel').innerHTML = '<p id="opened-a">opened A</p>';
};
window.openB = async () => {
  await import('/branch-b.js');
  document.getElementById('panel').innerHTML = '<p id="opened-b">opened B</p>';
};
`)
		case "/branch-a.js", "/branch-b.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = fmt.Fprint(w, `export const ok = true;`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer lab.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	c, err := crawler.New(crawler.Options{
		Context: ctx, MaxBrowsers: 1, MaxDepth: 2, PageMaxTimeout: 10 * time.Second, NoSandbox: true,
		RootURL: lab.URL, Interactive: true, InteractionDepth: 3,
		InteractionSelector: "button", InteractionMaxStates: 30,
		InteractionMaxActions: 40, InteractionSettle: time.Second,
		ScopeValidator: func(u string) bool { return strings.HasPrefix(u, lab.URL) },
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Crawl(lab.URL+"/"))

	mu.Lock()
	defer mu.Unlock()
	require.True(t, served["/branch-a.js"], "first same-URL branch did not load /branch-a.js")
	require.True(t, served["/branch-b.js"], "sibling same-URL branch did not load /branch-b.js after path restore")
}
