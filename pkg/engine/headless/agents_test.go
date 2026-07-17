package headless

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestBuildAgentPool(t *testing.T) {
	t.Parallel()
	p := buildAgentPool(&types.Options{HeadlessAgents: 2})
	require.Equal(t, 2, p.Len())

	p = buildAgentPool(&types.Options{ChromeDataDir: "/tmp/x", HeadlessAgents: 5})
	require.Equal(t, 1, p.Len())
	a, ok := p.Get("default")
	require.True(t, ok)
	require.Equal(t, "/tmp/x", a.DataDir)
}

func TestSplitCredentials(t *testing.T) {
	t.Parallel()
	u, p := splitCredentials("alice:s3cret")
	require.Equal(t, "alice", u)
	require.Equal(t, "s3cret", p)
}
