package hybrid

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/navigation"
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

// sessionSite serves / with root and records, for each linked page /pN,
// whether the request carried the cookie sid=s3cr3t.
type sessionSite struct {
	mu          sync.Mutex
	withSession map[string]bool
}

func newSessionSite(t *testing.T, root func(w http.ResponseWriter, r *http.Request)) (*sessionSite, *httptest.Server) {
	site := &sessionSite{withSession: map[string]bool{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Path == "/" {
			root(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/p") {
			http.NotFound(w, r)
			return
		}
		c, err := r.Cookie("sid")
		site.mu.Lock()
		site.withSession[r.URL.Path] = site.withSession[r.URL.Path] || (err == nil && c.Value == "s3cr3t")
		site.mu.Unlock()
		_, _ = w.Write([]byte("<html><body>page</body></html>"))
	}))
	t.Cleanup(srv.Close)
	return site, srv
}

func (site *sessionSite) requireAllWithSession(t *testing.T, pages int) {
	site.mu.Lock()
	defer site.mu.Unlock()
	require.Len(t, site.withSession, pages, "every linked page must be crawled")
	for path, ok := range site.withSession {
		require.True(t, ok, "%s was crawled without the session cookie", path)
	}
}

func newAgentsCrawler(t *testing.T) *Crawler {
	if path := hookChromePath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}
	opts := hookTestOptions(func(output.Result) {})
	opts.BrowserAgents = 3
	opts.Concurrency = 3
	opts.Parallelism = 3
	options, err := types.NewCrawlerOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })
	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })
	require.Len(t, crawler.agents, 3)
	require.NotNil(t, crawler.cookies)
	return crawler
}

func links(pages int) string {
	var b strings.Builder
	for i := 1; i <= pages; i++ {
		fmt.Fprintf(&b, `<a href="/p%d">p%d</a> `, i, i)
	}
	return b.String()
}

// TestCookiesSharedAcrossBrowserAgents crawls with three browser agents a site
// that sets a session cookie on / and requires it on every linked page. Each
// agent is its own Chrome, so without a shared store the pages that another
// agent takes lose the session.
func TestCookiesSharedAcrossBrowserAgents(t *testing.T) {
	const pages = 9
	site, srv := newSessionSite(t, func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s3cr3t", Path: "/", HttpOnly: true})
		_, _ = w.Write([]byte("<html><body>" + links(pages) + "</body></html>"))
	})
	require.NoError(t, newAgentsCrawler(t).Crawl(srv.URL))
	site.requireAllWithSession(t, pages)
}

// TestCookiesSharedFromShadowDOMLinks holds links found only in the shadow
// DOM until the cookies of the page are shared, like links of the HTML.
func TestCookiesSharedFromShadowDOMLinks(t *testing.T) {
	const pages = 9
	site, srv := newSessionSite(t, func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "s3cr3t", Path: "/", HttpOnly: true})
		_, _ = w.Write([]byte(`<html><body><div id="host"></div><script>
			document.getElementById("host").attachShadow({mode: "open"}).innerHTML = ` + "`" + links(pages) + "`" + `;
		</script></body></html>`))
	})
	crawler := newAgentsCrawler(t)
	// A slow AfterLoad on the root page widens the time between finding the
	// links and sharing the cookies, so a link queued too early is taken by
	// another agent without the session.
	crawler.SetHooks(&Hooks{AfterLoad: func(ctx context.Context, _ *rod.Page, req *navigation.Request, _ *navigation.Response) error {
		if strings.TrimSuffix(req.URL, "/") == strings.TrimSuffix(srv.URL, "/") {
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Second):
			}
		}
		return nil
	}})
	require.NoError(t, crawler.Crawl(srv.URL))
	site.requireAllWithSession(t, pages)
}

// TestCookiesSharedFromBeforeNavigate shares a cookie that the BeforeNavigate
// hook sets, as the hook is part of the navigation.
func TestCookiesSharedFromBeforeNavigate(t *testing.T) {
	const pages = 9
	site, srv := newSessionSite(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>" + links(pages) + "</body></html>"))
	})
	crawler := newAgentsCrawler(t)
	crawler.SetHooks(&Hooks{BeforeNavigate: func(ctx context.Context, page *rod.Page, req *navigation.Request) error {
		// Only the root page logs in; the other pages need the shared cookie.
		if strings.TrimSuffix(req.URL, "/") != strings.TrimSuffix(srv.URL, "/") {
			return nil
		}
		return page.Context(ctx).SetCookies([]*proto.NetworkCookieParam{{Name: "sid", Value: "s3cr3t", URL: srv.URL + "/"}})
	}})
	require.NoError(t, crawler.Crawl(srv.URL))
	site.requireAllWithSession(t, pages)
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

func TestCookieKeyPartition(t *testing.T) {
	t.Parallel()
	plain := &proto.NetworkCookie{Name: "id", Value: "a", Domain: "app.test", Path: "/", Session: true}
	partitioned := &proto.NetworkCookie{Name: "id", Value: "b", Domain: "app.test", Path: "/", Session: true,
		PartitionKey: &proto.NetworkCookiePartitionKey{TopLevelSite: "https://shop.test"}}
	other := &proto.NetworkCookie{Name: "id", Value: "c", Domain: "app.test", Path: "/", Session: true,
		PartitionKey: &proto.NetworkCookiePartitionKey{TopLevelSite: "https://news.test"}}
	require.NotEqual(t, newCookieKey(plain), newCookieKey(partitioned))
	require.NotEqual(t, newCookieKey(partitioned), newCookieKey(other))

	s := newCookieStore()
	after := map[cookieKey]*proto.NetworkCookie{}
	for _, c := range []*proto.NetworkCookie{plain, partitioned, other} {
		after[newCookieKey(c)] = c
	}
	s.record(map[cookieKey]*proto.NetworkCookie{}, after)
	require.Len(t, s.cookies, 3, "partitioned cookies must not collapse")
	require.Nil(t, s.cookies[newCookieKey(plain)].PartitionKey)
	require.Equal(t, "https://shop.test", s.cookies[newCookieKey(partitioned)].PartitionKey.TopLevelSite)
	require.Equal(t, "https://news.test", s.cookies[newCookieKey(other)].PartitionKey.TopLevelSite)
}

func hookChromePath() string {
	path, _ := launcher.LookPath()
	return path
}
