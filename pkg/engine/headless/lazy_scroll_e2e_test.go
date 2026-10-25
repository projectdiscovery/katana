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

// lazyLinksPage renders a feed whose links appear only after the visitor
// scrolls to the bottom, loadDelayMs after the scroll. Scroll restoration is
// manual, as in many SPAs, so going back does not reload the feed by itself.
const lazyLinksPage = `<!doctype html><html><head><title>feed</title></head><body>
<div style="height:4000px">feed</div>
<div id="more"></div>
<script>
history.scrollRestoration = 'manual';
let requested = false;
window.addEventListener('scroll', () => {
  if (requested) return;
  if (window.scrollY + window.innerHeight < document.body.scrollHeight - 200) return;
  requested = true;
  setTimeout(() => {
    const more = document.getElementById('more');
    for (let n = 1; n <= 20; n++) {
      const item = document.createElement('article');
      item.innerHTML = '<h2>Story ' + n + '</h2><p>Summary text for story number ' + n + '</p>';
      if (n <= 3) {
        const a = document.createElement('a');
        a.id = 'lazy-' + n;
        a.href = '/lazy/' + n;
        a.textContent = 'lazy ' + n;
        item.appendChild(a);
      }
      more.appendChild(item);
    }
  }, %d);
});
</script></body></html>`

func TestE2E_Crawl_FollowsLazyLoadedLinks(t *testing.T) {
	skipIfNoBrowser(t)

	tests := []struct {
		name        string
		loadDelayMs int
	}{
		{name: "immediate", loadDelayMs: 0},
		// Slower than one 300ms settle period, within the stability window.
		{name: "delayed", loadDelayMs: 500},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			hits := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				hits[r.URL.Path]++
				mu.Unlock()
				w.Header().Set("Content-Type", "text/html")
				if r.URL.Path == "/" {
					// no-store keeps the feed out of the back/forward cache, so a
					// history navigation reloads it without the scrolled-in links.
					w.Header().Set("Cache-Control", "no-store")
					_, _ = fmt.Fprintf(w, lazyLinksPage, test.loadDelayMs)
					return
				}
				_, _ = fmt.Fprintf(w, "<html><head><title>%s</title></head><body>%s</body></html>", r.URL.Path, r.URL.Path)
			}))
			t.Cleanup(server.Close)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			c, err := crawler.New(crawler.Options{
				Context:          ctx,
				MaxBrowsers:      1,
				MaxDepth:         3,
				MaxCrawlDuration: 60 * time.Second,
				PageMaxTimeout:   15 * time.Second,
				NoSandbox:        true,
				MaxScrollSteps:   10,
				Logger:           slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
				ScopeValidator:   func(u string) bool { return strings.HasPrefix(u, server.URL) },
			})
			require.NoError(t, err)
			t.Cleanup(c.Close)

			require.NoError(t, c.Crawl(server.URL+"/"))

			mu.Lock()
			defer mu.Unlock()
			for _, path := range []string{"/lazy/1", "/lazy/2", "/lazy/3"} {
				require.Positive(t, hits[path], "lazy link %s was never followed, hits=%v", path, hits)
			}
		})
	}
}
