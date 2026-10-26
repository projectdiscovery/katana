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
