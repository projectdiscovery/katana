package browser

import (
	"path/filepath"
	"testing"

	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/stretchr/testify/require"
)

func TestNewLauncherDefaults(t *testing.T) {
	t.Run("empty strategy defaults to heuristic", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
		})
		require.NoError(t, err)
		require.Equal(t, "heuristic", l.opts.PageLoadStrategy)
	})

	t.Run("explicit strategy is preserved", func(t *testing.T) {
		for _, strategy := range []string{"none", "load", "domcontentloaded", "networkidle", "heuristic"} {
			l, err := NewLauncher(LauncherOptions{
				MaxBrowsers:      1,
				PageLoadStrategy: strategy,
			})
			require.NoError(t, err)
			require.Equal(t, strategy, l.opts.PageLoadStrategy)
		}
	})

	t.Run("zero DOMWaitTime defaults to 5", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
		})
		require.NoError(t, err)
		require.Equal(t, 5, l.opts.DOMWaitTime)
	})

	t.Run("negative DOMWaitTime defaults to 5", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			DOMWaitTime: -1,
		})
		require.NoError(t, err)
		require.Equal(t, 5, l.opts.DOMWaitTime)
	})

	t.Run("positive DOMWaitTime is preserved", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			DOMWaitTime: 10,
		})
		require.NoError(t, err)
		require.Equal(t, 10, l.opts.DOMWaitTime)
	})

	t.Run("ChromeWSUrl is passed through", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			ChromeWSUrl: "ws://localhost:9222/devtools/browser/abc",
		})
		require.NoError(t, err)
		require.Equal(t, "ws://localhost:9222/devtools/browser/abc", l.opts.ChromeWSUrl)
	})
}

func TestLauncherIncognitoAndProfilePreservation(t *testing.T) {
	t.Run("incognito remains enabled by default", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{MaxBrowsers: 1})
		require.NoError(t, err)
		require.True(t, l.shouldUseIncognito())
	})

	t.Run("incognito is disabled when requested", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			NoIncognito: true,
			UserDataDir: "/tmp/profile",
		})
		require.NoError(t, err)
		require.False(t, l.shouldUseIncognito())
	})

	t.Run("profile preserving mode skips conflicting flags", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			NoIncognito: true,
			UserDataDir: "/tmp/profile",
		})
		require.NoError(t, err)

		for _, flagName := range []string{"use-mock-keychain", "password-store", "disable-extensions", "enable-automation"} {
			require.True(t, l.shouldSkipHeadlessFlag(flagName), "expected %s to be skipped", flagName)
		}
		require.False(t, l.shouldSkipHeadlessFlag("disable-gpu"))
	})

	t.Run("default mode keeps standard headless flags", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			UserDataDir: "/tmp/profile",
		})
		require.NoError(t, err)
		require.False(t, l.shouldSkipHeadlessFlag("use-mock-keychain"))
	})

	t.Run("only exact user supplied profile directory is preserved", func(t *testing.T) {
		profileDir := filepath.Join(t.TempDir(), "profile")
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers: 1,
			UserDataDir: profileDir,
		})
		require.NoError(t, err)

		require.True(t, l.shouldPreserveUserDataDir(profileDir))
		require.False(t, l.shouldPreserveUserDataDir(filepath.Join(profileDir, "nested")))
		require.False(t, l.shouldPreserveUserDataDir(""))
	})
}

func TestLauncherSystemChromeResolution(t *testing.T) {
	origLookPath := lookPath
	t.Cleanup(func() {
		lookPath = origLookPath
	})

	t.Run("system chrome enabled and chrome is discovered", func(t *testing.T) {
		lookPath = func() (string, bool) {
			return "/usr/bin/mock-chromium", true
		}

		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:        1,
			UseInstalledChrome: true,
		})
		require.NoError(t, err)

		cl, err := l.createChromeLauncher(t.TempDir())
		require.NoError(t, err)
		require.Equal(t, "/usr/bin/mock-chromium", cl.Get(flags.Bin))
	})

	t.Run("system chrome enabled and chrome not found returns error", func(t *testing.T) {
		lookPath = func() (string, bool) {
			return "", false
		}

		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:        1,
			UseInstalledChrome: true,
		})
		require.NoError(t, err)

		cl, err := l.createChromeLauncher(t.TempDir())
		require.Error(t, err)
		require.Nil(t, cl)
		require.Contains(t, err.Error(), "the chrome browser is not installed")
	})

	t.Run("explicit chromium path takes precedence over lookpath", func(t *testing.T) {
		lookPath = func() (string, bool) {
			return "/usr/bin/should-not-use", true
		}

		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:        1,
			UseInstalledChrome: true,
			ChromiumPath:       "/custom/path/to/chrome",
		})
		require.NoError(t, err)

		cl, err := l.createChromeLauncher(t.TempDir())
		require.NoError(t, err)
		require.Equal(t, "/custom/path/to/chrome", cl.Get(flags.Bin))
	})

	t.Run("explicit chromium path works without system chrome flag", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:  1,
			ChromiumPath: "/custom/path/to/chrome",
		})
		require.NoError(t, err)

		cl, err := l.createChromeLauncher(t.TempDir())
		require.NoError(t, err)
		require.Equal(t, "/custom/path/to/chrome", cl.Get(flags.Bin))
	})

	t.Run("use installed chrome option is preserved in launcher", func(t *testing.T) {
		l, err := NewLauncher(LauncherOptions{
			MaxBrowsers:        1,
			UseInstalledChrome: true,
		})
		require.NoError(t, err)
		require.True(t, l.opts.UseInstalledChrome)
	})
}
