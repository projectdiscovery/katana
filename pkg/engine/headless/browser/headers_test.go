package browser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCookieHeaderScopesToRootURL(t *testing.T) {
	cookies := parseCookieHeader("session=abc=123; theme=dark; invalid", "https://app.example.test/dashboard")
	require.Len(t, cookies, 2)
	require.Equal(t, "session", cookies[0].Name)
	require.Equal(t, "abc=123", cookies[0].Value)
	require.Equal(t, "https://app.example.test/dashboard", cookies[0].URL)
	require.Equal(t, "/", cookies[0].Path)
	require.Equal(t, "theme", cookies[1].Name)
}
