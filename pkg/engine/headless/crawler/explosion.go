package crawler

import "github.com/projectdiscovery/katana/pkg/engine/headless/types"

// overExplosionBudget reports whether pageState is one near-duplicate page too
// many. Only a first visit to another document counts. A revisited state was
// already admitted, and an action that changes the page in place (a menu or a
// dropdown) only reveals its links, which must still be queued.
func (c *Crawler) overExplosionBudget(pageState *types.PageState) bool {
	if c.explosionBudget == nil {
		return false
	}
	if _, err := c.crawlGraph.GetPageState(pageState.UniqueID); err == nil {
		return false
	}
	if origin, err := c.crawlGraph.GetPageState(pageState.OriginID); err == nil && sameDocument(origin.URL, pageState.URL) {
		return false
	}
	return !c.explosionBudget.Allow(pageState.SimHash)
}
