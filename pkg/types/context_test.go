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
		defer crawlerOpts.Close()
		require.NotNil(t, crawlerOpts.RateLimit)
	})

	t.Run("cancelling context unblocks rate limiter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		opts := &Options{Context: ctx, RateLimit: 1}
		crawlerOpts, err := NewCrawlerOptions(opts)
		require.NoError(t, err)
		defer crawlerOpts.Close()

		crawlerOpts.RateLimit.Take()

		done := make(chan struct{})
		go func() {
			crawlerOpts.RateLimit.Take()
			close(done)
		}()

		cancel()

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

		done := make(chan struct{})
		go func() {
			crawlerOpts.RateLimit.Take()
			close(done)
		}()

		require.NoError(t, crawlerOpts.Close())

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("Take() did not unblock after Close()")
		}
	})
}
