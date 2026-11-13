package hybrid

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/rod"
	"github.com/projectdiscovery/rod/lib/launcher"
	"github.com/projectdiscovery/rod/lib/proto"
	"github.com/stretchr/testify/require"
)

// setupBrowserTest launches a headless Chrome/Chromium and returns a browser
// and a page navigated to about:blank. Skips the test if no browser is found.
func setupBrowserTest(t *testing.T) (*rod.Browser, *rod.Page) {
	t.Helper()
	path, _ := launcher.LookPath()
	if path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}

	u, err := launcher.New().Leakless(true).Launch()
	if err != nil {
		t.Skipf("could not launch browser: %v", err)
	}
	browser := rod.New().ControlURL(u).MustConnect()
	t.Cleanup(func() { browser.MustClose() })

	page := browser.MustPage()
	t.Cleanup(func() { page.MustClose() })

	err = page.Navigate("about:blank")
	require.NoError(t, err)
	page.MustWaitLoad()

	return browser, page
}

// TestBasePageFreshTimeoutAfterExpiry verifies that saving a reference to the
// pre-timeout page (basePage) allows creating fresh timeouts for subsequent
// operations even after the original timeout context has expired.
//
// This is the mechanism used to fix #611: DOM inspection and HTML retrieval
// use basePage.Timeout() so they get independent time budgets instead of
// sharing the navigation timeout that may already be exhausted.
func TestBasePageFreshTimeoutAfterExpiry(t *testing.T) {
	_, page := setupBrowserTest(t)

	// Save basePage before applying timeout (mirrors the fix in crawl.go)
	basePage := page

	// Apply a very short timeout and force it to expire
	timedPage := page.Timeout(1 * time.Millisecond)
	time.Sleep(50 * time.Millisecond) // ensure timeout expires even under CI load

	// Use DOMGetDocument (a heavier operation) to verify the timeout is expired
	var depth = int(-1)
	getDoc := &proto.DOMGetDocument{Depth: &depth, Pierce: true}
	_, domErr := getDoc.Call(timedPage)
	require.Error(t, domErr, "timed page should fail after timeout expires")

	// Verify basePage can still create fresh timeouts and operate
	freshPage := basePage.Timeout(5 * time.Second)
	html, err := freshPage.HTML()
	require.NoError(t, err, "basePage with fresh timeout should still work")
	require.NotEmpty(t, html)
}

// TestDOMGetDocumentTimeoutDoesNotBlockHTML verifies that when DOMGetDocument
// times out on its own sub-timeout, HTML retrieval on a separate fresh timeout
// still succeeds. This directly tests the fix for #611.
func TestDOMGetDocumentTimeoutDoesNotBlockHTML(t *testing.T) {
	_, page := setupBrowserTest(t)

	// Save basePage before any timeout
	basePage := page

	// Simulate the scenario: give DOMGetDocument an impossibly short timeout
	domPage := basePage.Timeout(1 * time.Millisecond)
	time.Sleep(50 * time.Millisecond) // ensure timeout expires even under CI load

	var getDocumentDepth = int(-1)
	getDocument := &proto.DOMGetDocument{Depth: &getDocumentDepth, Pierce: true}
	_, domErr := getDocument.Call(domPage)

	// DOM inspection should fail due to timeout
	require.Error(t, domErr, "DOMGetDocument should fail with expired timeout")

	// But HTML retrieval with a fresh timeout from basePage should succeed
	body, err := basePage.Timeout(5 * time.Second).HTML()
	require.NoError(t, err, "HTML retrieval should succeed with fresh timeout from basePage")
	require.NotEmpty(t, body, "HTML body should not be empty")
}

func TestCancelDuringElementInteraction(t *testing.T) {
	if path, _ := launcher.LookPath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="#" onclick="while(true){}">x</a></body></html>`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	options, err := types.NewCrawlerOptions(&types.Options{
		Context:           ctx,
		MaxDepth:          2,
		FieldScope:        "rdn",
		BodyReadSize:      math.MaxInt,
		Timeout:           600,
		TimeStable:        1,
		Concurrency:       1,
		Parallelism:       1,
		RateLimit:         150,
		Strategy:          "depth-first",
		Headless:          true,
		HeadlessNoSandbox: true,
		OnResult:          func(output.Result) {},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })

	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })

	done := make(chan error, 1)
	go func() { done <- crawler.Crawl(srv.URL) }()

	time.Sleep(5 * time.Second)
	select {
	case err := <-done:
		t.Fatalf("Crawl completed before context cancellation: %v", err)
	default:
	}
	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Crawl did not return after context cancellation")
	}
}

func launchExternalBrowser(t *testing.T) (*rod.Browser, string) {
	t.Helper()
	path, _ := launcher.LookPath()
	if path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}

	wsURL, err := launcher.New().Leakless(true).Launch()
	if err != nil {
		t.Skipf("could not launch browser: %v", err)
	}
	browser := rod.New().ControlURL(wsURL).MustConnect()
	t.Cleanup(func() { _ = browser.Close() })

	return browser, wsURL
}

func TestCloseLeavesAttachedBrowserRunning(t *testing.T) {
	for _, noIncognito := range []bool{true, false} {
		name := "incognito"
		if noIncognito {
			name = "no-incognito"
		}
		t.Run(name, func(t *testing.T) {
			external, wsURL := launchExternalBrowser(t)

			options, err := types.NewCrawlerOptions(&types.Options{
				ChromeWSUrl:         wsURL,
				HeadlessNoIncognito: noIncognito,
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = options.Close() })

			crawler, err := New(options)
			require.NoError(t, err)
			require.NoError(t, crawler.Close())

			version, err := proto.BrowserGetVersion{}.Call(external)
			require.NoError(t, err, "attached browser should outlive crawler.Close")
			require.NotEmpty(t, version.Product)

			page, err := external.Page(proto.TargetCreateTarget{})
			require.NoError(t, err, "attached browser should still serve new pages")
			_ = page.Close()
		})
	}
}

func TestZeroTimeStableDoesNotPanic(t *testing.T) {
	if path, _ := launcher.LookPath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/a">a</a></body></html>`))
	}))
	defer srv.Close()

	options, err := types.NewCrawlerOptions(&types.Options{
		MaxDepth:          1,
		FieldScope:        "rdn",
		BodyReadSize:      math.MaxInt,
		Timeout:           10,
		TimeStable:        0,
		Concurrency:       1,
		Parallelism:       1,
		RateLimit:         150,
		Strategy:          "depth-first",
		Headless:          true,
		HeadlessNoSandbox: true,
		OnResult:          func(output.Result) {},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })

	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })

	require.NoError(t, crawler.Crawl(srv.URL))
}

func TestShouldParseSubresourceOncePerBody(t *testing.T) {
	newCrawler := func(t *testing.T, disableUnique bool) *Crawler {
		t.Helper()
		filter, err := filters.NewSimple()
		require.NoError(t, err)
		t.Cleanup(filter.Close)
		return &Crawler{Shared: &common.Shared{Options: &types.CrawlerOptions{
			Options:      &types.Options{DisableUniqueFilter: disableUnique},
			UniqueFilter: filter,
		}}}
	}
	script := []byte(`fetch("/api/v1/users")`)
	const scriptURL = "https://example.com/static/app.js"

	t.Run("repeated subresource body parsed once", func(t *testing.T) {
		c := newCrawler(t, false)
		require.True(t, c.shouldParse(false, scriptURL, nil, script))
		require.False(t, c.shouldParse(false, scriptURL, nil, script))
		require.True(t, c.shouldParse(false, scriptURL, nil, []byte(`fetch("/api/v2")`)))
	})
	t.Run("same body at another URL is parsed", func(t *testing.T) {
		c := newCrawler(t, false)
		require.True(t, c.shouldParse(false, "https://example.com/a.js", nil, script))
		require.True(t, c.shouldParse(false, "https://cdn.example.com/a.js", nil, script))
	})
	t.Run("same body with another Link header is parsed", func(t *testing.T) {
		c := newCrawler(t, false)
		first := map[string][]string{"Link": {`</a>; rel="preload"`}}
		second := map[string][]string{"link": {`</b>; rel="preload"`}}
		require.True(t, c.shouldParse(false, scriptURL, first, script))
		require.True(t, c.shouldParse(false, scriptURL, second, script))
		require.False(t, c.shouldParse(false, scriptURL, first, script))
	})
	t.Run("same body with another Location header is parsed", func(t *testing.T) {
		c := newCrawler(t, false)
		first := map[string][]string{"Location": {"/a"}}
		second := map[string][]string{"location": {"/b"}}
		require.True(t, c.shouldParse(false, scriptURL, first, script))
		require.True(t, c.shouldParse(false, scriptURL, second, script))
		require.False(t, c.shouldParse(false, scriptURL, first, script))
	})
	t.Run("empty subresource body always parsed", func(t *testing.T) {
		c := newCrawler(t, false)
		require.True(t, c.shouldParse(false, scriptURL, nil, nil))
		require.True(t, c.shouldParse(false, scriptURL, nil, nil))
	})
	t.Run("unique filter disabled", func(t *testing.T) {
		c := newCrawler(t, true)
		require.True(t, c.shouldParse(false, scriptURL, nil, script))
		require.True(t, c.shouldParse(false, scriptURL, nil, script))
	})
	t.Run("repeated main document skipped", func(t *testing.T) {
		c := newCrawler(t, false)
		html := []byte("<html>a</html>")
		require.True(t, c.shouldParse(true, "https://example.com/a", nil, html))
		require.False(t, c.shouldParse(true, "https://example.com/b", nil, html))
	})
}

// A blank page never fires FirstMeaningfulPaint, which used to stall the
// default and networkidle strategies until the request timed out.
func TestBlankPageDoesNotTimeOut(t *testing.T) {
	if path, _ := launcher.LookPath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()

	for _, strategy := range []string{"heuristic", "networkidle"} {
		t.Run(strategy, func(t *testing.T) {
			var results []output.Result
			options, err := types.NewCrawlerOptions(&types.Options{
				MaxDepth:          1,
				FieldScope:        "rdn",
				BodyReadSize:      math.MaxInt,
				Timeout:           8,
				TimeStable:        1,
				Concurrency:       1,
				Parallelism:       1,
				RateLimit:         150,
				Strategy:          "depth-first",
				PageLoadStrategy:  strategy,
				Headless:          true,
				HeadlessNoSandbox: true,
				OnResult:          func(r output.Result) { results = append(results, r) },
			})
			require.NoError(t, err)
			t.Cleanup(func() { _ = options.Close() })

			crawler, err := New(options)
			require.NoError(t, err)
			t.Cleanup(func() { _ = crawler.Close() })

			require.NoError(t, crawler.Crawl(srv.URL))
			require.NotEmpty(t, results)
			require.Empty(t, results[0].Error)
		})
	}
}

// Enabling lifecycle events replays the previous document's networkIdle, and
// an iframe goes idle before its parent; neither may end the wait.
func TestLoadWaitIgnoresStaleAndChildFrameEvents(t *testing.T) {
	_, page := setupBrowserTest(t)
	const slow = 2 * time.Second
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><iframe src="/frame"></iframe><img src="/slow"></body></html>`))
		case "/frame":
			_, _ = w.Write([]byte(`<html><body><h1>frame content</h1></body></html>`))
		case "/slow":
			time.Sleep(slow)
		}
	}))
	defer srv.Close()

	start := time.Now()
	wait := waitAnyLifecycleEvent(page, proto.PageLifecycleEventNameFirstMeaningfulPaint, proto.PageLifecycleEventNameNetworkIdle)
	require.NoError(t, page.Navigate(srv.URL))
	wait()
	require.GreaterOrEqual(t, time.Since(start), slow, "wait ended before the main frame loaded")
}

func TestIsStreamingResponse(t *testing.T) {
	tests := []struct {
		name         string
		resourceType proto.NetworkResourceType
		headers      []*proto.FetchHeaderEntry
		expected     bool
	}{
		{
			name:         "EventSource resource type",
			resourceType: proto.NetworkResourceTypeEventSource,
			expected:     true,
		},
		{
			name:         "WebSocket resource type",
			resourceType: proto.NetworkResourceTypeWebSocket,
			expected:     true,
		},
		{
			name:         "SSE text/event-stream content-type",
			resourceType: proto.NetworkResourceTypeXHR,
			headers: []*proto.FetchHeaderEntry{
				{Name: "Content-Type", Value: "text/event-stream"},
			},
			expected: true,
		},
		{
			name:         "SSE with charset parameter",
			resourceType: proto.NetworkResourceTypeFetch,
			headers: []*proto.FetchHeaderEntry{
				{Name: "content-type", Value: "text/event-stream; charset=utf-8"},
			},
			expected: true,
		},
		{
			name:         "Streaming JSON application/stream+json",
			resourceType: proto.NetworkResourceTypeFetch,
			headers: []*proto.FetchHeaderEntry{
				{Name: "Content-Type", Value: "application/stream+json"},
			},
			expected: true,
		},
		{
			name:         "NDJSON application/x-ndjson",
			resourceType: proto.NetworkResourceTypeXHR,
			headers: []*proto.FetchHeaderEntry{
				{Name: "Content-Type", Value: "application/x-ndjson"},
			},
			expected: true,
		},
		{
			name:         "Standard HTML response",
			resourceType: proto.NetworkResourceTypeDocument,
			headers: []*proto.FetchHeaderEntry{
				{Name: "Content-Type", Value: "text/html; charset=utf-8"},
			},
			expected: false,
		},
		{
			name:         "Standard JSON response",
			resourceType: proto.NetworkResourceTypeFetch,
			headers: []*proto.FetchHeaderEntry{
				{Name: "Content-Type", Value: "application/json"},
			},
			expected: false,
		},
		{
			name:         "JavaScript response",
			resourceType: proto.NetworkResourceTypeScript,
			headers: []*proto.FetchHeaderEntry{
				{Name: "Content-Type", Value: "application/javascript"},
			},
			expected: false,
		},
		{
			name:         "Empty headers",
			resourceType: proto.NetworkResourceTypeOther,
			headers:      nil,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStreamingResponse(tt.resourceType, tt.headers)
			require.Equal(t, tt.expected, got)
		})
	}
}

// A page that opens an EventSource used to keep the hijack body read running
// after the visit had already hit -timeout.
func TestEventSourceVisitDoesNotOutliveTimeout(t *testing.T) {
	if path, _ := launcher.LookPath(); path == "" {
		t.Skip("chrome/chromium not found, skipping browser test")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/events" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			flusher, ok := w.(http.Flusher)
			if !ok {
				return
			}
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-r.Context().Done():
					return
				case <-ticker.C:
					_, _ = w.Write([]byte("data: ping\n\n"))
					flusher.Flush()
				}
			}
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script>new EventSource("/events")</script>ok</body></html>`))
	}))
	t.Cleanup(srv.Close)

	const timeoutSeconds = 12
	options, err := types.NewCrawlerOptions(&types.Options{
		MaxDepth:          1,
		FieldScope:        "rdn",
		BodyReadSize:      math.MaxInt,
		Timeout:           timeoutSeconds,
		TimeStable:        1,
		Concurrency:       1,
		Parallelism:       1,
		RateLimit:         150,
		Strategy:          "depth-first",
		PageLoadStrategy:  "load",
		Headless:          true,
		HeadlessNoSandbox: true,
		OnResult:          func(output.Result) {},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })

	crawler, err := New(options)
	require.NoError(t, err)
	t.Cleanup(func() { _ = crawler.Close() })

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- crawler.Crawl(srv.URL) }()

	// load does not wait for the open stream. Past this, the visit is stuck
	// in the body read until -timeout.
	limit := 10 * time.Second
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if elapsed := time.Since(start); elapsed > limit {
			t.Fatalf("visit took %s, past %s", elapsed, limit)
		}
	case <-time.After(limit):
		t.Fatalf("event stream held the visit open for %s", time.Since(start))
	}
}
