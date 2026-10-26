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

	// A page that links to the next one by query is another page: a calendar
	// does not walk every month through the exemption.
	require.NoError(t, c.crawlGraph.AddPageState(types.PageState{UniqueID: "m1", OriginID: "list", URL: "https://example.com/calendar?month=1", SimHash: 0b00001111}))
	require.True(t, c.overExplosionBudget(&types.PageState{UniqueID: "m2", OriginID: "m1", URL: "https://example.com/calendar?month=2", SimHash: 0b11110001}))

	// A rejected state stays in the graph, but a later visit to it is still
	// rejected, and so is an in-place change of it.
	rejected := &types.PageState{UniqueID: "p3", OriginID: "list", URL: "https://example.com/p/3", SimHash: 0b11110010}
	require.True(t, c.overExplosionBudget(rejected))
	require.NoError(t, c.crawlGraph.AddPageState(*rejected))
	require.True(t, c.overExplosionBudget(rejected), "a revisit must not admit a rejected state")
	require.True(t, c.overExplosionBudget(&types.PageState{UniqueID: "p3-menu", OriginID: "p3", URL: "https://example.com/p/3#menu", SimHash: 0b11110011}))
	require.False(t, c.overExplosionBudget(first), "an admitted state stays admitted")

	off := &Crawler{crawlGraph: graph.NewCrawlGraph()}
	require.False(t, off.overExplosionBudget(&types.PageState{UniqueID: "x", URL: "https://example.com/x"}))
}

func TestSamePage(t *testing.T) {
	t.Parallel()
	require.True(t, samePage("https://example.com/a?x=1", "https://example.com/a?x=1#menu"))
	require.True(t, samePage("https://example.com/a?x=1&y=2", "https://example.com/a?y=2&x=1"))
	require.False(t, samePage("https://example.com/a?x=1", "https://example.com/a?x=2"))
	require.False(t, samePage("https://example.com/a", "https://example.com/b"))
}
