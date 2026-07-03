package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewCrawlerOptionsWithContext(t *testing.T) {
	t.Run("nil context defaults to background", func(t *testing.T) {
		opts := &Options{RateLimit: 10}
		crawlerOpts, err := NewCrawlerOptions(opts)
		require.NoError(t, err)
		defer func() { _ = crawlerOpts.Close() }()
		require.NotNil(t, crawlerOpts.RateLimit)
	})

	t.Run("cancelling context unblocks rate limiter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		opts := &Options{Context: ctx, RateLimit: 1}
		crawlerOpts, err := NewCrawlerOptions(opts)
		require.NoError(t, err)
		defer func() { _ = crawlerOpts.Close() }()

		crawlerOpts.RateLimit.Take()

		// Ensure the goroutine is blocked on Take() before we cancel.
		ready := make(chan struct{})
		done := make(chan struct{})
		go func() {
			close(ready)
			crawlerOpts.RateLimit.Take()
			close(done)
		}()
		<-ready

		cancel()

		// The rate limiter detects cancellation on its next tick (~1s for
		// RateLimit:1). Use 2s to allow for scheduling jitter.
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Take() did not unblock after context cancellation")
		}
	})

	t.Run("Close stops rate limiter goroutine", func(t *testing.T) {
		opts := &Options{RateLimit: 1}
		crawlerOpts, err := NewCrawlerOptions(opts)
		require.NoError(t, err)

		crawlerOpts.RateLimit.Take()

		ready := make(chan struct{})
		done := make(chan struct{})
		go func() {
			close(ready)
			crawlerOpts.RateLimit.Take()
			close(done)
		}()
		<-ready

		require.NoError(t, crawlerOpts.Close())

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Take() did not unblock after Close()")
		}
	})
}
