package standard

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

// connTracker counts the server side connections that are still open.
type connTracker struct {
	mu   sync.Mutex
	open map[net.Conn]struct{}
}

func (c *connTracker) track(conn net.Conn, state http.ConnState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch state {
	case http.StateNew:
		c.open[conn] = struct{}{}
	case http.StateClosed, http.StateHijacked:
		delete(c.open, conn)
	}
}

func (c *connTracker) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.open)
}

func newTrackedServer(t *testing.T) (*httptest.Server, *connTracker) {
	t.Helper()
	tracker := &connTracker{open: make(map[net.Conn]struct{})}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /private\n")
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body><a href="/next">next</a></body></html>`)
		default:
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<html><body>ok</body></html>`)
		}
	}))
	srv.Config.ConnState = tracker.track
	srv.Start()
	t.Cleanup(srv.Close)
	return srv, tracker
}

func newTestCrawler(t *testing.T, knownFiles string) *Crawler {
	t.Helper()
	options, err := types.NewCrawlerOptions(&types.Options{
		MaxDepth:     3,
		FieldScope:   "rdn",
		BodyReadSize: math.MaxInt,
		Timeout:      1,
		Retries:      1,
		Concurrency:  2,
		Parallelism:  2,
		RateLimit:    150,
		Strategy:     "breadth-first",
		KnownFiles:   knownFiles,
		Silent:       true,
		OnResult:     func(output.Result) {},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })

	crawler, err := New(options)
	require.NoError(t, err)
	return crawler
}

func requireNoOpenConnections(t *testing.T, tracker *connTracker) {
	t.Helper()
	require.Eventually(t, func() bool { return tracker.count() == 0 }, 2*time.Second, 20*time.Millisecond,
		"connections still open: %d", tracker.count())
}

func TestCrawlClosesIdleConnections(t *testing.T) {
	srv, tracker := newTrackedServer(t)
	crawler := newTestCrawler(t, "")
	t.Cleanup(func() { _ = crawler.Close() })

	for i := 0; i < 3; i++ {
		require.NoError(t, crawler.Crawl(srv.URL))
		requireNoOpenConnections(t, tracker)
	}
}

func TestCloseClosesKnownFilesIdleConnections(t *testing.T) {
	srv, tracker := newTrackedServer(t)
	crawler := newTestCrawler(t, "robotstxt")

	require.NoError(t, crawler.Crawl(srv.URL))
	require.NoError(t, crawler.Close())
	requireNoOpenConnections(t, tracker)
}
