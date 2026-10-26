package types

import (
	"fmt"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
)

// ApplyCrawlStrategy overlays a named -crawl-strategy preset onto options.
// Empty CrawlStrategy is a no-op. Requires headless, hybrid, or -cwu.
func ApplyCrawlStrategy(options *Options) error {
	if options == nil || options.CrawlStrategy == "" {
		return nil
	}
	if !options.Headless && !options.HeadlessHybrid && options.ChromeWSUrl == "" {
		return fmt.Errorf("-crawl-strategy requires -headless, -hh, or -cwu")
	}
	s, err := cartography.ParseStrategy(options.CrawlStrategy)
	if err != nil {
		return err
	}
	cfg := cartography.ConfigFor(s)
	gologger.Warning().Msgf(
		"crawl-strategy %q overlays depth=%d page-load=%s dom-wait=%ds agents=%d rewalk=%d explosion-budget=%d filter-similar=%t",
		s, cfg.MaxDepth, cfg.PageLoadStrategy, cfg.DOMWaitTime, cfg.AgentCount, cfg.RewalkSample, cfg.ExplosionBudget, cfg.FilterSimilarURLs,
	)
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
