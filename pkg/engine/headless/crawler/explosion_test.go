package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/stretchr/testify/require"
)

func TestExplosionBudgetGate(t *testing.T) {
	t.Parallel()
	c := &Crawler{
		explosionBudget: cartography.NewExplosionBudget(1, 3),
		options:         Options{},
	}
	require.True(t, c.explosionBudget.Allow(0b11110000))
	require.False(t, c.explosionBudget.Allow(0b11110001))
}
