package headless

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/assert"
)

// TestSetHooks_SnapshotsCallerStruct guards the contract that SetHooks copies
// the supplied Hooks value: mutating the caller's struct after SetHooks
// returns must not affect the engine's installed hooks.
func TestSetHooks_SnapshotsCallerStruct(t *testing.T) {
	original := func(_ *browser.BrowserPage, _ *types.Action) error { return nil }
	mutated := func(_ *browser.BrowserPage, _ *types.Action) error { return assert.AnError }

	hooks := &Hooks{BeforeAction: original}
	h := &Headless{}
	h.SetHooks(hooks)

	hooks.BeforeAction = mutated

	require := assert.New(t)
	require.NotNil(h.hooks.BeforeAction, "BeforeAction should still be installed")
	require.NoError(
		h.hooks.BeforeAction(nil, nil),
		"engine must keep the snapshot taken at SetHooks time, not the caller's mutated pointer",
	)
}

// TestSetHooks_NilClears verifies that passing nil resets the engine to the
// zero-value Hooks, which is in turn safe to invoke at every callback site.
func TestSetHooks_NilClears(t *testing.T) {
	h := &Headless{}
	h.SetHooks(&Hooks{BeforeAction: func(_ *browser.BrowserPage, _ *types.Action) error { return nil }})
	h.SetHooks(nil)

	assert.Equal(t, Hooks{}, h.hooks, "passing nil to SetHooks should clear all installed hooks")
}
