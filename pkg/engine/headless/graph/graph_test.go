package graph

import (
	"encoding/json"
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestPathsFrom(t *testing.T) {
	t.Parallel()
	g := NewCrawlGraph()

	root := types.PageState{UniqueID: "root", URL: "about:blank", Depth: 0}
	require.NoError(t, g.AddPageState(root))

	homeAction := &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/", OriginID: "root", Depth: 1}
	require.NoError(t, g.AddPageState(types.PageState{
		UniqueID:         "home",
		OriginID:         "root",
		URL:              "https://example.com/",
		Depth:            1,
		NavigationAction: homeAction,
	}))

	click := &types.Action{Type: types.ActionTypeLeftClick, OriginID: "home", Depth: 2, Element: &types.HTMLElement{TagName: "a", ID: "next"}}
	require.NoError(t, g.AddPageState(types.PageState{
		UniqueID:         "next",
		OriginID:         "home",
		URL:              "https://example.com/next",
		Depth:            2,
		NavigationAction: click,
	}))

	other := &types.Action{Type: types.ActionTypeLeftClick, OriginID: "home", Depth: 2, Element: &types.HTMLElement{TagName: "a", ID: "other"}}
	require.NoError(t, g.AddPageState(types.PageState{
		UniqueID:         "other",
		OriginID:         "home",
		URL:              "https://example.com/other",
		Depth:            2,
		NavigationAction: other,
	}))

	// A state without an edge from the entrypoint cannot be replayed.
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "orphan", URL: "https://example.com/orphan"}))

	paths, unreachable, err := g.PathsFrom("root")
	require.NoError(t, err)
	require.Equal(t, []string{"orphan"}, unreachable)
	require.Len(t, paths, 3)

	byTarget := map[string]*types.Path{}
	for _, p := range paths {
		require.Equal(t, "root", p.EntryID)
		byTarget[p.TargetID] = p
	}
	require.Equal(t, []*types.Action{homeAction}, byTarget["home"].Steps)
	require.Equal(t, []*types.Action{homeAction, click}, byTarget["next"].Steps)
	require.Equal(t, []*types.Action{homeAction, other}, byTarget["other"].Steps)
	require.Equal(t, "https://example.com/next", byTarget["next"].TargetURL)

	_, _, err = g.PathsFrom("missing")
	require.Error(t, err)
}

func TestPathsFromEmptyStepsMarshalAsArray(t *testing.T) {
	t.Parallel()
	g := NewCrawlGraph()
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "root", URL: "about:blank"}))
	// A state reached without a recorded action still gets a path, with no steps.
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "page", URL: "https://example.com/"}))
	require.NoError(t, g.AddEdge("root", "page", &types.Action{Type: types.ActionTypeLoadURL}))

	paths, _, err := g.PathsFrom("root")
	require.NoError(t, err)
	require.Len(t, paths, 1)
	raw, err := json.Marshal(paths[0])
	require.NoError(t, err)
	require.Contains(t, string(raw), `"steps":[]`)
}
