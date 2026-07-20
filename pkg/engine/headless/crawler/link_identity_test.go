package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestAcceptLinkIdentityBudget(t *testing.T) {
	t.Parallel()
	c := &Crawler{options: Options{LinkIdentityBudget: 1}}
	nav := &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:    "a",
			ID:         "buy",
			Attributes: map[string]string{"href": "/p?id=1&token=a"},
		},
	}
	require.True(t, c.acceptLinkIdentity(nav))
	nav2 := &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:    "a",
			ID:         "buy",
			Attributes: map[string]string{"href": "/p?id=2&token=b"},
		},
	}
	require.False(t, c.acceptLinkIdentity(nav2))

	disabled := &Crawler{options: Options{LinkIdentityBudget: -1}}
	require.True(t, disabled.acceptLinkIdentity(nav2))
}
