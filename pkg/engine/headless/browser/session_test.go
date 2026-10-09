package browser

import (
	"context"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/stretchr/testify/require"
)

func TestClearSessionNilPage(t *testing.T) {
	t.Parallel()
	require.NoError(t, ClearSession(context.Background(), nil))
}

func TestClearSessionOnBlankPage(t *testing.T) {
	path, found := launcher.LookPath()
	if !found || path == "" {
		t.Skip("chrome/chromium not found")
	}
	controlURL, err := launcher.New().Bin(path).Leakless(true).Launch()
	if err != nil {
		t.Skipf("could not launch browser: %v", err)
	}
	b := rod.New().ControlURL(controlURL).MustConnect()
	t.Cleanup(func() { _ = b.Close() })

	page := b.MustPage("about:blank")
	t.Cleanup(func() { _ = page.Close() })

	require.NoError(t, ClearSession(context.Background(), page))
}
