package crawler

import (
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// acceptLinkIdentity reports whether nav, found on the page at sourceURL, may
// be enqueued. With LinkIdentityBudget > 0, at most that many near-duplicate
// doors (same cartography.LinkKey) are enqueued. Zero or less disables it.
func (c *Crawler) acceptLinkIdentity(nav *types.Action, sourceURL string) bool {
	budget := c.options.LinkIdentityBudget
	if budget <= 0 {
		return true
	}
	key := cartography.LinkKey(sourceURL, nav)
	if key == "" {
		return true
	}
	if c.linkIdentities == nil {
		c.linkIdentities = make(map[string]int)
	}
	if c.linkIdentities[key] >= budget {
		return false
	}
	c.linkIdentities[key]++
	return true
}
