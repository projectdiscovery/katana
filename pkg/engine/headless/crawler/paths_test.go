package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestPathsFromGraph(t *testing.T) {
	t.Parallel()
	c := &Crawler{crawlGraph: graph.NewCrawlGraph()}
	require.NoError(t, c.crawlGraph.AddPageState(types.PageState{UniqueID: emptyPageHash, URL: "about:blank"}))
	require.NoError(t, c.crawlGraph.AddPageState(types.PageState{
		UniqueID:         "home",
		OriginID:         emptyPageHash,
		URL:              "https://example.com/",
		NavigationAction: &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/", OriginID: emptyPageHash},
	}))

	paths, err := c.Paths()
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.Equal(t, "home", paths[0].TargetID)
	require.Len(t, paths[0].Steps, 1)

	empty := &Crawler{}
	paths, err = empty.Paths()
	require.NoError(t, err)
	require.Nil(t, paths)
}
