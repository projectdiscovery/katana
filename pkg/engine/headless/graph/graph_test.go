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

func TestPathsFromUsesTraversedEdgeAction(t *testing.T) {
	t.Parallel()
	g := NewCrawlGraph()
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "root", URL: "about:blank"}))

	homeAction := &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/", OriginID: "root", Depth: 1}
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "home", OriginID: "root", URL: "https://example.com/", Depth: 1, NavigationAction: homeAction}))

	// "next" was first reached by a click on "home", but the entrypoint also
	// loads it directly. The shortest path uses the direct edge, so its step
	// must be the direct load, not the click that only works from "home".
	click := &types.Action{Type: types.ActionTypeLeftClick, OriginID: "home", Depth: 2, Element: &types.HTMLElement{TagName: "a", ID: "next"}}
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "next", OriginID: "home", URL: "https://example.com/next", Depth: 2, NavigationAction: click}))
	direct := &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/next", OriginID: "root", Depth: 1}
	require.NoError(t, g.AddEdge("root", "next", direct))

	// "page" has no NavigationAction; only the edge records how to reach it.
	require.NoError(t, g.AddPageState(types.PageState{UniqueID: "page", URL: "https://example.com/page"}))
	pageLoad := &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/page", OriginID: "root", Depth: 1}
	require.NoError(t, g.AddEdge("root", "page", pageLoad))

	paths, unreachable, err := g.PathsFrom("root")
	require.NoError(t, err)
	require.Empty(t, unreachable)
	byTarget := map[string]*types.Path{}
	for _, p := range paths {
		byTarget[p.TargetID] = p
	}
	require.Equal(t, []*types.Action{homeAction}, byTarget["home"].Steps)
	require.Equal(t, []*types.Action{direct}, byTarget["next"].Steps)
	require.Equal(t, []*types.Action{pageLoad}, byTarget["page"].Steps)

	raw, err := json.Marshal(byTarget["page"])
	require.NoError(t, err)
	require.Contains(t, string(raw), `"steps":[{`)
}
