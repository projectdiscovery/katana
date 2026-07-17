package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyCrawlStrategy(t *testing.T) {
	t.Parallel()
	opts := &Options{CrawlStrategy: "fast", MaxDepth: 99}
	require.NoError(t, ApplyCrawlStrategy(opts))
	require.Equal(t, 2, opts.MaxDepth)
	require.Equal(t, "domcontentloaded", opts.PageLoadStrategy)
	require.True(t, opts.FilterSimilar)
	require.Equal(t, 0, opts.RewalkSample)

	opts = &Options{CrawlStrategy: "thorough"}
	require.NoError(t, ApplyCrawlStrategy(opts))
	require.Equal(t, 5, opts.MaxDepth)
	require.Equal(t, "heuristic", opts.PageLoadStrategy)
	require.Equal(t, 3, opts.RewalkSample)

	require.NoError(t, ApplyCrawlStrategy(&Options{}))
	require.Error(t, ApplyCrawlStrategy(&Options{CrawlStrategy: "nope"}))
}
