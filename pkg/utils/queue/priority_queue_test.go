package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPriorityQueue(t *testing.T) {
	queue := newPriorityQueue()
	queue.Push("lower", 3)
	queue.Push("higher", 4)
	queue.Push("lowest", 1)

	require.Equal(t, "lowest", queue.Pop(), "could not pop lowest priority first")
	require.Equal(t, "lower", queue.Pop(), "could not pop lower priority first")
	require.Equal(t, "higher", queue.Pop(), "could not pop higher priority first")
}
