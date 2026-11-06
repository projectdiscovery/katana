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
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/stretchr/testify/require"
)

// hoverDropdownPage hides its menu links until the visitor hovers the menu,
// so the crawler cannot click them. The base href makes "guide" resolve to
// /docs/guide, as the browser would.
const hoverDropdownPage = `<!doctype html><html><head><title>home</title><base href="/docs/"><style>
.menu{display:none}.dropdown:hover .menu{display:block}
</style></head><body>
<a href="/top">Top</a>
<a href="/nopointer" style="pointer-events:none">No pointer</a>
<div class="dropdown"><span>More</span><ul class="menu">
<li><a href="/hidden/alpha">Alpha</a></li>
<li><a href="/hidden/beta">Beta</a></li>
<li><a href="guide">Guide</a></li>
<li><a href="%s/outside">Outside</a></li>
</ul></div></body></html>`

func TestE2E_Crawl_FollowsHiddenDropdownLinks(t *testing.T) {
	skipIfNoBrowser(t)

	var mu sync.Mutex
	hits := map[string]int{}
	record := func(r *http.Request) {
		mu.Lock()
		hits[r.Host+r.URL.Path]++
		mu.Unlock()
	}
	outside := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { record(r) }))
	t.Cleanup(outside.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record(r)
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" {
			_, _ = fmt.Fprintf(w, hoverDropdownPage, outside.URL)
			return
		}
		_, _ = fmt.Fprintf(w, "<html><head><title>%s</title></head><body>%s</body></html>", r.URL.Path, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	// The pages are static. Waiting on the load event keeps each hidden-link
	// navigation short. The SPA heuristic was using the whole crawl budget on
	// Windows before the last link, the relative one, was requested.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	c, err := crawler.New(crawler.Options{
		Context:          ctx,
		MaxBrowsers:      1,
		MaxDepth:         3,
		MaxCrawlDuration: 3 * time.Minute,
		PageMaxTimeout:   15 * time.Second,
		PageLoadStrategy: "load",
		NoSandbox:        true,
		Logger:           slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		ScopeValidator:   func(u string) bool { return strings.HasPrefix(u, server.URL) },
	})
	require.NoError(t, err)
	t.Cleanup(c.Close)

	require.NoError(t, c.Crawl(server.URL+"/"))

	host := strings.TrimPrefix(server.URL, "http://")
	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/hidden/alpha", "/hidden/beta", "/docs/guide", "/nopointer"} {
		require.Positive(t, hits[host+path], "hidden link %s was never followed, hits=%v", path, hits)
	}
	require.Zero(t, hits[strings.TrimPrefix(outside.URL, "http://")+"/outside"], "out-of-scope hidden link was requested")
}
