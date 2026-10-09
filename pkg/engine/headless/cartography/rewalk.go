package cartography

import (
	"context"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// Session clears browser-side state so a rewalk starts from a blank slate.
type Session interface {
	Clear(ctx context.Context) error
}

// Observer replays a path after a clean session and returns what location was reached.
type Observer interface {
	Rewalk(ctx context.Context, path *types.Path) (LocationFingerprint, error)
}

// RewalkReport is one clean-session replay of a path toward expected.
type RewalkReport struct {
	Path     *types.Path
	Expected LocationFingerprint
	Observed LocationFingerprint
	Err      error
}

// Runner clears the session, rewalks each path, and classifies the set of outcomes.
type Runner struct {
	Session    Session
	Observer   Observer
	MaxHamming int
}

// Run executes clean-session rewalks for paths that should reach expected.
func (r *Runner) Run(ctx context.Context, expected LocationFingerprint, paths []*types.Path) (Outcome, []RewalkReport, error) {
	if r == nil || r.Observer == nil {
		return OutcomeNonDeterministic, nil, nil
	}
	reports := make([]RewalkReport, 0, len(paths))
	observed := make([]LocationFingerprint, 0, len(paths))
	for _, p := range paths {
		if r.Session != nil {
			if err := r.Session.Clear(ctx); err != nil {
				return OutcomeNonDeterministic, reports, err
			}
		}
		rep := RewalkReport{Path: p, Expected: expected}
		got, err := r.Observer.Rewalk(ctx, p)
		rep.Observed = got
		rep.Err = err
		reports = append(reports, rep)
		if err != nil {
			continue
		}
		observed = append(observed, got)
	}
	return ClassifyRewalks(expected, observed, r.MaxHamming), reports, nil
}
