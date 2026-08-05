package crawler

import (
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// Hooks bundles optional lifecycle callbacks invoked by the headless crawler.
// All fields are optional; nil callbacks are skipped.
//
// Callbacks run synchronously on the crawler's own goroutine and block its
// progress for their duration — they should return quickly. A non-nil error
// returned from any callback aborts the surrounding crawl step and is
// propagated back to the caller of Crawl.
//
// A Hooks value is consulted on every action; callers may safely mutate the
// fields between Crawl invocations but should not mutate them while a Crawl
// is in flight. If multiple Crawl invocations run concurrently on the same
// engine the callbacks must themselves be safe to call concurrently.
//
// The supplied *browser.BrowserPage embeds *rod.Page (page.Page) for callers
// that want to reach the raw rod API. Callbacks should treat the page as
// read-only — navigating, closing, or otherwise mutating it from inside a
// callback races with the crawler.
type Hooks struct {
	// BeforeAction is invoked just before each action is dispatched, including
	// actions that will subsequently fail. A non-nil error aborts the action.
	BeforeAction func(page *browser.BrowserPage, action *types.Action) error
	// AfterAction is invoked only after an action completes successfully.
	// It is not called when the action returns an error. A non-nil error from
	// AfterAction is returned in place of the (otherwise nil) action error.
	AfterAction func(page *browser.BrowserPage, action *types.Action) error
	// BeforeNavigateBack is invoked once per browser-history back step during
	// state restoration, immediately before page.NavigateBack(). A non-nil
	// error aborts the navigation.
	BeforeNavigateBack func(page *browser.BrowserPage) error
}

// runWithActionHooks invokes hooks.BeforeAction, then fn, then on success
// hooks.AfterAction, returning the first non-nil error encountered. The
// semantics are:
//
//   - BeforeAction error → fn and AfterAction are skipped; error is returned.
//   - fn error           → AfterAction is skipped; fn error is returned.
//   - fn nil, AfterAction error → AfterAction error is returned.
//
// runWithActionHooks must remain side-effect-free beyond the supplied hooks
// so its semantics can be verified in isolation.
func runWithActionHooks(hooks Hooks, page *browser.BrowserPage, action *types.Action, fn func() error) (err error) {
	if cb := hooks.BeforeAction; cb != nil {
		if err := cb(page, action); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			return
		}
		if cb := hooks.AfterAction; cb != nil {
			if cerr := cb(page, action); cerr != nil {
				err = cerr
			}
		}
	}()
	return fn()
}

// runWithNavigateBackHook invokes hooks.BeforeNavigateBack and then fn,
// short-circuiting on a non-nil error from either.
func runWithNavigateBackHook(hooks Hooks, page *browser.BrowserPage, fn func() error) error {
	if cb := hooks.BeforeNavigateBack; cb != nil {
		if err := cb(page); err != nil {
			return err
		}
	}
	return fn()
}
