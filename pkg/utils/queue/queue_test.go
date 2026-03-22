package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPopWithContextCancellation(t *testing.T) {
	q, err := New("breadth-first", 30)
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
	q, err := New("breadth-first", 2)
	require.NoError(t, err)
	q.Push("item1", 0)

	// nil context should not panic (testing nil-safety contract)
	items := q.PopWithContext(nil) //nolint:staticcheck // SA1012: intentionally testing nil context
	item := <-items
	require.Equal(t, "item1", item)
}

func TestPopBackwardCompatible(t *testing.T) {
	q, err := New("breadth-first", 2)
	require.NoError(t, err)
	q.Push("item1", 0)

	items := q.Pop()
	item := <-items
	require.Equal(t, "item1", item)
}
