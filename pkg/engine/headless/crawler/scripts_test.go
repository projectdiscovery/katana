package crawler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSameScriptOrigin(t *testing.T) {
	require.True(t, sameScriptOrigin("https://cdn.example.test/a.js", "https://cdn.example.test/b.js"))
	require.False(t, sameScriptOrigin("https://cdn.example.test/a.js", "https://other.example.test/b.js"))
	require.False(t, sameScriptOrigin("https://cdn.example.test/a.js", "http://cdn.example.test/b.js"))
	require.False(t, sameScriptOrigin("not a URL", "https://cdn.example.test/b.js"))
}
