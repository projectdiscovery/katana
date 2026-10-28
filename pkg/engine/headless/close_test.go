package headless

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newClosingTestHeadless() *Headless {
	return &Headless{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestCloseJoinsRunningCrawls(t *testing.T) {
	h := newClosingTestHeadless()
	h.crawls.Add(1)
	const crawlCleanup = 200 * time.Millisecond
	go func() {
		time.Sleep(crawlCleanup)
		h.crawls.Done()
	}()

	start := time.Now()
	require.NoError(t, h.Close())
	require.GreaterOrEqual(t, time.Since(start), crawlCleanup, "Close must wait for running crawls")
}

func TestCloseJoinIsBounded(t *testing.T) {
	old := crawlJoinTimeout
	crawlJoinTimeout = 100 * time.Millisecond
	t.Cleanup(func() { crawlJoinTimeout = old })

	h := newClosingTestHeadless()
	h.crawls.Add(1)
	t.Cleanup(h.crawls.Done)

	done := make(chan struct{})
	go func() {
		_ = h.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close must not wait past crawlJoinTimeout for a stuck crawl")
	}
}

func TestCrawlAfterCloseIsRejected(t *testing.T) {
	h := newClosingTestHeadless()
	require.NoError(t, h.Close())
	require.ErrorIs(t, h.Crawl("http://127.0.0.1:1"), errClosing)
}
