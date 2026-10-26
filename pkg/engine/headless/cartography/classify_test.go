package cartography

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSameLocation(t *testing.T) {
	t.Parallel()
	a := LocationFingerprint{UniqueID: "x", SimHash: 0b1111}
	b := LocationFingerprint{UniqueID: "x", SimHash: 0}
	require.True(t, SameLocation(a, b, 3))

	c := LocationFingerprint{SimHash: 0b1111}
	d := LocationFingerprint{SimHash: 0b1110} // hamming 1
	require.True(t, SameLocation(c, d, 3))
	require.False(t, SameLocation(c, LocationFingerprint{SimHash: 0}, 3))
	// A low-popcount hash is close to zero but must not match a page that has
	// no content fingerprint; only the URL can decide then.
	low := LocationFingerprint{URL: "https://example.com/a", SimHash: 0b111}
	require.False(t, SameLocation(low, LocationFingerprint{URL: "https://example.com/b"}, 3))
	require.True(t, SameLocation(low, LocationFingerprint{URL: "https://example.com/a"}, 3))
}

func TestClassifyRewalks(t *testing.T) {
	t.Parallel()
	expected := LocationFingerprint{SimHash: 0b11110000}

	t.Run("all match", func(t *testing.T) {
		t.Parallel()
		got := ClassifyRewalks(expected, []LocationFingerprint{
			{SimHash: 0b11110000},
			{SimHash: 0b11110001}, // distance 1
		}, 3)
		require.Equal(t, OutcomeMatch, got)
	})

	t.Run("app change", func(t *testing.T) {
		t.Parallel()
		got := ClassifyRewalks(expected, []LocationFingerprint{
			{SimHash: 0b00001111},
			{SimHash: 0b00001110},
		}, 3)
		require.Equal(t, OutcomeAppChange, got)
	})

	t.Run("session fork", func(t *testing.T) {
		t.Parallel()
		got := ClassifyRewalks(expected, []LocationFingerprint{
			{SimHash: 0b11110000},
			{SimHash: 0b00001111},
		}, 3)
		require.Equal(t, OutcomeSessionFork, got)
	})

	t.Run("non deterministic", func(t *testing.T) {
		t.Parallel()
		got := ClassifyRewalks(expected, []LocationFingerprint{
			{SimHash: 0b00001111},
			{SimHash: 0b111100000000}, // far from both expected and first
		}, 3)
		require.Equal(t, OutcomeNonDeterministic, got)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, OutcomeNonDeterministic, ClassifyRewalks(expected, nil, 3))
	})
}
