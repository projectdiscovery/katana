package recipe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_SaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	require.NoError(t, err)

	r := &Recipe{
		Domain:   "example.com",
		LoginURL: "https://example.com/login",
		Steps: []Step{
			{Action: "clear", Selector: "#email", Field: "username"},
			{Action: "type", Selector: "#email", Value: "{{username}}", Field: "username"},
			{Action: "clear", Selector: "#password", Field: "password"},
			{Action: "type", Selector: "#password", Value: "{{password}}", Field: "password"},
			{Action: "click", Selector: "button[type=submit]", Field: "submit"},
			{Action: "wait", Value: "navigation"},
		},
		Metadata: Metadata{
			SessionCookie:  "session_id",
			SuccessURL:     "https://example.com/dashboard",
			LogoutSelector: `a[href="/logout"]`,
		},
		CreatedAt: time.Now(),
		Version:   1,
	}

	// Save
	err = store.Save(r)
	require.NoError(t, err)

	// Verify file exists
	assert.True(t, store.Exists("example.com"))
	assert.False(t, store.Exists("nonexistent.com"))

	// Get
	loaded, err := store.Get("example.com")
	require.NoError(t, err)
	assert.Equal(t, r.Domain, loaded.Domain)
	assert.Equal(t, r.LoginURL, loaded.LoginURL)
	assert.Equal(t, len(r.Steps), len(loaded.Steps))
	assert.Equal(t, r.Steps[1].Value, loaded.Steps[1].Value)
	assert.Equal(t, r.Metadata.SessionCookie, loaded.Metadata.SessionCookie)
	assert.Equal(t, r.Metadata.LogoutSelector, loaded.Metadata.LogoutSelector)
}

func TestStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	require.NoError(t, err)

	_, err = store.Get("nonexistent.com")
	assert.Error(t, err)
}

func TestStore_SaveOverwrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	require.NoError(t, err)

	r1 := &Recipe{Domain: "example.com", Version: 1, CreatedAt: time.Now()}
	r2 := &Recipe{Domain: "example.com", Version: 2, CreatedAt: time.Now()}

	require.NoError(t, store.Save(r1))
	require.NoError(t, store.Save(r2))

	loaded, err := store.Get("example.com")
	require.NoError(t, err)
	assert.Equal(t, 2, loaded.Version)
}

func TestStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	require.NoError(t, err)

	r := &Recipe{Domain: "example.com", Version: 1, CreatedAt: time.Now()}
	require.NoError(t, store.Save(r))
	assert.True(t, store.Exists("example.com"))

	require.NoError(t, store.Delete("example.com"))
	assert.False(t, store.Exists("example.com"))
}

func TestStore_DomainSanitization(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	require.NoError(t, err)

	// Domain with port should be sanitized (colons not allowed in Windows paths)
	r := &Recipe{Domain: "example.com:8080", Version: 1, CreatedAt: time.Now()}
	require.NoError(t, store.Save(r))
	assert.True(t, store.Exists("example.com:8080"))

	// Verify the directory name uses underscore
	entries, _ := os.ReadDir(dir)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, "example.com_8080", entries[0].Name())
}

func TestStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStoreAt(dir)
	require.NoError(t, err)

	r := &Recipe{Domain: "example.com", Version: 1, CreatedAt: time.Now()}
	require.NoError(t, store.Save(r))

	// No .tmp files should be left behind
	pattern := filepath.Join(dir, "example.com", "*.tmp")
	matches, _ := filepath.Glob(pattern)
	assert.Empty(t, matches)
}

func TestSubstituteCredentials(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		user     string
		pass     string
		expected string
	}{
		{"username only", "{{username}}", "admin", "secret", "admin"},
		{"password only", "{{password}}", "admin", "secret", "secret"},
		{"both", "{{username}}:{{password}}", "admin", "secret", "admin:secret"},
		{"no placeholders", "static-value", "admin", "secret", "static-value"},
		{"empty value", "", "admin", "secret", ""},
		{"special chars in password", "{{password}}", "admin", "p@ss!w0rd#123", "p@ss!w0rd#123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteCredentials(tt.value, tt.user, tt.pass)
			assert.Equal(t, tt.expected, got)
		})
	}
}
