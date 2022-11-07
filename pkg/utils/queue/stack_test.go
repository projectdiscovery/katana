package queue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStack(t *testing.T) {
	queue := newStack()
	queue.Push("lower")
	queue.Push("higher")
	queue.Push("lowest")

	require.Equal(t, "lowest", queue.Pop(), "could not pop correct value")
	require.Equal(t, "higher", queue.Pop(), "could not pop correct value")
	require.Equal(t, "lower", queue.Pop(), "could not pop correct value")
}
