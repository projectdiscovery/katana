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

func TestLinkKeyGeneratedIDs(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/list"
	at := func(id, css, xpath string) *types.Action {
		a := click(id, css, "Buy", "/product?id=1")
		a.Element.XPath = xpath
		return a
	}
	const xpath = "/html/body/div/a[2]"
	// A generated ID, on the element or on an ancestor, is not an anchor: the
	// key falls back to the XPath, which stays the same between visits.
	for _, pair := range [][2]*types.Action{
		{at(":r1:", `A#\:r1\:`, xpath), at(":r7:", `A#\:r7\:`, xpath)},
		{at("ember123", "A#ember123", xpath), at("ember456", "A#ember456", xpath)},
		{at("buy-9f8e7d6c", "A#buy-9f8e7d6c", xpath), at("buy-1a2b3c4d", "A#buy-1a2b3c4d", xpath)},
		{at("", "DIV#item-123 > A", xpath), at("", "DIV#item-456 > A", xpath)},
	} {
		a, b := LinkKey(page, pair[0]), LinkKey(page, pair[1])
		require.NotEmpty(t, a)
		require.Equal(t, a, b, "ids %q and %q", pair[0].Element.ID, pair[1].Element.ID)
	}
	// A stable ID is still the anchor, so a layout shift does not split it.
	require.Equal(t, LinkKey(page, at("buy", "A#buy", xpath)), LinkKey(page, at("buy", "A#buy", "/html/body/div[2]/a[2]")))
	// PositionalLinkKey ignores any ID.
	require.Equal(t, PositionalLinkKey(page, at("buy-abc", "A#buy-abc", xpath)), PositionalLinkKey(page, at("buy-xyz", "A#buy-xyz", xpath)))
	require.Equal(t, LinkPosition(page, at("buy-abc", "A#buy-abc", xpath)), LinkPosition(page, at("buy-xyz", "A#buy-xyz", xpath)))
	require.Empty(t, LinkPosition(page, at("buy", "A#buy", "")))
}

func TestGeneratedID(t *testing.T) {
	t.Parallel()
	for _, id := range []string{":r1:", "ember123", "item-123", "el_4821", "btn-9f8e7d6c", "550e8400-e29b-41d4-a716-446655440000"} {
		require.True(t, GeneratedID(id), id)
	}
	for _, id := range []string{"", "buy", "main-nav", "h2", "col-md-6", "login_form"} {
		require.False(t, GeneratedID(id), id)
	}
}

func TestLinkKeyPathShape(t *testing.T) {
	t.Parallel()
	const page = "https://example.com/list"
	link := func(href string) string { return LinkKey(page, click("", "ul > li > a", "Item", href)) }

	// Variable segments of the target path share one shape.
	require.Equal(t, link("/product/123"), link("/product/456"))
	require.Equal(t, link("/user/550e8400-e29b-41d4-a716-446655440000/edit"), link("/user/6ba7b810-9dad-11d1-80b4-00c04fd430c8/edit"))
	require.Equal(t, link("/blob/9f8e7d6c5b4a"), link("/blob/0a1b2c3d4e5f"))
	require.Equal(t, link("/s/a1B2c3D4e5F6g7H8"), link("/s/Z9y8X7w6V5u4T3s2"))
	require.Equal(t, LinkKey(page, &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/#/item/1"}), LinkKey(page, &types.Action{Type: types.ActionTypeLoadURL, Input: "https://example.com/#/item/2"}))

	// Routes that differ stay apart.
	require.NotEqual(t, link("/product/123"), link("/user/123"))
	require.NotEqual(t, link("/product/123"), link("/product/123/reviews"))
	require.NotEqual(t, link("/api/v1/users"), link("/api/v2/users"))
	require.NotEqual(t, link("/docs/install"), link("/docs/usage"))

	// A relative and an absolute href to the same place agree; a fragment-only
	// href does not take the query keys of the source page.
	require.Equal(t, link("/product?id=1"), link("https://example.com/product?id=2"))
	require.Equal(t, LinkKey(page+"?tab=1", click("", "a", "Top", "#top")), LinkKey(page, click("", "a", "Top", "#top")))
	require.NotEqual(t, link("/product?id=1"), link("https://other.example/product?id=1"))
}

func TestPathShape(t *testing.T) {
	t.Parallel()
	require.Equal(t, "/product/{v}/reviews", PathShape("/product/123/reviews"))
	require.Equal(t, "/api/v1/users", PathShape("/api/v1/users"))
	require.Equal(t, "/{v}/{v}", PathShape("/2024/550e8400-e29b-41d4-a716-446655440000"))
	require.Equal(t, "/deadbeef", PathShape("/deadbeef"), "a hex word without digits is a name")
	require.Empty(t, PathShape(""))
}
