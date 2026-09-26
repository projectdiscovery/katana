package hybrid

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

// --- browser-free tests -----------------------------------------------------

func TestRunHooks_ZeroValueIsNoOp(t *testing.T) {
	req := &navigation.Request{URL: "http://example.test/"}
	require.NoError(t, runBeforeNavigate(context.Background(), Hooks{}, nil, req))
	require.NoError(t, runAfterLoad(context.Background(), Hooks{}, nil, req, &navigation.Response{}))
}

type hookTestCtxKey struct{}

func TestRunBeforeNavigate_PassesArgumentsAndWrapsError(t *testing.T) {
	sentinel := errors.New("seed failed")
	ctx := context.WithValue(context.Background(), hookTestCtxKey{}, "marker")
	req := &navigation.Request{URL: "http://example.test/login", Depth: 0}

	var gotCtx context.Context
	var gotReq *navigation.Request
	hooks := Hooks{BeforeNavigate: func(c context.Context, _ *rod.Page, r *navigation.Request) error {
		gotCtx, gotReq = c, r
		return sentinel
	}}

	err := runBeforeNavigate(ctx, hooks, nil, req)
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel, "the hook's error must stay inspectable")
	require.Contains(t, err.Error(), "BeforeNavigate hook failed")
	require.Contains(t, err.Error(), req.URL, "the reported error must name the URL")
	require.Same(t, req, gotReq)
	require.Equal(t, "marker", gotCtx.Value(hookTestCtxKey{}))
}

func TestRunAfterLoad_CanAttachExtraAndWrapsError(t *testing.T) {
	req := &navigation.Request{URL: "http://example.test/"}
	resp := &navigation.Response{}

	hooks := Hooks{AfterLoad: func(_ context.Context, _ *rod.Page, _ *navigation.Request, r *navigation.Response) error {
		if r.Extra == nil {
			r.Extra = map[string]any{}
		}
		r.Extra["probe"] = "ok"
		return nil
	}}
	require.NoError(t, runAfterLoad(context.Background(), hooks, nil, req, resp))
	require.Equal(t, "ok", resp.Extra["probe"])

	sentinel := errors.New("probe failed")
	hooks.AfterLoad = func(context.Context, *rod.Page, *navigation.Request, *navigation.Response) error { return sentinel }
	err := runAfterLoad(context.Background(), hooks, nil, req, resp)
	require.ErrorIs(t, err, sentinel)
	require.Contains(t, err.Error(), "AfterLoad hook failed")
	require.Contains(t, err.Error(), req.URL)
}

func TestSetHooks_SnapshotsCallerStruct(t *testing.T) {
	original := func(context.Context, *rod.Page, *navigation.Request) error { return nil }
	mutated := func(context.Context, *rod.Page, *navigation.Request) error { return errors.New("mutated") }

	hooks := &Hooks{BeforeNavigate: original}
	c := &Crawler{}
	c.SetHooks(hooks)
	hooks.BeforeNavigate = mutated

	require.NotNil(t, c.hooks.BeforeNavigate)
	require.NoError(t, c.hooks.BeforeNavigate(context.Background(), nil, nil),
		"the crawler must keep the snapshot taken at SetHooks time")
}

func TestSetHooks_NilClears(t *testing.T) {
	c := &Crawler{}
	c.SetHooks(&Hooks{AfterLoad: func(context.Context, *rod.Page, *navigation.Request, *navigation.Response) error { return nil }})
	c.SetHooks(nil)
	require.Nil(t, c.hooks.BeforeNavigate)
	require.Nil(t, c.hooks.AfterLoad)
}

func TestResponseExtra_JSON(t *testing.T) {
	out, err := json.Marshal(navigation.Response{StatusCode: 200})
	require.NoError(t, err)
	require.NotContains(t, string(out), `"extra"`, "an unset Extra must not change existing output")

	out, err = json.Marshal(navigation.Response{StatusCode: 200, Extra: map[string]any{"k": "v"}})
	require.NoError(t, err)
	require.Contains(t, string(out), `"extra":{"k":"v"}`)
}

// --- browser tests (skipped when no Chrome/Chromium is installed) -----------

// hookTestOptions returns crawler options for the browser tests. The two
// HeadlessOptionalArguments keep Chrome away from the OS credential store:
// buildChromeLauncher deletes rod's default use-mock-keychain flag, and without
// it every launch on macOS raises a Keychain access prompt.
func hookTestOptions(onResult func(output.Result)) *types.Options {
	return &types.Options{
		HeadlessOptionalArguments: []string{"use-mock-keychain=true", "password-store=basic"},
		MaxDepth:                  1,
		FieldScope:                "rdn",
		BodyReadSize:              math.MaxInt,
		Timeout:                   10,
		TimeStable:                0,
		PageLoadStrategy:          "load",
		Concurrency:               1,
		Parallelism:               1,
		RateLimit:                 150,
		Strategy:                  "depth-first",
		Headless:                  true,
		HeadlessNoSandbox:         true,
		OnResult:                  onResult,
	}
}

func newHookTestCrawler(t *testing.T, onResult func(output.Result)) *Crawler {
	t.Helper()
	if path, _ := launcher.LookPath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}
	options, err := types.NewCrawlerOptions(hookTestOptions(onResult))
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })

	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })
	return crawler
}

type resultSink struct {
	mu      sync.Mutex
	results []output.Result
}

func (s *resultSink) add(r output.Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = append(s.results, r)
}

func (s *resultSink) forURL(u string) (output.Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.results {
		if r.Request != nil && strings.TrimSuffix(r.Request.URL, "/") == strings.TrimSuffix(u, "/") {
			return r, true
		}
	}
	return output.Result{}, false
}

// TestHooks_SeedBeforeNavigateAndReadAfterLoad covers both callbacks on a real
// crawl: a cookie set in BeforeNavigate must reach the server on the very first
// request, and data read from the live page in AfterLoad must arrive on the
// result via Response.Extra.
func TestHooks_SeedBeforeNavigateAndReadAfterLoad(t *testing.T) {
	var sawCookie atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil && c.Value == "seeded" {
			sawCookie.Store(true)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script>window.__probe = "rendered";</script></body></html>`))
	}))
	defer srv.Close()

	sink := &resultSink{}
	crawler := newHookTestCrawler(t, sink.add)

	var beforeCalls, afterCalls atomic.Int32
	crawler.SetHooks(&Hooks{
		BeforeNavigate: func(ctx context.Context, page *rod.Page, req *navigation.Request) error {
			beforeCalls.Add(1)
			return page.Context(ctx).SetCookies([]*proto.NetworkCookieParam{{
				Name: "session", Value: "seeded", URL: srv.URL,
			}})
		},
		AfterLoad: func(ctx context.Context, page *rod.Page, req *navigation.Request, resp *navigation.Response) error {
			afterCalls.Add(1)
			obj, err := page.Context(ctx).Eval(`() => window.__probe`)
			if err != nil {
				return err
			}
			resp.Extra = map[string]any{"probe": obj.Value.Str(), "depth": req.Depth}
			return nil
		},
	})

	require.NoError(t, crawler.Crawl(srv.URL))

	require.True(t, sawCookie.Load(), "cookie seeded in BeforeNavigate must be sent with the first request")
	require.GreaterOrEqual(t, beforeCalls.Load(), int32(1))
	require.GreaterOrEqual(t, afterCalls.Load(), int32(1))

	res, ok := sink.forURL(srv.URL)
	require.True(t, ok, "expected a result for the seed URL")
	require.Empty(t, res.Error)
	require.NotNil(t, res.Response)
	require.Equal(t, "rendered", res.Response.Extra["probe"])
	require.Equal(t, 0, res.Response.Extra["depth"])
}

// TestHooks_BeforeNavigateErrorSkipsRequest verifies that a failing
// BeforeNavigate aborts the request before any traffic is sent and that the
// failure is reported on the result, naming the URL.
func TestHooks_BeforeNavigateErrorSkipsRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>hi</body></html>`))
	}))
	defer srv.Close()

	sink := &resultSink{}
	crawler := newHookTestCrawler(t, sink.add)

	var afterCalled atomic.Bool
	crawler.SetHooks(&Hooks{
		BeforeNavigate: func(context.Context, *rod.Page, *navigation.Request) error {
			return errors.New("no session available")
		},
		AfterLoad: func(context.Context, *rod.Page, *navigation.Request, *navigation.Response) error {
			afterCalled.Store(true)
			return nil
		},
	})

	require.NoError(t, crawler.Crawl(srv.URL))

	require.Zero(t, hits.Load(), "no request may be sent when BeforeNavigate fails")
	require.False(t, afterCalled.Load(), "AfterLoad must not run for an aborted request")
	res, ok := sink.forURL(srv.URL)
	require.True(t, ok, "the aborted request must still be reported")
	require.Contains(t, res.Error, "BeforeNavigate hook failed")
	require.Contains(t, res.Error, "no session available")
	require.Contains(t, res.Error, srv.URL)
}

// TestHooks_AfterLoadErrorIsReported verifies that a failing AfterLoad is
// reported like any other navigation error.
func TestHooks_AfterLoadErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>hi</body></html>`))
	}))
	defer srv.Close()

	sink := &resultSink{}
	crawler := newHookTestCrawler(t, sink.add)
	crawler.SetHooks(&Hooks{
		AfterLoad: func(context.Context, *rod.Page, *navigation.Request, *navigation.Response) error {
			return errors.New("capture failed")
		},
	})

	require.NoError(t, crawler.Crawl(srv.URL))

	res, ok := sink.forURL(srv.URL)
	require.True(t, ok)
	require.Contains(t, res.Error, "AfterLoad hook failed")
	require.Contains(t, res.Error, "capture failed")
}
