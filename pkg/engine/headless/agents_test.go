package headless

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestBuildAgents(t *testing.T) {
	t.Parallel()

	require.Equal(t, []agent{{label: "anon"}}, buildAgents(&types.Options{}))
	require.Equal(t, []agent{{label: "bob", credentials: "bob:pw"}}, buildAgents(&types.Options{AuthCredentials: "bob:pw"}))

	got := buildAgents(&types.Options{
		AuthCredentials: "ignored:pw",
		HeadlessAgents:  []string{"anon", "alice:a:b", "bob:pw", "bob:pw", " "},
	})
	require.Equal(t, []agent{
		{label: "anon"},
		{label: "alice", credentials: "alice:a:b"},
		{label: "bob", credentials: "bob:pw"},
	}, got, "each identity uses its own credentials, duplicates are dropped")

	shared := buildAgents(&types.Options{ChromeWSUrl: "ws://127.0.0.1:9222", HeadlessAgents: []string{"anon", "bob:pw"}})
	require.Len(t, shared, 1)
}

func TestAgentDedupScope(t *testing.T) {
	t.Parallel()
	a := agent{label: "bob"}
	require.Empty(t, a.dedupScope(1))
	require.NotEqual(t, a.dedupScope(2), agent{label: "anon"}.dedupScope(2))
}

func TestSplitCredentials(t *testing.T) {
	t.Parallel()
	user, pass := splitCredentials("bob:p:w")
	require.Equal(t, "bob", user)
	require.Equal(t, "p:w", pass)
}
