package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPopWithContextCancellation(t *testing.T) {
	q, err := New("breadth-first")
	require.NoError(t, err)

	q.Push("item1", 0)
	ctx, cancel := context.WithCancel(context.Background())
	items := q.PopWithContext(ctx)

	item := <-items
	require.Equal(t, "item1", item)

	cancel()

	select {
	case _, ok := <-items:
		require.False(t, ok)
	case <-time.After(3 * time.Second):
		t.Fatal("PopWithContext channel did not close after context cancellation")
	}
}

func TestPopWithContextNil(t *testing.T) {
	q, err := New("breadth-first")
	require.NoError(t, err)
	q.Push("item1", 0)

	// nil context should not panic (testing nil-safety contract)
	items := q.PopWithContext(nil) //nolint:staticcheck // SA1012: intentionally testing nil context
	item := <-items
	require.Equal(t, "item1", item)
}

func TestPopBackwardCompatible(t *testing.T) {
	q, err := New("breadth-first")
	require.NoError(t, err)
	q.Push("item1", 0)

	items := q.Pop()
	item := <-items
	require.Equal(t, "item1", item)
}

// A popped item stays in progress until Done, so the queue neither closes
// while its page may still push links nor waits once nothing is left.
func TestPopEndsWhenNothingIsInProgress(t *testing.T) {
	q, err := New("breadth-first")
	require.NoError(t, err)
	q.Push("root", 0)

	items := q.PopWithContext(context.Background())
	require.Equal(t, "root", <-items)

	select {
	case <-items:
		t.Fatal("queue closed while the popped item was still in progress")
	case <-time.After(200 * time.Millisecond):
	}

	q.Push("child", 0)
	q.Done()
	select {
	case item := <-items:
		require.Equal(t, "child", item)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("pushed item was not handed out promptly")
	}

	q.Done()
	select {
	case _, ok := <-items:
		require.False(t, ok)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("queue stayed open with nothing queued or in progress")
	}
}
