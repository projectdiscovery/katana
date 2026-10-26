package cartography

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func click(id, css, text, href string) *types.Action {
	return &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:     "a",
			ID:          id,
			CSSSelector: css,
			TextContent: text,
			Attributes:  map[string]string{"href": href},
		},
	}
}

func TestLinkKeyIgnoresChurn(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/list"
	// Query values, visible text and the source page query churn between visits.
	a := LinkKey(page+"?session=1", click("buy", "a#buy", "Buy (1)", "/product?id=1&token=aaa"))
	b := LinkKey(page+"?session=2", click("buy", "a#buy", "Buy (2)", "/product?token=bbb&id=2"))
	require.NotEmpty(t, a)
	require.Equal(t, a, b)
}

func TestLinkKeySeparatesDoors(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/list"
	base := LinkKey(page, click("", "div > a:nth-child(1)", "Edit", "/edit?id=1"))

	// Same text, other element: a second Edit button is another door.
	require.NotEqual(t, base, LinkKey(page, click("", "div > a:nth-child(2)", "Edit", "/edit?id=2")))
	// Same element, other target path.
	require.NotEqual(t, base, LinkKey(page, click("", "div > a:nth-child(1)", "Edit", "/delete?id=1")))
	// Same element, other query keys.
	require.NotEqual(t, base, LinkKey(page, click("", "div > a:nth-child(1)", "Edit", "/edit?id=1&mode=full")))
	// Same element on another page.
	require.NotEqual(t, base, LinkKey("https://example.com/other", click("", "div > a:nth-child(1)", "Edit", "/edit?id=1")))
}

func TestLinkKeyWithoutAnchor(t *testing.T) {
	t.Parallel()
	require.Empty(t, LinkKey("https://example.com/", nil))
	require.Empty(t, LinkKey("https://example.com/", click("", "", "Edit", "/edit")))
	require.Empty(t, LinkKey("https://example.com/", &types.Action{Type: types.ActionTypeFillForm}))
	load := &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/p?id=1"}
	require.Equal(t, LinkKey("https://example.com/", load), LinkKey("https://example.com/", &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/p?id=2"}))
}

func TestLinkKeyHashRoutes(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/app"
	load := func(u string) *types.Action { return &types.Action{Type: types.ActionTypeLoadURL, Input: u} }

	// Distinct hash routes are distinct doors.
	require.NotEqual(t, LinkKey(page, load(page+"#/users")), LinkKey(page, load(page+"#/settings")))
	require.NotEqual(t, LinkKey(page, load(page+"#!/users")), LinkKey(page, load(page+"#!/settings")))
	require.NotEqual(t, LinkKey(page, click("", "nav > a", "Go", "#/users")), LinkKey(page, click("", "nav > a", "Go", "#/settings")))
	// The same element on two routes is two doors.
	require.NotEqual(t, LinkKey(page+"#/users", click("", "main > a", "Edit", "/edit")), LinkKey(page+"#/settings", click("", "main > a", "Edit", "/edit")))

	// Route query values, in-page anchors and token fragments still churn.
	require.Equal(t, LinkKey(page, load(page+"#/item?id=1")), LinkKey(page, load(page+"#/item?id=2")))
	require.Equal(t, LinkKey(page, load(page+"#top")), LinkKey(page, load(page+"#comments")))
	require.Equal(t, LinkKey(page, load(page+"#access_token=a")), LinkKey(page, load(page+"#access_token=b")))
	require.Equal(t, LinkKey(page+"#top", click("", "main > a", "Edit", "/edit")), LinkKey(page+"#bottom", click("", "main > a", "Edit", "/edit")))
}
