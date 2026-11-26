package hybrid

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainDocNavFailedDocument(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onLoadingFailed("other")
	require.Equal(t, mdPending, nav.verdict(), "a different request is not the main document")

	nav.onLoadingFailed("req-1")
	require.Equal(t, mdPending, nav.verdict(), "the failure waits out the settle window")
	nav.settleElapsed()
	require.Equal(t, mdFailed, nav.verdict())
}

func TestMainDocNavRedirectKeepsTheWatchedRequest(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onRequestWillBeSent("req-1", false, true)
	nav.onLoadingFailed("req-1")
	nav.settleElapsed()
	require.Equal(t, mdFailed, nav.verdict(), "a redirect reuses the request id")
}

func TestMainDocNavReplacementDropsTheOldFailure(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onLoadingFailed("req-1")
	nav.onRequestWillBeSent("req-2", true, false)
	nav.settleElapsed()
	require.Equal(t, mdPending, nav.verdict(), "the new main document replaced the failed one")
}

func TestMainDocNavFinishedRetiresTheFailure(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onLoadingFinished("req-1")
	nav.onLoadingFailed("req-1")
	nav.settleElapsed()
	require.Equal(t, mdPending, nav.verdict(), "a finished main document is not a failure")
}
