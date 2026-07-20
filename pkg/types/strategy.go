package types

import (
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
)

// ApplyCrawlStrategy overlays a named -crawl-strategy preset onto options.
// Empty CrawlStrategy is a no-op.
func ApplyCrawlStrategy(options *Options) error {
	if options == nil || options.CrawlStrategy == "" {
		return nil
	}
	s, err := cartography.ParseStrategy(options.CrawlStrategy)
	if err != nil {
		return err
	}
	cfg := cartography.ConfigFor(s)
	options.PageLoadStrategy = cfg.PageLoadStrategy
	options.DOMWaitTime = cfg.DOMWaitTime
	options.MaxDepth = cfg.MaxDepth
	options.ExplosionBudget = cfg.ExplosionBudget
	options.ExplosionHamming = cfg.ExplosionHamming
	options.RewalkSample = cfg.RewalkSample
	options.HeadlessAgents = cfg.AgentCount
	options.FilterSimilar = cfg.FilterSimilarURLs
	return nil
}
