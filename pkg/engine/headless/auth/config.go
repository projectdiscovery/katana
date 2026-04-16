package auth

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AuthConfig defines how the crawler should authenticate with the target application.
// Users provide credentials and optionally a login URL — the system figures out everything else.
type AuthConfig struct {
	// LoginURL is the URL of the login page.
	// If empty, the crawler auto-detects login pages during crawling.
	LoginURL string `yaml:"login_url,omitempty" json:"login_url,omitempty"`

	// Credentials contains the username and password to authenticate with.
	Credentials Credentials `yaml:"credentials" json:"credentials"`
}

// Credentials holds authentication credentials.
type Credentials struct {
	Username string `yaml:"username" json:"username"`
	Email    string `yaml:"email" json:"email,omitempty"`
	Password string `yaml:"password" json:"password"`
}

// GetUsername returns the username, falling back to email if username is empty.
func (c *Credentials) GetUsername() string {
	if c.Username != "" {
		return c.Username
	}
	return c.Email
}

// LoadAuthConfig reads and parses an authentication configuration from a YAML file.
func LoadAuthConfig(path string) (*AuthConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open auth config: %w", err)
	}
	defer f.Close()

	var config AuthConfig
	if err := yaml.NewDecoder(f).Decode(&config); err != nil {
		return nil, fmt.Errorf("decode auth config: %w", err)
	}

	if config.Credentials.GetUsername() == "" && config.Credentials.Password == "" {
		return nil, fmt.Errorf("auth config: credentials are required (set username or email + password)")
	}

	return &config, nil
}
