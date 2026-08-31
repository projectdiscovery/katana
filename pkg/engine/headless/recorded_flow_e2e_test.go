package headless_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/projectdiscovery/katana/internal/testutils/authlab"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/require"
)

func skipIfNoBrowser(t *testing.T) {
	t.Helper()
	if path, found := launcher.LookPath(); !found || path == "" {
		t.Skip("chrome/chromium not found, skipping recorded-flow crawl e2e")
	}
}

func writeTempFlow(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flow.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

type resultCollector struct {
	mu   sync.Mutex
	urls []string
}

func (c *resultCollector) callback(rr *output.Result) {
	if rr == nil || rr.Request == nil {
		return
	}
	c.mu.Lock()
	c.urls = append(c.urls, rr.Request.URL)
	c.mu.Unlock()
}

func (c *resultCollector) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.urls))
	copy(out, c.urls)
	return out
}

func containsURL(urls []string, substr string) bool {
	for _, u := range urls {
		if strings.Contains(u, substr) {
			return true
		}
	}
	return false
}

func TestE2E_Crawl_WithRecordedFlow_DiscoversGatedPages(t *testing.T) {
	skipIfNoBrowser(t)

	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	flowPath := writeTempFlow(t, authlab.ChromeRecordingSimple(lab.URL))
	steps, err := auth.StepsFromFile(flowPath, authlab.Username, authlab.Password)
	require.NoError(t, err)

	collector := &resultCollector{}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := crawler.New(crawler.Options{
		Context:         ctx,
		MaxBrowsers:     1,
		MaxDepth:        2,
		PageMaxTimeout:  30 * time.Second,
		NoSandbox:       true,
		AuthUsername:    authlab.Username,
		AuthPassword:    authlab.Password,
		AuthSteps:       steps,
		RequestCallback: collector.callback,
		Logger:          slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		ScopeValidator:  func(u string) bool { return strings.HasPrefix(u, lab.URL) },
	})
	require.NoError(t, err)
	t.Cleanup(c.Close)

	require.NoError(t, c.Crawl(lab.URL+"/app/dashboard"))

	urls := collector.list()
	require.True(t, containsURL(urls, "/app/dashboard") || lab.DashboardHits.Load() > 0,
		"expected dashboard visit, urls=%v", urls)
	require.True(t, containsURL(urls, "/app/secret") || lab.SecretHits.Load() > 0,
		"recorded flow should unlock gated /app/secret during crawl, urls=%v secretHits=%d",
		urls, lab.SecretHits.Load())
	require.GreaterOrEqual(t, lab.LoginPosts.Load(), int64(1), "login must have been submitted")
}

func TestE2E_Crawl_WithoutAuth_CannotReachSecret(t *testing.T) {
	skipIfNoBrowser(t)

	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	collector := &resultCollector{}
	// Bound the crawl with MaxCrawlDuration so CI (cold Chrome cache / slow
	// runners) exits cleanly instead of relying on the parent context deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c, err := crawler.New(crawler.Options{
		Context:          ctx,
		MaxBrowsers:      1,
		MaxDepth:         1,
		MaxCrawlDuration: 30 * time.Second,
		PageMaxTimeout:   15 * time.Second,
		NoSandbox:        true,
		RequestCallback:  collector.callback,
		Logger:           slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		ScopeValidator:   func(u string) bool { return strings.HasPrefix(u, lab.URL) },
	})
	require.NoError(t, err)
	t.Cleanup(c.Close)

	// Start from the public home — without a recorded flow the crawler should
	// never successfully fetch the gated secret page content.
	require.NoError(t, c.Crawl(lab.URL+"/"))
	urls := collector.list()
	require.NotEmpty(t, urls, "crawl must produce some public-page traffic")
	require.Equal(t, int64(0), lab.SecretHits.Load(), "unauthenticated crawl must not hit /app/secret")
	require.False(t, containsURL(urls, "/app/secret"))
}

func TestE2E_Crawl_UsernameFirstRecordedFlow(t *testing.T) {
	skipIfNoBrowser(t)

	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	steps, err := auth.StepsFromData([]byte(authlab.ChromeRecordingStep(lab.URL)), authlab.Username, authlab.Password)
	require.NoError(t, err)

	collector := &resultCollector{}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c, err := crawler.New(crawler.Options{
		Context:         ctx,
		MaxBrowsers:     1,
		MaxDepth:        2,
		PageMaxTimeout:  30 * time.Second,
		NoSandbox:       true,
		AuthUsername:    authlab.Username,
		AuthPassword:    authlab.Password,
		AuthSteps:       steps,
		RequestCallback: collector.callback,
		Logger:          slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		ScopeValidator:  func(u string) bool { return strings.HasPrefix(u, lab.URL) },
	})
	require.NoError(t, err)
	t.Cleanup(c.Close)

	require.NoError(t, c.Crawl(lab.URL+"/app/dashboard"))
	require.GreaterOrEqual(t, lab.LoginPosts.Load(), int64(1))
	require.True(t, containsURL(collector.list(), "/app/secret") || lab.SecretHits.Load() > 0,
		"username-first recorded flow should unlock secret pages")
}
