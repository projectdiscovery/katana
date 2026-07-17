package cartography

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestLinkIdentityDropsVolatileQuery(t *testing.T) {
	t.Parallel()
	id := &LinkIdentity{}
	id.Observe(FeaturesFromAction(&types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:     "a",
			ID:          "buy",
			CSSSelector: "a#buy",
			TextContent: "Buy",
			Attributes:  map[string]string{"href": "/product?id=1&token=aaa"},
		},
	}))
	id.Observe(FeaturesFromAction(&types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:     "a",
			ID:          "buy",
			CSSSelector: "a#buy",
			TextContent: "Buy",
			Attributes:  map[string]string{"href": "/product?id=2&token=bbb"},
		},
	}))

	core := id.Core()
	require.Equal(t, 2, id.Visits())
	require.Equal(t, "a", core.Tag)
	require.Equal(t, "buy", core.ID)
	require.Equal(t, "/product", core.PathOnly)
	// query key set changed (id/token values change identity of key set still same keys - wait, keys are id,token both times)
	// Path stayed; query keys same set [id, token] - should remain
	require.Equal(t, []string{"id", "token"}, core.QueryKeys)

	// Path changes → path dropped
	id.Observe(FeaturesFromAction(&types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:    "a",
			ID:         "buy",
			Attributes: map[string]string{"href": "/other?id=3&token=ccc"},
		},
	}))
	core = id.Core()
	require.Empty(t, core.PathOnly)
	require.True(t, id.Matches(FeaturesFromAction(&types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:    "a",
			ID:         "buy",
			Attributes: map[string]string{"href": "/whatever?id=9&token=zzz"},
		},
	})))
}
