//go:build windows || 386 || nocgo

package utils

import (
	"testing"
)

func TestExtractJsluiceEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // expected endpoints (we check presence, not exact match)
	}{
		{
			name:     "fetch call",
			input:    `fetch("/api/users")`,
			expected: []string{"/api/users"},
		},
		{
			name:     "multiple endpoints",
			input:    `fetch("/api/users"); fetch("https://example.com/data");`,
			expected: []string{"/api/users", "https://example.com/data"},
		},
		{
			name:     "window.open",
			input:    `window.open("/login")`,
			expected: []string{"/login"},
		},
		{
			name:     "document.location assignment",
			input:    `document.location = "/dashboard"`,
			expected: []string{"/dashboard"},
		},
		{
			name:     "ajax call with url property",
			input:    `$.ajax({ url: "/api/submit", method: "POST" })`,
			expected: []string{"/api/submit"},
		},
		{
			name:     "variable assignment",
			input:    `const apiUrl = "/api/v1/endpoint";`,
			expected: []string{"/api/v1/endpoint"},
		},
		{
			name:     "nested function",
			input:    `function getData() { return fetch("/data.json"); }`,
			expected: []string{"/data.json"},
		},
		{
			name:     "arrow function",
			input:    `const getUser = () => fetch("/api/user");`,
			expected: []string{"/api/user"},
		},
		{
			name:     "absolute URL",
			input:    `const url = "https://api.example.com/v1/users";`,
			expected: []string{"https://api.example.com/v1/users"},
		},
		{
			name:     "relative path",
			input:    `import config from "./config.json";`,
			expected: []string{"./config.json"},
		},
		{
			name:  "should skip non-URL strings",
			input: `const name = "hello world"; const count = "123";`,
			expected: []string{}, // no URLs expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)

			// Create a map for easy lookup
			found := make(map[string]bool)
			for _, ep := range endpoints {
				found[ep.Endpoint] = true
			}

			// Check that all expected endpoints are found
			for _, expected := range tt.expected {
				if !found[expected] {
					t.Errorf("Expected endpoint %q not found. Got: %v", expected, endpoints)
				}
			}

			// For "should skip" tests, verify no endpoints found
			if len(tt.expected) == 0 && len(endpoints) > 0 {
				t.Errorf("Expected no endpoints, but found: %v", endpoints)
			}
		})
	}
}

func TestIsPathCommonJSLibraryFile(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "jquery.js",
			path: "jquery.js",
			want: true,
		},
		{
			name: "app.js",
			path: "app.js",
			want: false,
		},
		{
			name: "lodash.min.js",
			path: "lodash.min.js",
			want: true,
		},
		{
			name: "react.production.js",
			path: "react.production.js",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathCommonJSLibraryFile(tt.path); got != tt.want {
				t.Errorf("IsPathCommonJSLibraryFile() = %v, want %v", got, tt.want)
			}
		})
	}
}
