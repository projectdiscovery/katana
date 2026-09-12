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
	require.NoError(t, c.crawlGraph.AddPageState(types.PageState{UniqueID: "root", URL: "about:blank"}))
	require.NoError(t, c.crawlGraph.AddPageState(types.PageState{
		UniqueID:         "home",
		OriginID:         "root",
		URL:              "https://example.com/",
		NavigationAction: &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/", OriginID: "root"},
	}))

	paths, err := c.Paths()
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.Equal(t, "home", paths[0].TargetID)
	require.Equal(t, 1, paths[0].Len())

	empty := &Crawler{}
	paths, err = empty.Paths()
	require.NoError(t, err)
	require.Nil(t, paths)
}
