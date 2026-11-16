package hybrid

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
)

// A navigation that starts after the main document finished (an onclick
// download, a 204) must not reject the displayed page unless it commits.

func TestMainDocNavUncommittedDownloadAfterFinishIgnored(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onLoadingFinished("req-1")
	nav.onRequestWillBeSent("req-dl", true, false)
	nav.onLoadingFailed("req-dl")
	nav.settleElapsed()
	require.Equal(t, mdPending, nav.verdict(), "a download that never committed cannot reject the finished page")
	require.Equal(t, "req-1", nav.watched)
	require.True(t, nav.finished)
}

func TestMainDocNavCommittedReplacementAfterFinishCanFail(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onLoadingFinished("req-1")
	nav.onRequestWillBeSent("req-2", true, false)
	nav.onCommitted("req-2")
	nav.onLoadingFailed("req-2")
	nav.settleElapsed()
	require.Equal(t, mdFailed, nav.verdict(), "a committed replacement is the displayed document")
}

func TestMainDocNavReplacementWhileLoadingIsImmediate(t *testing.T) {
	var nav mainDocNav
	nav.onRequestWillBeSent("req-1", true, false)
	nav.onRequestWillBeSent("req-2", true, false)
	require.Equal(t, "req-2", nav.watched)
	require.Empty(t, nav.candidate)
}

func TestMainDocWatcherFrameNavigatedPromotesOnlyMainFrame(t *testing.T) {
	w := newTestWatcher(t)
	w.mainFrameID = "main"
	w.mu.Lock()
	w.nav.onRequestWillBeSent("req-1", true, false)
	w.nav.onLoadingFinished("req-1")
	w.nav.onRequestWillBeSent("req-2", true, false)
	w.mu.Unlock()

	// A child frame (or another frame id) committing the same loader is ignored.
	w.onFrameNavigated(&proto.PageFrameNavigated{Frame: &proto.PageFrame{ID: "child", ParentID: "main", LoaderID: "req-2"}})
	w.onFrameNavigated(&proto.PageFrameNavigated{Frame: &proto.PageFrame{ID: "other", LoaderID: "req-2"}})
	w.onFrameNavigated(&proto.PageFrameNavigated{})
	require.Equal(t, "req-1", w.nav.watched)

	w.onFrameNavigated(&proto.PageFrameNavigated{Frame: &proto.PageFrame{ID: "main", LoaderID: "req-2"}})
	require.Equal(t, "req-2", w.nav.watched)
	require.Empty(t, w.nav.candidate)
}
