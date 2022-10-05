package filters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSimpleFilter(t *testing.T) {
	simple, err := NewSimple()
	require.NoError(t, err, "could not create filter")
	defer simple.Close()

	unique := simple.Unique("https://example.com")
	require.True(t, unique, "could not get unique value")

	unique = simple.Unique("https://example.com")
	require.False(t, unique, "could get unique value")
}
