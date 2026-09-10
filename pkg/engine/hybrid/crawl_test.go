package hybrid

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
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
	cancel()

	select {
	case <-done:
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
