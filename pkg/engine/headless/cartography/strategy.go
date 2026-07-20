package cartography

import (
	"fmt"
	"strings"
)

// Strategy is a named crawl intensity preset.
type Strategy string

const (
	StrategyFast     Strategy = "fast"
	StrategyBalanced Strategy = "balanced"
	StrategyThorough Strategy = "thorough"
)

// StrategyConfig is the knob set a preset expands into.
type StrategyConfig struct {
	PageLoadStrategy  string
	DOMWaitTime       int
	MaxDepth          int
	ExplosionBudget   int
	ExplosionHamming  int
	RewalkSample      int // how many discovered paths to clean-session rewalk after the crawl (0 = off)
	AgentCount        int
	FilterSimilarURLs bool
}

// ParseStrategy validates a strategy name, tolerating surrounding whitespace
// and any letter case.
func ParseStrategy(s string) (Strategy, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch Strategy(s) {
	case StrategyFast, StrategyBalanced, StrategyThorough:
		return Strategy(s), nil
	case "":
		return StrategyBalanced, nil
	default:
		return "", fmt.Errorf("unknown crawl-strategy %q (want fast, balanced, thorough)", s)
	}
}

// ConfigFor returns the preset configuration.
func ConfigFor(s Strategy) StrategyConfig {
	switch s {
	case StrategyFast:
		return StrategyConfig{
			PageLoadStrategy:  "domcontentloaded",
			DOMWaitTime:       0,
			MaxDepth:          2,
			ExplosionBudget:   1,
			ExplosionHamming:  3,
			RewalkSample:      0,
			AgentCount:        1,
			FilterSimilarURLs: true,
		}
	case StrategyThorough:
		return StrategyConfig{
			PageLoadStrategy:  "heuristic",
			DOMWaitTime:       2,
			MaxDepth:          5,
			ExplosionBudget:   3,
			ExplosionHamming:  2,
			RewalkSample:      3,
			AgentCount:        2,
			FilterSimilarURLs: true,
		}
	default: // balanced
		return StrategyConfig{
			PageLoadStrategy:  "domcontentloaded",
			DOMWaitTime:       1,
			MaxDepth:          3,
			ExplosionBudget:   1,
			ExplosionHamming:  3,
			RewalkSample:      1,
			AgentCount:        1,
			FilterSimilarURLs: true,
		}
	}
}
