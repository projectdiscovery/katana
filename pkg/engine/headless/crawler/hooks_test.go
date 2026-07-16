package crawler

import (
	"errors"
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trace captures the relative ordering of hook and dispatch invocations so
// individual cases can both assert "what ran" and "in what order".
type trace struct{ steps []string }

func (tr *trace) add(s string) { tr.steps = append(tr.steps, s) }

func TestRunWithActionHooks_Success(t *testing.T) {
	var tr trace
	hooks := Hooks{
		BeforeAction: func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("before"); return nil },
		AfterAction:  func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("after"); return nil },
	}

	err := runWithActionHooks(hooks, nil, &types.Action{}, func() error {
		tr.add("dispatch")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"before", "dispatch", "after"}, tr.steps,
		"BeforeAction → dispatch → AfterAction must run in order on success")
}

func TestRunWithActionHooks_DispatchError_SkipsAfter(t *testing.T) {
	var tr trace
	sentinel := errors.New("dispatch failed")
	hooks := Hooks{
		BeforeAction: func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("before"); return nil },
		AfterAction:  func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("after"); return nil },
	}

	err := runWithActionHooks(hooks, nil, &types.Action{}, func() error {
		tr.add("dispatch")
		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, []string{"before", "dispatch"}, tr.steps,
		"AfterAction must not run when dispatch returns an error")
}

func TestRunWithActionHooks_BeforeError_SkipsDispatchAndAfter(t *testing.T) {
	var tr trace
	sentinel := errors.New("aborted by BeforeAction")
	hooks := Hooks{
		BeforeAction: func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("before"); return sentinel },
		AfterAction:  func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("after"); return nil },
	}

	err := runWithActionHooks(hooks, nil, &types.Action{}, func() error {
		tr.add("dispatch")
		return nil
	})

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, []string{"before"}, tr.steps,
		"BeforeAction error must short-circuit dispatch and AfterAction")
}

func TestRunWithActionHooks_AfterError_PropagatedOnSuccess(t *testing.T) {
	var tr trace
	sentinel := errors.New("post-action failure")
	hooks := Hooks{
		AfterAction: func(_ *browser.BrowserPage, _ *types.Action) error { tr.add("after"); return sentinel },
	}

	err := runWithActionHooks(hooks, nil, &types.Action{}, func() error {
		tr.add("dispatch")
		return nil
	})

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, []string{"dispatch", "after"}, tr.steps)
}

func TestRunWithActionHooks_ZeroValue_NoPanicNoCallbacks(t *testing.T) {
	called := false
	err := runWithActionHooks(Hooks{}, nil, &types.Action{}, func() error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called, "dispatch must run even when no hooks are installed")
}

func TestRunWithNavigateBackHook_Success(t *testing.T) {
	var tr trace
	hooks := Hooks{
		BeforeNavigateBack: func(_ *browser.BrowserPage) error { tr.add("before"); return nil },
	}

	err := runWithNavigateBackHook(hooks, nil, func() error {
		tr.add("navigate")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"before", "navigate"}, tr.steps)
}

func TestRunWithNavigateBackHook_BeforeError_SkipsNavigation(t *testing.T) {
	var tr trace
	sentinel := errors.New("aborted")
	hooks := Hooks{
		BeforeNavigateBack: func(_ *browser.BrowserPage) error { tr.add("before"); return sentinel },
	}

	err := runWithNavigateBackHook(hooks, nil, func() error {
		tr.add("navigate")
		return nil
	})

	require.ErrorIs(t, err, sentinel)
	assert.Equal(t, []string{"before"}, tr.steps,
		"BeforeNavigateBack error must short-circuit the navigation")
}

func TestRunWithNavigateBackHook_NavigationError_Propagated(t *testing.T) {
	sentinel := errors.New("nav failed")
	err := runWithNavigateBackHook(Hooks{}, nil, func() error { return sentinel })

	require.ErrorIs(t, err, sentinel)
}

// TestExecuteCrawlStateAction_DispatchesThroughHooks is the integration sanity
// check that executeCrawlStateAction actually routes through runWithActionHooks.
// The dispatch unavoidably fails for ActionTypeUnknown (no real browser), so
// we only assert the hooks behave as the runner contract demands on that path.
func TestExecuteCrawlStateAction_DispatchesThroughHooks(t *testing.T) {
	var beforeCalled, afterCalled bool
	c := &Crawler{
		options: Options{
			Hooks: Hooks{
				BeforeAction: func(_ *browser.BrowserPage, _ *types.Action) error { beforeCalled = true; return nil },
				AfterAction:  func(_ *browser.BrowserPage, _ *types.Action) error { afterCalled = true; return nil },
			},
		},
	}

	err := c.executeCrawlStateAction(&types.Action{Type: types.ActionTypeUnknown}, &browser.BrowserPage{})

	require.Error(t, err)
	assert.True(t, beforeCalled)
	assert.False(t, afterCalled)
}
