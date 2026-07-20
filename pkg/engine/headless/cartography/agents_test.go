package cartography

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentPoolDefaults(t *testing.T) {
	t.Parallel()
	p := NewAgentPool()
	require.Equal(t, 1, p.Len())
	a, ok := p.Get("anon")
	require.True(t, ok)
	require.Equal(t, "anon", a.Role)

	p2 := NewAgentPool(
		&Agent{Role: "user", Credentials: "u:p", DataDir: "/tmp/u"},
		&Agent{ID: "admin", Role: "admin", Credentials: "a:p"},
	)
	require.Equal(t, 2, p2.Len())
	first := p2.All()[0]
	require.Equal(t, "agent-0", first.ID)
	require.Equal(t, "user", first.Role)
}

func TestDetectSingleLoginConflict(t *testing.T) {
	t.Parallel()
	require.False(t, DetectSingleLoginConflict([]*Agent{
		{ID: "a", Credentials: "u:p"},
		{ID: "b", Credentials: "u2:p"},
	}))
	require.True(t, DetectSingleLoginConflict([]*Agent{
		{ID: "a", Credentials: "u:p"},
		{ID: "b", Credentials: "u:p"},
	}))
	require.False(t, DetectSingleLoginConflict([]*Agent{
		{ID: "a"},
		{ID: "b", Credentials: "u:p"},
	}))
}
