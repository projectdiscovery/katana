package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathLenAndClone(t *testing.T) {
	t.Parallel()
	require.Equal(t, 0, (*Path)(nil).Len())

	p := &Path{
		EntryID:  "root",
		TargetID: "leaf",
		Steps: []*Action{
			{Type: ActionTypeLoadURL, Input: "https://example.com"},
			{Type: ActionTypeLeftClick},
		},
	}
	require.Equal(t, 2, p.Len())

	c := p.Clone()
	require.Equal(t, p.EntryID, c.EntryID)
	require.Equal(t, p.TargetID, c.TargetID)
	require.Len(t, c.Steps, 2)
	c.Steps = append(c.Steps, &Action{Type: ActionTypeWait})
	require.Equal(t, 2, p.Len())
	require.Equal(t, 3, c.Len())
}

func TestPathMarshalEmptySteps(t *testing.T) {
	t.Parallel()
	p := &Path{EntryID: "a", TargetID: "b"}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"steps":[]`)
}
