package cartography

import (
	"context"
	"errors"
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

type fakeObserver struct {
	results []LocationFingerprint
	err     error
	calls   int
	clears  int
}

func (f *fakeObserver) Clear(context.Context) error {
	f.clears++
	return nil
}

func (f *fakeObserver) Rewalk(context.Context, *types.Path) (LocationFingerprint, error) {
	i := f.calls
	f.calls++
	if f.err != nil {
		return LocationFingerprint{}, f.err
	}
	if i >= len(f.results) {
		return LocationFingerprint{}, errors.New("no more results")
	}
	return f.results[i], nil
}

func TestRunnerRun(t *testing.T) {
	t.Parallel()
	expected := LocationFingerprint{SimHash: 0b11110000}
	paths := []*types.Path{
		{EntryID: "root", TargetID: "a"},
		{EntryID: "root", TargetID: "a"},
	}
	obs := &fakeObserver{results: []LocationFingerprint{
		{SimHash: 0b11110000},
		{SimHash: 0b00001111},
	}}
	r := &Runner{Clear: obs.Clear, Rewalk: obs.Rewalk, MaxHamming: 3}
	outcome, reports, err := r.Run(context.Background(), expected, paths)
	require.NoError(t, err)
	require.Equal(t, OutcomeSessionFork, outcome)
	require.Equal(t, 2, obs.clears)
	require.Len(t, reports, 2)
}
