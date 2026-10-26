package cartography

import (
	"net/url"
	"sort"
	"strings"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// LinkKey returns the identity of the door an action opens, as found on the
// page at sourceURL. Doors with the same key are near-duplicates: the same
// element of the same page, whose target differs only in query values or
// visible text, such as session tokens, product IDs or counters.
//
// The key holds only features that do not churn: the source page without its
// query, the action type and tag, the element anchor (ID, else CSS path, else
// XPath), and the target path with its sorted query keys. It is an exact key,
// so it never widens over time and never joins elements that differ in anchor
// or target path. An empty key means the action has no anchor to track.
func LinkKey(sourceURL string, a *types.Action) string {
	if a == nil {
		return ""
	}
	var target string
	var anchor string
	switch {
	case a.Element != nil:
		anchor = firstNonEmpty(a.Element.ID, a.Element.CSSSelector, a.Element.XPath)
		if anchor == "" {
			return ""
		}
		if a.Element.ID != "" {
			anchor = "#" + anchor
		}
		target = targetFeatures(a.Element.Attributes["href"])
		anchor = strings.ToLower(strings.TrimSpace(a.Element.TagName)) + "|" + anchor
	case a.Type == types.ActionTypeLoadURL && a.Input != "":
		target = targetFeatures(a.Input)
	default:
		return ""
	}
	return strings.Join([]string{pageFeatures(sourceURL), string(a.Type), anchor, target}, "\x00")
}

// pageFeatures returns the source page without query and fragment, so a page
// revisited with a new token in its own URL keeps its doors.
func pageFeatures(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Scheme + "://" + u.Host + u.Path
}

// targetFeatures returns the path and the sorted query keys of a link target.
// Query values are left out: they are what churns.
func targetFeatures(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	keys := make([]string, 0, len(u.Query()))
	for k := range u.Query() {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return u.Host + u.Path + "?" + strings.Join(keys, "&")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
