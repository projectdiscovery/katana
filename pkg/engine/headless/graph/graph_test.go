package graph

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestPathFromRoot(t *testing.T) {
	t.Parallel()
	g := NewCrawlGraph()

	root := types.PageState{UniqueID: "root", URL: "about:blank", Depth: 0}
	require.NoError(t, g.AddPageState(root))

	homeAction := &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/", OriginID: "root", Depth: 1}
	home := types.PageState{
		UniqueID:         "home",
		OriginID:         "root",
		URL:              "https://example.com/",
		Depth:            1,
		NavigationAction: homeAction,
	}
	require.NoError(t, g.AddPageState(home))

	click := &types.Action{Type: types.ActionTypeLeftClick, OriginID: "home", Depth: 2, Element: &types.HTMLElement{TagName: "a", ID: "next"}}
	next := types.PageState{
		UniqueID:         "next",
		OriginID:         "home",
		URL:              "https://example.com/next",
		Depth:            2,
		NavigationAction: click,
	}
	require.NoError(t, g.AddPageState(next))

	rootID, err := g.RootID()
	require.NoError(t, err)
	require.Equal(t, "root", rootID)

	p, err := g.PathFromRoot("next")
	require.NoError(t, err)
	require.Equal(t, "root", p.EntryID)
	require.Equal(t, "next", p.TargetID)
	require.Equal(t, "https://example.com/next", p.TargetURL)
	require.Len(t, p.Steps, 2)
	require.Equal(t, types.ActionTypeLoadURL, p.Steps[0].Type)
	require.Equal(t, types.ActionTypeLeftClick, p.Steps[1].Type)

	self, err := g.PathFromRoot("root")
	require.NoError(t, err)
	require.Equal(t, 0, self.Len())

	all, err := g.AllPathsFromRoot()
	require.NoError(t, err)
	require.Len(t, all, 2)
}
