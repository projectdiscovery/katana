package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func buyLink(href string) *types.Action {
	return &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:    "a",
			ID:         "buy",
			Attributes: map[string]string{"href": href},
		},
	}
}

func TestAcceptLinkIdentityBudget(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/"
	c := &Crawler{options: Options{LinkIdentityBudget: 1}}
	require.True(t, c.acceptLinkIdentity(buyLink("/p?id=1&token=a"), page))
	require.False(t, c.acceptLinkIdentity(buyLink("/p?id=2&token=b"), page))
	require.True(t, c.acceptLinkIdentity(buyLink("/p?id=2&token=b"), "https://example.com/other"), "the budget is per source page")

	two := &Crawler{options: Options{LinkIdentityBudget: 2}}
	require.True(t, two.acceptLinkIdentity(buyLink("/p?id=1"), page))
	require.True(t, two.acceptLinkIdentity(buyLink("/p?id=2"), page))
	require.False(t, two.acceptLinkIdentity(buyLink("/p?id=3"), page))

	for _, budget := range []int{0, -1} {
		off := &Crawler{options: Options{LinkIdentityBudget: budget}}
		require.True(t, off.acceptLinkIdentity(buyLink("/p?id=1"), page))
		require.True(t, off.acceptLinkIdentity(buyLink("/p?id=2"), page), "budget %d must disable the check", budget)
	}
}

func TestAcceptLinkIdentityChangingID(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/"
	// The ID does not look generated, but it changes on every visit while the
	// link stays at the same place with the same target shape.
	visit := func(id, href string) *types.Action {
		return &types.Action{
			Type: types.ActionTypeLeftClick,
			Element: &types.HTMLElement{
				TagName:     "a",
				ID:          id,
				CSSSelector: "A#" + id,
				XPath:       "/html/body/main/a",
				Attributes:  map[string]string{"href": href},
			},
		}
	}
	c := &Crawler{options: Options{LinkIdentityBudget: 1}}
	require.True(t, c.acceptLinkIdentity(visit("buy-alpha", "/p?id=1"), page))
	require.False(t, c.acceptLinkIdentity(visit("buy-bravo", "/p?id=2"), page), "a changed ID must not get a fresh budget")
	require.False(t, c.acceptLinkIdentity(visit("buy-charlie", "/p?id=3"), page))
	require.False(t, c.acceptLinkIdentity(visit("buy-alpha", "/p?id=4"), page))
	require.True(t, c.acceptLinkIdentity(visit("buy-delta", "/cart"), page), "another target route is another door")

	// A stable ID keeps its own key and budget.
	two := &Crawler{options: Options{LinkIdentityBudget: 2}}
	require.True(t, two.acceptLinkIdentity(visit("buy", "/p?id=1"), page))
	require.True(t, two.acceptLinkIdentity(visit("buy", "/p?id=2"), page))
	require.False(t, two.acceptLinkIdentity(visit("buy", "/p?id=3"), page))
}

func TestAcceptLinkIdentityPathIDs(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/"
	c := &Crawler{options: Options{LinkIdentityBudget: 2}}
	require.True(t, c.acceptLinkIdentity(buyLink("/product/1"), page))
	require.True(t, c.acceptLinkIdentity(buyLink("/product/2"), page))
	require.False(t, c.acceptLinkIdentity(buyLink("/product/3"), page), "numeric path IDs share one budget")
	require.True(t, c.acceptLinkIdentity(buyLink("/user/3"), page), "another route keeps its own budget")
}
