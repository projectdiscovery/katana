package crawler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReserveActionHonorsInteractionMaxActions(t *testing.T) {
	e := &interactiveExplorer{
		crawler: &Crawler{options: Options{InteractionMaxActions: 2}},
	}
	require.True(t, e.reserveAction())
	require.Equal(t, 1, e.attempts)
	require.True(t, e.reserveAction())
	require.Equal(t, 2, e.attempts)
	require.False(t, e.reserveAction())
	require.Equal(t, 2, e.attempts)
}

func TestReserveActionUnlimitedWhenMaxZero(t *testing.T) {
	e := &interactiveExplorer{
		crawler: &Crawler{options: Options{InteractionMaxActions: 0}},
	}
	for i := 0; i < 5; i++ {
		require.True(t, e.reserveAction())
	}
	require.Equal(t, 5, e.attempts)
}

func TestReserveActionCountsRestoreAndExploreTogether(t *testing.T) {
	// Restore path of length 2 plus one exploratory click must not exceed max=3.
	e := &interactiveExplorer{
		crawler: &Crawler{options: Options{InteractionMaxActions: 3}},
	}
	// Simulate restore clicks.
	require.True(t, e.reserveAction())
	require.True(t, e.reserveAction())
	// Exploratory click still fits.
	require.True(t, e.reserveAction())
	require.Equal(t, 3, e.attempts)
	// Next restore or explore would exceed.
	require.False(t, e.reserveAction())
	require.Equal(t, 3, e.attempts)
}
