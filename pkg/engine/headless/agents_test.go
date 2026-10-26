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

func TestBuildAgentsSameUserOtherPassword(t *testing.T) {
	t.Parallel()
	got := buildAgents(&types.Options{HeadlessAgents: []string{"bob:old", "bob:new", "anon", "bob:third"}})
	require.Equal(t, []agent{
		{label: "bob", credentials: "bob:old"},
		{label: "bob#2", credentials: "bob:new"},
		{label: "anon"},
		{label: "bob#3", credentials: "bob:third"},
	}, got)
	scopes := map[string]bool{}
	for _, a := range got {
		scope := a.dedupScope(len(got))
		require.NotContains(t, scope, "old")
		require.NotContains(t, scope, "new")
		scopes[scope] = true
	}
	require.Len(t, scopes, len(got), "each identity needs its own dedupe scope")
}

func TestBuildAgentsSessionHeader(t *testing.T) {
	t.Parallel()
	for _, header := range []string{"Cookie: sid=1", "cookie:sid=1", "Authorization: Bearer x"} {
		got := buildAgents(&types.Options{CustomHeaders: []string{header}, HeadlessAgents: []string{"anon", "bob:pw"}})
		require.Equal(t, []agent{{label: "anon"}}, got, header)
	}
	got := buildAgents(&types.Options{CustomHeaders: []string{"X-Test: 1"}, HeadlessAgents: []string{"anon", "bob:pw"}})
	require.Len(t, got, 2, "other headers keep every identity")
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
