package recipe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Recipe is a deterministic, replayable login interaction sequence.
// Produced by the heuristic or AI login flow and persisted to disk
// for zero-AI replay on future crawls of the same domain.
type Recipe struct {
	Domain    string     `json:"domain"`
	LoginURL  string     `json:"login_url"`
	Steps     []Step     `json:"steps"`
	Metadata  Metadata   `json:"metadata"`
	CreatedAt time.Time  `json:"created_at"`
	Version   int        `json:"version"`
	UsedCount int        `json:"used_count"`
}

// Step is a single deterministic browser action within a recipe.
// Values may contain {{username}} and {{password}} placeholders
// that are substituted at replay time.
type Step struct {
	// Action is the browser action to perform.
	// One of: "navigate", "type", "click", "select", "clear", "wait", "press_enter"
	Action string `json:"action"`

	// Selector is the CSS selector for the target element (empty for navigate/wait/press_enter).
	Selector string `json:"selector,omitempty"`

	// Value is the action payload: text to type, URL to navigate, duration to wait.
	// Supports placeholders: {{username}}, {{password}}
	Value string `json:"value,omitempty"`

	// Fallback is an alternative CSS selector if the primary one fails.
	Fallback string `json:"fallback,omitempty"`

	// Field is the semantic meaning of this step: "username", "password", "submit", "mfa_code".
	Field string `json:"field,omitempty"`
}

// Metadata stores learned facts about the login flow for session monitoring
// and quick re-identification.
type Metadata struct {
	// SessionCookie is the name of the session cookie set after login.
	SessionCookie string `json:"session_cookie,omitempty"`

	// SuccessURL is the URL the browser landed on after successful login.
	SuccessURL string `json:"success_url,omitempty"`

	// LogoutSelector is a CSS selector for the logout/signout element.
	LogoutSelector string `json:"logout_selector,omitempty"`

	// UserIndicator is a CSS selector for an element that proves the user is logged in.
	UserIndicator string `json:"user_indicator,omitempty"`

	// CSRFMechanism describes how CSRF tokens are delivered: "hidden_field", "meta_tag", "cookie".
	CSRFMechanism string `json:"csrf_mechanism,omitempty"`

	// PasswordSelector is the CSS selector for the password field, used for quick login page detection.
	PasswordSelector string `json:"password_selector,omitempty"`
}

// Store manages recipe persistence at ~/.katana/recipes/{domain}/login.json.
type Store struct {
	baseDir string
}

// NewStore creates a new recipe store. It creates the base directory if it doesn't exist.
func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("recipe store: %w", err)
	}
	baseDir := filepath.Join(home, ".katana", "recipes")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("recipe store: create dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

// NewStoreAt creates a store at a custom base directory (useful for testing).
func NewStoreAt(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("recipe store: create dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

// recipePath returns the file path for a domain's login recipe.
func (s *Store) recipePath(domain string) string {
	// Sanitize domain for filesystem use
	safeDomain := strings.ReplaceAll(domain, ":", "_")
	safeDomain = strings.ReplaceAll(safeDomain, "/", "_")
	return filepath.Join(s.baseDir, safeDomain, "login.json")
}

// Exists returns true if a recipe exists for the given domain.
func (s *Store) Exists(domain string) bool {
	_, err := os.Stat(s.recipePath(domain))
	return err == nil
}

// Get loads a recipe for the given domain. Returns an error if not found.
func (s *Store) Get(domain string) (*Recipe, error) {
	path := s.recipePath(domain)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("recipe not found for %s: %w", domain, err)
	}

	var r Recipe
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("recipe corrupted for %s: %w", domain, err)
	}
	return &r, nil
}

// Save persists a recipe to disk atomically (write to temp, rename).
func (s *Store) Save(r *Recipe) error {
	path := s.recipePath(r.Domain)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("recipe save: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("recipe save: marshal: %w", err)
	}

	// Atomic write: temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("recipe save: write: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // cleanup on failure
		return fmt.Errorf("recipe save: rename: %w", err)
	}

	return nil
}

// Delete removes a recipe for the given domain.
func (s *Store) Delete(domain string) error {
	dir := filepath.Dir(s.recipePath(domain))
	return os.RemoveAll(dir)
}

// SubstituteCredentials replaces {{username}} and {{password}} placeholders
// in a value string with actual credentials.
func SubstituteCredentials(value, username, password string) string {
	value = strings.ReplaceAll(value, "{{username}}", username)
	value = strings.ReplaceAll(value, "{{password}}", password)
	return value
}
