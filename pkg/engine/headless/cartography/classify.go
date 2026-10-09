package cartography

import (
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer/simhash"
)

// Outcome is the result of comparing rewalk observations to a stored location.
type Outcome string

const (
	OutcomeMatch            Outcome = "match"
	OutcomeAppChange        Outcome = "app_change"
	OutcomeSessionFork      Outcome = "session_fork"
	OutcomeNonDeterministic Outcome = "non_deterministic"
)

// LocationFingerprint is the stable identity of a crawl location for rewalk checks.
type LocationFingerprint struct {
	UniqueID string
	URL      string
	SimHash  uint64
}

// DefaultMaxHamming is the max SimHash distance still treated as the same location.
const DefaultMaxHamming = 3

// SameLocation reports whether two fingerprints describe the same location.
func SameLocation(a, b LocationFingerprint, maxHamming int) bool {
	if maxHamming < 0 {
		maxHamming = DefaultMaxHamming
	}
	if a.UniqueID != "" && b.UniqueID != "" && a.UniqueID == b.UniqueID {
		return true
	}
	if a.SimHash == 0 && b.SimHash == 0 {
		return a.URL != "" && a.URL == b.URL
	}
	return int(simhash.Distance(a.SimHash, b.SimHash)) <= maxHamming
}

// ClassifyRewalks applies Burp-style divergence rules to rewalks of paths that
// should reach expected:
//   - all match expected → match
//   - none match expected, but all agree → app_change
//   - some match expected, some do not → session_fork
//   - otherwise → non_deterministic
func ClassifyRewalks(expected LocationFingerprint, observed []LocationFingerprint, maxHamming int) Outcome {
	if len(observed) == 0 {
		return OutcomeNonDeterministic
	}
	matchExpected := 0
	for _, o := range observed {
		if SameLocation(expected, o, maxHamming) {
			matchExpected++
		}
	}
	switch {
	case matchExpected == len(observed):
		return OutcomeMatch
	case matchExpected == 0:
		if allAgree(observed, maxHamming) {
			return OutcomeAppChange
		}
		return OutcomeNonDeterministic
	default:
		return OutcomeSessionFork
	}
}

func allAgree(fs []LocationFingerprint, maxHamming int) bool {
	if len(fs) <= 1 {
		return true
	}
	base := fs[0]
	for i := 1; i < len(fs); i++ {
		if !SameLocation(base, fs[i], maxHamming) {
			return false
		}
	}
	return true
}
