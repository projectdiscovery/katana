package browser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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
