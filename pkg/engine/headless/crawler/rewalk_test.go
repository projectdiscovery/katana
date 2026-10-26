package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestSamplePaths(t *testing.T) {
	t.Parallel()
	paths := []*types.Path{{TargetID: "a"}, {TargetID: "b"}, {TargetID: "c"}, {TargetID: "d"}}
	require.Nil(t, samplePaths(paths, 0))
	require.ElementsMatch(t, paths, samplePaths(paths, 10))

	// A sample is a subset without repeats, and over many draws every path
	// shows up, not only the first n.
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		got := samplePaths(paths, 2)
		require.Len(t, got, 2)
		require.NotEqual(t, got[0].TargetID, got[1].TargetID)
		for _, p := range got {
			seen[p.TargetID] = true
		}
	}
	require.Len(t, seen, 4)
	require.Equal(t, "a", paths[0].TargetID, "the caller's slice must not be reordered")
}

func TestCrawledOrigins(t *testing.T) {
	t.Parallel()
	c := &Crawler{crawlGraph: graph.NewCrawlGraph()}
	for i, u := range []string{"about:blank", "https://example.com/a", "https://example.com/b?x=1", "http://api.example.com:8080/", "data:text/html,x"} {
		require.NoError(t, c.crawlGraph.AddPageState(types.PageState{UniqueID: string(rune('a' + i)), URL: u}))
	}
	require.Equal(t, []string{"http://api.example.com:8080", "https://example.com"}, c.crawledOrigins())
}
