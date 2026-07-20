package crawler

import (
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

func (c *Crawler) acceptLinkIdentity(nav *types.Action) bool {
	budget := c.options.LinkIdentityBudget
	if budget < 0 {
		return true // disabled
	}
	if budget == 0 {
		budget = 1
	}
	f := cartography.FeaturesFromAction(nav)
	for _, id := range c.linkIdentities {
		if !id.Matches(f) {
			continue
		}
		id.Observe(f)
		return id.Visits() <= budget
	}
	id := &cartography.LinkIdentity{}
	id.Observe(f)
	c.linkIdentities = append(c.linkIdentities, id)
	return true
}
