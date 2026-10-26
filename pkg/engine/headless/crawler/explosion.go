package crawler

import (
	"net/url"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// overExplosionBudget reports whether pageState is one near-duplicate page too
// many. Only a first visit to another document counts. A revisit keeps the
// first answer: an admitted state stays admitted, and a rejected state, which
// is still added to the graph, stays rejected. An action that changes the
// page in place (a menu or a dropdown) only reveals its links, so it follows
// the page it changes: queued on an admitted page, dropped on a rejected one.
func (c *Crawler) overExplosionBudget(pageState *types.PageState) bool {
	if c.explosionBudget == nil {
		return false
	}
	if _, rejected := c.explosionRejected[pageState.UniqueID]; rejected {
		return true
	}
	if _, err := c.crawlGraph.GetPageState(pageState.UniqueID); err == nil {
		return false
	}
	if origin, err := c.crawlGraph.GetPageState(pageState.OriginID); err == nil && samePage(origin.URL, pageState.URL) {
		if _, rejected := c.explosionRejected[origin.UniqueID]; rejected {
			c.rejectExplosion(pageState)
			return true
		}
		return false
	}
	if c.explosionBudget.Allow(pageState.SimHash) {
		return false
	}
	c.rejectExplosion(pageState)
	return true
}

func (c *Crawler) rejectExplosion(pageState *types.PageState) {
	if c.explosionRejected == nil {
		c.explosionRejected = map[string]struct{}{}
	}
	c.explosionRejected[pageState.UniqueID] = struct{}{}
}

// samePage reports whether a and b are one page: the same document with the
// same query. Only the fragment may differ. A new query (/calendar?month=2)
// is another page and counts against the budget.
func samePage(a, b string) bool {
	if !sameDocument(a, b) {
		return false
	}
	ua, errA := url.Parse(a)
	ub, errB := url.Parse(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return ua.Query().Encode() == ub.Query().Encode()
}
