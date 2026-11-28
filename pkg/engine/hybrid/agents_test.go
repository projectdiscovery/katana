package hybrid

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestHybridBrowserAgents(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, hybridBrowserAgents(nil))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{BrowserAgents: 0}))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{Concurrency: 10}))
	require.Equal(t, 4, hybridBrowserAgents(&types.Options{BrowserAgents: 4}))
	require.Equal(t, maxHybridBrowserAgents, hybridBrowserAgents(&types.Options{BrowserAgents: 100}))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{BrowserAgents: 4, ChromeWSUrl: "ws://127.0.0.1:9222"}))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{BrowserAgents: 4, ChromeDataDir: "/tmp/chrome"}))
}
