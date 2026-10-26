package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestOverExplosionBudget(t *testing.T) {
	t.Parallel()
	c := &Crawler{
		crawlGraph:      graph.NewCrawlGraph(),
		explosionBudget: cartography.NewExplosionBudget(1, 3),
	}
	require.NoError(t, c.crawlGraph.AddPageState(types.PageState{UniqueID: "list", URL: "https://example.com/list"}))

	first := &types.PageState{UniqueID: "p1", OriginID: "list", URL: "https://example.com/p/1", SimHash: 0b11110000}
	require.False(t, c.overExplosionBudget(first))
	require.NoError(t, c.crawlGraph.AddPageState(*first))

	// A near-duplicate page at another URL is over budget.
	require.True(t, c.overExplosionBudget(&types.PageState{UniqueID: "p2", OriginID: "list", URL: "https://example.com/p/2", SimHash: 0b11110001}))
	// A revisit of an admitted state is not counted again.
	require.False(t, c.overExplosionBudget(first))
	// An in-place change of the page (an opened menu) is never dropped.
	require.False(t, c.overExplosionBudget(&types.PageState{UniqueID: "p1-menu", OriginID: "p1", URL: "https://example.com/p/1#menu", SimHash: 0b11110001}))

	off := &Crawler{crawlGraph: graph.NewCrawlGraph()}
	require.False(t, off.overExplosionBudget(&types.PageState{UniqueID: "x", URL: "https://example.com/x"}))
}
