package cartography

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStrategy(t *testing.T) {
	t.Parallel()
	s, err := ParseStrategy("")
	require.NoError(t, err)
	require.Equal(t, StrategyBalanced, s)

	s, err = ParseStrategy("thorough")
	require.NoError(t, err)
	require.Equal(t, StrategyThorough, s)

	_, err = ParseStrategy("nope")
	require.Error(t, err)
}

func TestConfigForOrdering(t *testing.T) {
	t.Parallel()
	fast := ConfigFor(StrategyFast)
	balanced := ConfigFor(StrategyBalanced)
	thorough := ConfigFor(StrategyThorough)

	require.Less(t, fast.MaxDepth, balanced.MaxDepth)
	require.Less(t, balanced.MaxDepth, thorough.MaxDepth)
	require.Less(t, fast.RewalkSample, thorough.RewalkSample)
	require.Equal(t, "domcontentloaded", fast.PageLoadStrategy)
	require.Equal(t, "heuristic", thorough.PageLoadStrategy)
	require.True(t, fast.FilterSimilarURLs)
}
