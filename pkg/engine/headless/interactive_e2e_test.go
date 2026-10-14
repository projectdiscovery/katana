package headless_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/assets"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/require"
)

func TestInteractiveCrawlFourLevelsCapturesChunksAndCookie(t *testing.T) {
	skipIfNoBrowser(t)
	largeScript := `export const payload = "` + strings.Repeat("x", 2*1024*1024) + `";`
	compressedScript := `export const compressed = "` + strings.Repeat("z", 100000) + `";`
	lab := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			if cookie, err := r.Cookie("session"); err != nil || cookie.Value != "ok" {
				http.Error(w, "cookie missing", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><button id="next" onclick="advance()">next 0</button><button id="sibling" onclick="showSibling()">sibling</button><button id="popup" onclick="window.open('/popup')">popup</button><script type="module" src="/main.js"></script></body></html>`)
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
			_, _ = fmt.Fprint(w, `import './nested.js'; import './compressed.js'; window.unusedWorker = () => new Worker('./unrun.js'); window.advance = async () => { const d = Number(document.body.dataset.depth || 0) + 1; await import('/chunk' + d + '.js'); document.body.dataset.depth = d; document.querySelector('#next').textContent = 'next ' + d; document.querySelector('#sibling')?.remove(); document.querySelector('#popup')?.remove(); }; window.showSibling = async () => { await import('/sibling.js'); document.body.dataset.sibling = 'opened'; };`)
		case "/nested.js":
			_, _ = fmt.Fprint(w, `export const nested = true;`)
		case "/compressed.js":
			w.Header().Set("Content-Encoding", "gzip")
			zip := gzip.NewWriter(w)
			_, _ = zip.Write([]byte(compressedScript))
			_ = zip.Close()
		case "/sibling.js":
			_, _ = fmt.Fprint(w, `export const sibling = true;`)
		case "/popup.js":
			_, _ = fmt.Fprint(w, `window.openNested = async () => { await import('/popup-chunk.js'); document.querySelector('#popup-next').textContent = 'opened'; };`)
		case "/popup-chunk.js":
			_, _ = fmt.Fprint(w, `export const popupChunk = true;`)
		case "/unrun.js":
			_, _ = fmt.Fprint(w, `importScripts('./unrun-nested.js');`)
		case "/unrun-nested.js":
			_, _ = fmt.Fprint(w, `const unrunNested = true;`)
		case "/chunk4.js":
			_, _ = fmt.Fprint(w, largeScript)
		default:
			if strings.HasPrefix(r.URL.Path, "/chunk") {
				_, _ = fmt.Fprint(w, `export const chunk = true;`)
				return
			}
			http.NotFound(w, r)
		}
	}))
	defer lab.Close()

	dir := t.TempDir()
	store, err := assets.NewWithDiscovery(dir, 2)
	require.NoError(t, err)
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	c, err := crawler.New(crawler.Options{
		Context: ctx, MaxBrowsers: 1, MaxDepth: 1, PageMaxTimeout: 15 * time.Second,
		RootURL: lab.URL, CustomHeaders: map[string]string{"Cookie": "session=ok"},
		Interactive: true, InteractionDepth: 4,
		InteractionSelector: "button", InteractionMaxStates: 20,
		InteractionMaxActions: 20, InteractionSettle: time.Second,
		ScriptCallback: func(r assets.Response) { _, _ = store.Save(r) },
		ScriptStore:    store,
		ScopeValidator: func(u string) bool { return strings.HasPrefix(u, lab.URL) },
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	})
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Crawl(lab.URL))

	manifest, err := os.ReadFile(filepath.Join(dir, "manifest.jsonl"))
	require.NoError(t, err)
	records := make(map[string]assets.Record)
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		var record assets.Record
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		if record.Complete {
			records[record.URL] = record
		}
	}
	for _, path := range []string{"/main.js", "/nested.js", "/compressed.js", "/chunk1.js", "/chunk2.js", "/chunk3.js", "/chunk4.js", "/sibling.js", "/popup.js", "/popup-chunk.js", "/unrun.js", "/unrun-nested.js"} {
		require.Contains(t, records, lab.URL+path, "missing script %s", path)
	}
	large := records[lab.URL+"/chunk4.js"]
	require.Equal(t, len(largeScript), large.Bytes)
	body, err := os.ReadFile(filepath.Join(dir, large.File))
	require.NoError(t, err)
	require.Equal(t, largeScript, string(body))
	compressed := records[lab.URL+"/compressed.js"]
	body, err = os.ReadFile(filepath.Join(dir, compressed.File))
	require.NoError(t, err)
	require.Equal(t, compressedScript, string(body))
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
