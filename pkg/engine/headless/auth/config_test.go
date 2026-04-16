package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAuthConfig_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")

	content := `
login_url: https://example.com/login
credentials:
  username: admin@example.com
  password: secret123
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	config, err := LoadAuthConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/login", config.LoginURL)
	assert.Equal(t, "admin@example.com", config.Credentials.Username)
	assert.Equal(t, "secret123", config.Credentials.Password)
}

func TestLoadAuthConfig_CredentialsOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")

	content := `
credentials:
  username: user
  password: pass
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	config, err := LoadAuthConfig(path)
	require.NoError(t, err)
	assert.Empty(t, config.LoginURL) // auto-detect mode
	assert.Equal(t, "user", config.Credentials.Username)
	assert.Equal(t, "pass", config.Credentials.Password)
}

func TestLoadAuthConfig_NoCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")

	content := `login_url: https://example.com/login`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	_, err := LoadAuthConfig(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credentials are required")
}

func TestLoadAuthConfig_FileNotFound(t *testing.T) {
	_, err := LoadAuthConfig("/nonexistent/path/auth.yaml")
	assert.Error(t, err)
}

func TestLoadAuthConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`{invalid yaml[[`), 0o644))

	_, err := LoadAuthConfig(path)
	assert.Error(t, err)
}
