package crawler

import (
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// linkPosition is what was first seen at a cartography.LinkPosition.
type linkPosition struct {
	id, css, key string
	// volatile is set once the ID or CSS path at the position changed: the
	// position is then keyed on its XPath.
	volatile bool
}

// acceptLinkIdentity reports whether nav, found on the page at sourceURL, may
// be enqueued. With LinkIdentityBudget > 0, at most that many near-duplicate
// doors (same cartography.LinkKey) are enqueued. Zero or less disables it.
func (c *Crawler) acceptLinkIdentity(nav *types.Action, sourceURL string) bool {
	budget := c.options.LinkIdentityBudget
	if budget <= 0 {
		return true
	}
	if c.linkIdentities == nil {
		c.linkIdentities = make(map[string]int)
	}
	key := c.linkKey(nav, sourceURL)
	if key == "" {
		return true
	}
	if c.linkIdentities[key] >= budget {
		return false
	}
	c.linkIdentities[key]++
	return true
}

// linkKey returns the cartography.LinkKey of nav. When the ID or CSS path of
// the element at the same position changed since an earlier visit, they are
// not stable, so it returns the cartography.PositionalLinkKey instead. The
// count of the first key carries over, so the change costs no extra budget.
func (c *Crawler) linkKey(nav *types.Action, sourceURL string) string {
	key := cartography.LinkKey(sourceURL, nav)
	pos := cartography.LinkPosition(sourceURL, nav)
	if key == "" || pos == "" {
		return key
	}
	if c.linkPositions == nil {
		c.linkPositions = make(map[string]*linkPosition)
	}
	seen, ok := c.linkPositions[pos]
	if !ok {
		c.linkPositions[pos] = &linkPosition{id: nav.Element.ID, css: nav.Element.CSSSelector, key: key}
		return key
	}
	if !seen.volatile && seen.id == nav.Element.ID && seen.css == nav.Element.CSSSelector {
		return key
	}
	positional := cartography.PositionalLinkKey(sourceURL, nav)
	if !seen.volatile {
		seen.volatile = true
		if seen.key != positional {
			c.linkIdentities[positional] = max(c.linkIdentities[positional], c.linkIdentities[seen.key])
		}
	}
	return positional
}
