package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetPageFromPoolReleasesSlotWhenCreateFails(t *testing.T) {
	launcher, err := NewLauncher(LauncherOptions{
		MaxBrowsers:  1,
		ChromiumPath: "/nonexistent/chrome",
		NoSandbox:    true,
	})
	require.NoError(t, err)
	defer launcher.Close()

	_, err = launcher.GetPageFromPool()
	require.Error(t, err)
	require.Len(t, launcher.browserPool, 1)

	done := make(chan struct{})
	go func() {
		_, _ = launcher.GetPageFromPool()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("GetPageFromPool blocked after a failed page creation")
	}
}

func TestPutBrowserToPoolReplenishesDiscardedSlot(t *testing.T) {
	skipIfNoBrowser(t)

	launcher, err := NewLauncher(LauncherOptions{
		MaxBrowsers: 1,
		NoSandbox:   true,
	})
	require.NoError(t, err)
	defer launcher.Close()

	page, err := launcher.GetPageFromPool()
	require.NoError(t, err)
	require.Empty(t, launcher.browserPool)

	// Simulate a page whose action context hit a deadline. The discarded page
	// must release its pool slot; otherwise the next GetPageFromPool blocks.
	page.cancel()
	launcher.PutBrowserToPool(page)
	require.Len(t, launcher.browserPool, 1)

	replacement, err := launcher.GetPageFromPool()
	require.NoError(t, err)
	require.NotNil(t, replacement)
	launcher.PutBrowserToPool(replacement)
}
