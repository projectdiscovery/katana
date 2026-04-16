package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffCookies_NewCookies(t *testing.T) {
	before := []CookieEntry{
		{Name: "tracking", Value: "abc"},
	}
	after := []CookieEntry{
		{Name: "tracking", Value: "abc"},
		{Name: "session_id", Value: "xyz123"},
	}

	diff := DiffCookies(before, after)
	assert.Len(t, diff, 1)
	assert.Equal(t, "session_id", diff[0].Name)
}

func TestDiffCookies_ChangedCookie(t *testing.T) {
	before := []CookieEntry{
		{Name: "csrf", Value: "old_token"},
	}
	after := []CookieEntry{
		{Name: "csrf", Value: "new_token"},
	}

	diff := DiffCookies(before, after)
	assert.Len(t, diff, 1)
	assert.Equal(t, "new_token", diff[0].Value)
}

func TestDiffCookies_NoDiff(t *testing.T) {
	cookies := []CookieEntry{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}

	diff := DiffCookies(cookies, cookies)
	assert.Empty(t, diff)
}

func TestDetectNewSessionCookie(t *testing.T) {
	tests := []struct {
		name     string
		diff     []CookieEntry
		expected string
	}{
		{
			"prefers httpOnly cookie",
			[]CookieEntry{
				{Name: "tracking_id", Value: "abc", HTTPOnly: false},
				{Name: "session", Value: "xyz", HTTPOnly: true},
			},
			"session",
		},
		{
			"prefers session-like name",
			[]CookieEntry{
				{Name: "theme", Value: "dark"},
				{Name: "sid", Value: "abc123"},
			},
			"sid",
		},
		{
			"falls back to first",
			[]CookieEntry{
				{Name: "custom_cookie", Value: "val"},
			},
			"custom_cookie",
		},
		{
			"empty diff",
			[]CookieEntry{},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, DetectNewSessionCookie(tt.diff))
		})
	}
}

func TestIsLoginURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com/login", true},
		{"https://example.com/signin", true},
		{"https://example.com/auth/login", true},
		{"https://example.com/sso/callback", true},
		{"https://example.com/dashboard", false},
		{"https://example.com/api/users", false},
		{"https://example.com/blog/login-tips", false}, // "login" is part of a word but not a path segment
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsLoginURL(tt.url))
		})
	}
}

func TestHasLoginError(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{"invalid credentials", "Invalid credentials. Please try again.", true},
		{"wrong password", "Wrong password for this account", true},
		{"failed login", "Authentication failed. Check your email and password.", true},
		{"normal page", "Welcome to the dashboard. You are logged in.", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HasLoginError(tt.text))
		})
	}
}
