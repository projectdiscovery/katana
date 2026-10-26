package hybrid

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestCookieStoreRecordChangesOnly(t *testing.T) {
	t.Parallel()
	cookie := func(name, value string) *proto.NetworkCookie {
		return &proto.NetworkCookie{Name: name, Value: value, Domain: "app.test", Path: "/", Session: true, Expires: -1}
	}
	key := func(name string) cookieKey { return cookieKey{name: name, domain: "app.test", path: "/"} }
	snapshot := func(cookies ...*proto.NetworkCookie) map[cookieKey]*proto.NetworkCookie {
		m := map[cookieKey]*proto.NetworkCookie{}
		for _, c := range cookies {
			m[key(c.Name)] = c
		}
		return m
	}

	s := newCookieStore()
	// Agent A logs in: sid is new.
	s.record(snapshot(), snapshot(cookie("sid", "v2")))
	require.Equal(t, "v2", s.cookies[key("sid")].Value)
	require.Zero(t, s.cookies[key("sid")].Expires, "a session cookie must not be set as expired")
	require.Equal(t, "http://app.test/", s.cookies[key("sid")].URL, "a host-only cookie is set by URL")
	require.Empty(t, s.cookies[key("sid")].Domain)

	// A domain cookie keeps its domain.
	wide := &proto.NetworkCookie{Name: "wide", Value: "1", Domain: ".app.test", Path: "/", Expires: 4102444800}
	s.record(snapshot(), map[cookieKey]*proto.NetworkCookie{{name: "wide", domain: ".app.test", path: "/"}: wide})
	got := s.cookies[cookieKey{name: "wide", domain: ".app.test", path: "/"}]
	require.Equal(t, ".app.test", got.Domain)
	require.Empty(t, got.URL)
	require.Equal(t, proto.TimeSinceEpoch(4102444800), got.Expires)

	// Agent B still has an old sid that its page did not change: no overwrite.
	s.record(snapshot(cookie("sid", "v1")), snapshot(cookie("sid", "v1")))
	require.Equal(t, "v2", s.cookies[key("sid")].Value)

	// Agent B logs out: sid is deleted.
	s.record(snapshot(cookie("sid", "v2")), snapshot())
	require.Equal(t, proto.TimeSinceEpoch(1), s.cookies[key("sid")].Expires)
}

// TestCookiesSharedAcrossBrowserAgents crawls with three browser agents a site
// that sets a session cookie on / and requires it on every linked page. Each
// agent is its own Chrome, so without a shared store the pages that another
// agent takes lose the session.
func TestCookiesSharedAcrossBrowserAgents(t *testing.T) {
	const pages = 9
	var mu sync.Mutex
	withSession := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" {
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s3cr3t", Path: "/", HttpOnly: true})
			var links strings.Builder
			for i := 1; i <= pages; i++ {
				fmt.Fprintf(&links, `<a href="/p%d">p%d</a> `, i, i)
			}
			_, _ = w.Write([]byte("<html><body>" + links.String() + "</body></html>"))
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/p") {
			http.NotFound(w, r)
			return
		}
		c, err := r.Cookie("sid")
		mu.Lock()
		withSession[r.URL.Path] = withSession[r.URL.Path] || (err == nil && c.Value == "s3cr3t")
		mu.Unlock()
		_, _ = w.Write([]byte("<html><body>page</body></html>"))
	}))
	defer srv.Close()

	opts := hookTestOptions(func(output.Result) {})
	opts.BrowserAgents = 3
	opts.Concurrency = 3
	opts.Parallelism = 3
	options, err := types.NewCrawlerOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })
	if path := hookChromePath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}
	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })
	require.Len(t, crawler.agents, 3)
	require.NotNil(t, crawler.cookies)

	require.NoError(t, crawler.Crawl(srv.URL))

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, withSession, pages, "every linked page must be crawled")
	for path, ok := range withSession {
		require.True(t, ok, "%s was crawled without the session cookie", path)
	}
}

func TestCookieStoreOnlyWithSeveralAgents(t *testing.T) {
	if path := hookChromePath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}
	options, err := types.NewCrawlerOptions(hookTestOptions(func(output.Result) {}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })
	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })
	require.Nil(t, crawler.cookies, "one agent keeps its cookies in its own browser")
}

func hookChromePath() string {
	path, _ := launcher.LookPath()
	return path
}
