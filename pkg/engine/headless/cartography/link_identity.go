package cartography

import (
	"net/url"
	"sort"
	"strings"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// LinkKey returns the identity of the door an action opens, as found on the
// page at sourceURL. Doors with the same key are near-duplicates: the same
// element of the same page, whose target differs only in query values,
// variable path segments or visible text, such as session tokens, product IDs
// or counters.
//
// The key holds only features that do not churn: the source page without its
// query, the action type and tag, the element anchor (ID, else CSS path, else
// XPath), and the shape of the target path with its sorted query keys. An ID
// that looks generated is not an anchor, and neither is a CSS path that holds
// one. A hash route in the fragment (#/path) counts as path; other fragments
// are left out. It is an exact key, so it never widens over time and never
// joins elements that differ in anchor or target route. An empty key means the
// action has no anchor to track.
func LinkKey(sourceURL string, a *types.Action) string {
	return linkKey(sourceURL, a, false)
}

// PositionalLinkKey is LinkKey with the element ID and CSS path left out, so
// the element is anchored on its XPath. Use it when the ID of an element
// changes between visits.
func PositionalLinkKey(sourceURL string, a *types.Action) string {
	return linkKey(sourceURL, a, true)
}

// LinkPosition returns the place of an element action: its source page, tag,
// XPath and target shape. The same link seen on two visits has the same
// position even when its ID changes. It is empty when the action has no
// element with an XPath.
func LinkPosition(sourceURL string, a *types.Action) string {
	if a == nil || a.Element == nil || strings.TrimSpace(a.Element.XPath) == "" {
		return ""
	}
	return strings.Join([]string{
		pageFeatures(sourceURL),
		string(a.Type),
		strings.ToLower(strings.TrimSpace(a.Element.TagName)),
		strings.TrimSpace(a.Element.XPath),
		targetFeatures(sourceURL, a.Element.Attributes["href"]),
	}, "\x00")
}

func linkKey(sourceURL string, a *types.Action, positional bool) string {
	if a == nil {
		return ""
	}
	var target string
	var anchor string
	switch {
	case a.Element != nil:
		id := strings.TrimSpace(a.Element.ID)
		css := a.Element.CSSSelector
		if positional || GeneratedID(id) {
			id = ""
			if a.Element.ID != "" {
				// The CSS path of an element with an ID ends on that ID.
				css = ""
			}
		}
		if positional || cssHasGeneratedID(css) {
			css = ""
		}
		anchor = firstNonEmpty(id, css, a.Element.XPath)
		if anchor == "" {
			return ""
		}
		if id != "" {
			anchor = "#" + anchor
		}
		target = targetFeatures(sourceURL, a.Element.Attributes["href"])
		anchor = strings.ToLower(strings.TrimSpace(a.Element.TagName)) + "|" + anchor
	case a.Type == types.ActionTypeLoadURL && a.Input != "":
		target = targetFeatures(sourceURL, a.Input)
	default:
		return ""
	}
	return strings.Join([]string{pageFeatures(sourceURL), string(a.Type), anchor, target}, "\x00")
}

// pageFeatures returns the source page without query, so a page revisited
// with a new token in its own URL keeps its doors. Only a hash route of the
// fragment is kept.
func pageFeatures(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Scheme + "://" + u.Host + u.Path + routeFeatures(u.Fragment)
}

// targetFeatures returns the path shape and the sorted query keys of a link
// target, with its scheme and host, plus its hash route if it has one. A
// relative target is resolved
// against the source page first, so a relative and an absolute href to the
// same place agree. Query values and variable path segments are left out:
// they are what churns.
func targetFeatures(sourceURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	// A fragment-only target stays as it is, so it does not take the query
	// keys of the source page.
	if !u.IsAbs() && !strings.HasPrefix(raw, "#") {
		if base, err := url.Parse(sourceURL); err == nil && base.IsAbs() {
			u = base.ResolveReference(u)
		}
	}
	origin := u.Host
	if u.Scheme != "" {
		// http and https to the same path are separate doors.
		origin = strings.ToLower(u.Scheme) + "://" + u.Host
	}
	return origin + PathShape(u.Path) + "?" + queryKeys(u.Query()) + routeFeatures(u.Fragment)
}

// routeFeatures returns the hash route of a fragment ("#/users" or
// "#!/users"), with the sorted keys of its own query. SPAs route on it, so
// distinct routes are distinct pages. Any other fragment, such as an in-page
// anchor or a token, is left out.
func routeFeatures(fragment string) string {
	route := strings.TrimPrefix(fragment, "!")
	if !strings.HasPrefix(route, "/") {
		return ""
	}
	route, rawQuery, hasQuery := strings.Cut(route, "?")
	route = PathShape(route)
	if !hasQuery {
		return "#" + route
	}
	query, _ := url.ParseQuery(rawQuery)
	return "#" + route + "?" + queryKeys(query)
}

func queryKeys(query url.Values) string {
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, "&")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
