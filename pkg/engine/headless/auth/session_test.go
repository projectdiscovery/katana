package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCookieStateChanged(t *testing.T) {
	t.Run("added cookie proves a session change", func(t *testing.T) {
		require.True(t, CookieStateChanged(
			CookieState{"csrf": "before"},
			CookieState{"csrf": "before", "session": "authenticated"},
		))
	})

	t.Run("changed cookie proves a session change", func(t *testing.T) {
		require.True(t, CookieStateChanged(
			CookieState{"session": "anonymous"},
			CookieState{"session": "authenticated"},
		))
	})

	t.Run("unchanged anonymous cookie is not authentication", func(t *testing.T) {
		require.False(t, CookieStateChanged(
			CookieState{"csrf": "same"},
			CookieState{"csrf": "same"},
		))
	})

	t.Run("unchanged preexisting session is not replay proof", func(t *testing.T) {
		require.False(t, CookieStateChanged(
			CookieState{"session": "already-present"},
			CookieState{"session": "already-present"},
		))
	})

	t.Run("cookie deletion alone is not authentication", func(t *testing.T) {
		require.False(t, CookieStateChanged(
			CookieState{"csrf": "before"},
			CookieState{},
		))
	})
}

func TestSessionStateChanged(t *testing.T) {
	t.Run("same-origin local storage supports token login", func(t *testing.T) {
		require.True(t, SessionStateChanged(
			SessionState{Origin: "https://app.example", LocalStorage: "[]"},
			SessionState{Origin: "https://app.example", LocalStorage: `[["token","opaque"]]`},
		))
	})

	t.Run("same-origin session storage supports token login", func(t *testing.T) {
		require.True(t, SessionStateChanged(
			SessionState{Origin: "https://app.example", SessionStorage: "[]"},
			SessionState{Origin: "https://app.example", SessionStorage: `[["token","opaque"]]`},
		))
	})

	t.Run("cross-origin storage is not proof", func(t *testing.T) {
		require.False(t, SessionStateChanged(
			SessionState{Origin: "https://idp.example", LocalStorage: "[]"},
			SessionState{Origin: "https://app.example", LocalStorage: `[["theme","dark"]]`},
		))
	})

	t.Run("cookie proof works across origins", func(t *testing.T) {
		require.True(t, SessionStateChanged(
			SessionState{Origin: "https://idp.example", Cookies: CookieState{}},
			SessionState{Origin: "https://app.example", Cookies: CookieState{"app-session": "ok"}},
		))
	})
}
