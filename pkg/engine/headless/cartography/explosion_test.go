package cartography

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExplosionBudget(t *testing.T) {
	t.Parallel()
	b := NewExplosionBudget(1, 3)

	require.True(t, b.Allow(0b11110000))
	require.False(t, b.Allow(0b11110001)) // near-dupe, budget 1
	require.True(t, b.Allow(0b00001111))  // new cluster
	require.Equal(t, 2, b.ClusterCount())

	b2 := NewExplosionBudget(2, 3)
	require.True(t, b2.Allow(0b11110000))
	require.True(t, b2.Allow(0b11110001))
	require.False(t, b2.Allow(0b11110010))
}
