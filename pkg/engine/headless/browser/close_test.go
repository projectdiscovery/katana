package browser

import (
	"os"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
)

// TestCloseAllRemovesDataDirsAndBlocksNewPages checks that CloseAll closes
// every checked-out browser and removes its data dir, and that a page created
// after CloseAll is closed at once instead of being left behind.
func TestCloseAllRemovesDataDirsAndBlocksNewPages(t *testing.T) {
	skipIfNoBrowser(t)

	l, err := NewLauncher(LauncherOptions{MaxBrowsers: 3, NoSandbox: true})
	require.NoError(t, err)

	var dirs []string
	for i := 0; i < 2; i++ {
		page, err := l.GetPageFromPool()
		require.NoError(t, err)
		require.DirExists(t, page.userDataDir)
		dirs = append(dirs, page.userDataDir)
	}

	l.CloseAll()
	for _, dir := range dirs {
		_, err := os.Stat(dir)
		require.True(t, os.IsNotExist(err), "data dir %s must be removed", dir)
	}
	l.liveMu.Lock()
	require.Empty(t, l.live)
	l.liveMu.Unlock()

	_, err = l.GetPageFromPool()
	require.Error(t, err, "a page created after CloseAll must not be handed out")
	l.liveMu.Lock()
	require.Empty(t, l.live)
	l.liveMu.Unlock()

	l.Close()
}

// TestCloseAllClosesPageInExternalBrowser checks that the page is closed with
// a fresh context. CloseAll cancels the page context first, so closing with it
// would leave katana's tab open in the external browser.
func TestCloseAllClosesPageInExternalBrowser(t *testing.T) {
	skipIfNoBrowser(t)

	chrome := launcher.New().Headless(true).NoSandbox(true)
	wsURL, err := chrome.Launch()
	require.NoError(t, err)
	defer func() {
		chrome.Kill()
		chrome.Cleanup() // waits for Chrome to exit, then removes rod's temporary profile
	}()

	user := rod.New().ControlURL(wsURL)
	require.NoError(t, user.Connect())
	defer func() { _ = user.Close() }()

	l, err := NewLauncher(LauncherOptions{MaxBrowsers: 1, NoSandbox: true, ChromeWSUrl: wsURL})
	require.NoError(t, err)
	defer l.Close()

	page, err := l.GetPageFromPool()
	require.NoError(t, err)
	require.True(t, hasTarget(t, user, page.TargetID))

	l.CloseAll()
	require.False(t, hasTarget(t, user, page.TargetID), "katana's tab must be closed in the external browser")
	require.True(t, isBrowserConnected(user), "the external browser must keep running")
}

func hasTarget(t *testing.T, browser *rod.Browser, id proto.TargetTargetID) bool {
	t.Helper()
	targets, err := proto.TargetGetTargets{}.Call(browser)
	require.NoError(t, err)
	for _, target := range targets.TargetInfos {
		if target.TargetID == id {
			return true
		}
	}
	return false
}
