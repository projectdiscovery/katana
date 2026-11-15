package hybrid

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newTestWatcher builds a watcher with no CDP subscription so the notify /
// revalidate / sleep paths can be driven directly through the state machine.
func newTestWatcher(t *testing.T) *mainDocWatcher {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &mainDocWatcher{ctx: ctx, cancel: cancel, notify: make(chan struct{}, 1)}
}

// failWatched drives a main-document failure and finalizes it synchronously.
func failWatched(w *mainDocWatcher, reqID string) {
	w.mu.Lock()
	w.nav.onRequestWillBeSent(reqID, true, false)
	w.nav.onLoadingFailed(reqID)
	w.reason = "net::ERR_CONNECTION_CLOSED"
	w.failGen++
	gen := w.failGen
	w.mu.Unlock()
	w.finalizeAfter(gen)
}

func TestMainDocWatcher_StaleNotificationIsIgnored(t *testing.T) {
	w := newTestWatcher(t)
	failWatched(w, "A")

	// A replacement navigation supersedes the failure after the notification
	// was sent but before anyone consumed it.
	w.mu.Lock()
	w.nav.onRequestWillBeSent("B", true, false)
	w.mu.Unlock()

	failed, _ := w.failedNow()
	require.False(t, failed, "superseded failure must not revalidate")

	// The stale notification is drained and the sleep runs to completion.
	start := time.Now()
	require.NoError(t, w.sleepUnlessFailed(context.Background(), 150*time.Millisecond))
	require.GreaterOrEqual(t, time.Since(start), 150*time.Millisecond)
}

func TestMainDocWatcher_SleepInterruptedByFailure(t *testing.T) {
	w := newTestWatcher(t)
	w.mu.Lock()
	w.nav.onRequestWillBeSent("A", true, false)
	w.mu.Unlock()

	go func() {
		time.Sleep(50 * time.Millisecond)
		w.mu.Lock()
		w.nav.onLoadingFailed("A")
		w.reason = "net::ERR_CONNECTION_CLOSED"
		w.failGen++
		gen := w.failGen
		w.mu.Unlock()
		w.finalizeAfter(gen) // includes the settle window
	}()

	start := time.Now()
	err := w.sleepUnlessFailed(context.Background(), 30*time.Second)
	require.True(t, errors.Is(err, errMainDocumentFailed), "got %v", err)
	require.Contains(t, err.Error(), "ERR_CONNECTION_CLOSED")
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestMainDocWatcher_NilWatcherSleepIsCtxAware(t *testing.T) {
	var w *mainDocWatcher
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	require.NoError(t, w.sleepUnlessFailed(ctx, 30*time.Second))
	require.Less(t, time.Since(start), 5*time.Second)
	failed, _ := w.failedNow()
	require.False(t, failed)
}

func TestMainDocWatcher_SubscriptionOutlivesParentlessTimeout(t *testing.T) {
	// The watcher's lifetime is its own ctx (parented to the session), not the
	// navigation timeout: a finalized failure after a nav-timeout-sized delay
	// still notifies.
	w := newTestWatcher(t)
	navCtx, navCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer navCancel()
	<-navCtx.Done()

	failWatched(w, "A")
	select {
	case <-w.notifyChan():
	default:
		t.Fatal("failure after nav timeout must still notify")
	}
	failed, reason := w.failedNow()
	require.True(t, failed)
	require.Equal(t, "net::ERR_CONNECTION_CLOSED", reason)
}

// recordFailure drives a fresh failure on reqID the way onLoadingFailed does,
// without starting the finalizer, and returns when it was recorded.
func recordFailure(w *mainDocWatcher, reqID string) {
	w.mu.Lock()
	w.nav.onLoadingFailed(reqID)
	w.reason = "net::ERR_CONNECTION_CLOSED"
	w.failGen++
	w.failedAt = time.Now()
	w.mu.Unlock()
}

func TestMainDocWatcher_SettledFailure_WaitsOutTheWindow(t *testing.T) {
	w := newTestWatcher(t)
	w.mu.Lock()
	w.nav.onRequestWillBeSent("A", true, false)
	w.mu.Unlock()
	recordFailure(w, "A")

	start := time.Now()
	failed, reason := w.settledFailure(context.Background())
	require.True(t, failed)
	require.Equal(t, "net::ERR_CONNECTION_CLOSED", reason)
	require.GreaterOrEqual(t, time.Since(start), mainDocSettleWindow-10*time.Millisecond,
		"a fresh failure must not be judged before its settle window")
}

func TestMainDocWatcher_SettledFailure_ReplacementInsideWindowWins(t *testing.T) {
	w := newTestWatcher(t)
	w.mu.Lock()
	w.nav.onRequestWillBeSent("A", true, false)
	w.mu.Unlock()
	recordFailure(w, "A")

	// A superseding navigation lands inside the settle window.
	go func() {
		time.Sleep(mainDocSettleWindow / 3)
		w.mu.Lock()
		w.nav.onRequestWillBeSent("B", true, false)
		w.mu.Unlock()
	}()

	failed, _ := w.settledFailure(context.Background())
	require.False(t, failed, "a replacement inside the settle window must retire the failure")
}

func TestMainDocWatcher_SettledFailure_NoFailureReturnsImmediately(t *testing.T) {
	w := newTestWatcher(t)
	w.mu.Lock()
	w.nav.onRequestWillBeSent("A", true, false)
	w.nav.onLoadingFinished("A")
	w.mu.Unlock()

	start := time.Now()
	failed, _ := w.settledFailure(context.Background())
	require.False(t, failed)
	require.Less(t, time.Since(start), 50*time.Millisecond)
}

func TestMainDocWatcher_SettledFailure_CtxEndKeepsObservedFailure(t *testing.T) {
	w := newTestWatcher(t)
	w.mu.Lock()
	w.nav.onRequestWillBeSent("A", true, false)
	w.mu.Unlock()
	recordFailure(w, "A")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	failed, _ := w.settledFailure(ctx)
	require.True(t, failed, "an observed failure stands when the crawl is ending")
}
