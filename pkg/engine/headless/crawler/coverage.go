package crawler

import (
	"fmt"
	"sync"

	"github.com/go-rod/rod/lib/proto"
)

// CoverageTracker maintains a global set of executed JS byte ranges and
// computes coverage gain per crawl action. Thread-safe for multi-tab use.
// Only instantiated when --coverage-guided is enabled.
type CoverageTracker struct {
	mu      sync.Mutex
	covered map[string]struct{} // "scriptID:start-end" → seen

	templateCoverage map[string]*templateCovStats // URL fingerprint → stats
}

type templateCovStats struct {
	totalNewBytes  int
	visits         int
	zeroGainStreak int // consecutive visits with 0 new bytes
}

// NewCoverageTracker creates a new coverage tracker.
func NewCoverageTracker() *CoverageTracker {
	return &CoverageTracker{
		covered:          make(map[string]struct{}),
		templateCoverage: make(map[string]*templateCovStats),
	}
}

// RecordAndDiff takes CDP ProfilerScriptCoverage results, diffs against
// the global set, and returns the count of newly-covered bytes.
func (ct *CoverageTracker) RecordAndDiff(scripts []*proto.ProfilerScriptCoverage) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	newBytes := 0
	for _, script := range scripts {
		for _, fn := range script.Functions {
			for _, r := range fn.Ranges {
				if r.Count == 0 {
					continue
				}
				key := fmt.Sprintf("%s:%d-%d", script.ScriptID, r.StartOffset, r.EndOffset)
				if _, seen := ct.covered[key]; !seen {
					ct.covered[key] = struct{}{}
					newBytes += r.EndOffset - r.StartOffset
				}
			}
		}
	}
	return newBytes
}

// TotalRanges returns the total number of unique JS byte ranges seen so far.
func (ct *CoverageTracker) TotalRanges() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return len(ct.covered)
}

// RecordTemplateGain tracks coverage gain for a URL template.
func (ct *CoverageTracker) RecordTemplateGain(rawURL string, newBytes int) {
	if !isTemplatedURL(rawURL) {
		return
	}
	tmpl := urlFingerprint(rawURL)

	ct.mu.Lock()
	defer ct.mu.Unlock()

	stats := ct.templateCoverage[tmpl]
	if stats == nil {
		stats = &templateCovStats{}
		ct.templateCoverage[tmpl] = stats
	}
	stats.visits++
	stats.totalNewBytes += newBytes
	if newBytes == 0 {
		stats.zeroGainStreak++
	} else {
		stats.zeroGainStreak = 0
	}
}

// IsTemplateCoverageExhausted returns true if the template has had 2+
// consecutive visits with zero new JS coverage.
func (ct *CoverageTracker) IsTemplateCoverageExhausted(rawURL string) bool {
	if !isTemplatedURL(rawURL) {
		return false
	}
	tmpl := urlFingerprint(rawURL)

	ct.mu.Lock()
	defer ct.mu.Unlock()

	stats := ct.templateCoverage[tmpl]
	if stats == nil {
		return false
	}
	return stats.zeroGainStreak >= 2
}

// CoverageBoost returns a priority bonus (negative = higher priority) for
// actions discovered on pages with high coverage gain. Returns 0 if the
// template has low or no coverage data.
func (ct *CoverageTracker) CoverageBoost(originURL string) int {
	tmpl := originURL
	if isTemplatedURL(originURL) {
		tmpl = urlFingerprint(originURL)
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	stats := ct.templateCoverage[tmpl]
	if stats == nil || stats.visits == 0 {
		// First visit to a new page type — give moderate boost
		return -10
	}

	avgGain := stats.totalNewBytes / stats.visits
	switch {
	case avgGain > 2000:
		return -15
	case avgGain > 500:
		return -10
	default:
		return 0
	}
}
