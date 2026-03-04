//go:build !jsluice || 386 || windows

package utils

import (
	"testing"
)

func TestExtractJsluiceEndpoints_Stub(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURLs []string
	}{
		{
			name:  "simple URL in quotes",
			input: `var url = "https://example.com/api/users";`,
			wantURLs: []string{
				"https://example.com/api/users",
			},
		},
		{
			name:  "relative path",
			input: `fetch("/api/data")`,
			wantURLs: []string{
				"/api/data",
			},
		},
		{
			name:  "multiple URLs",
			input: `const api1 = "https://api.example.com/v1"; const api2 = "/api/v2/users";`,
			wantURLs: []string{
				"https://api.example.com/v1",
				"/api/v2/users",
			},
		},
		{
			name:     "no URLs",
			input:    `var x = 123; console.log("hello");`,
			wantURLs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)
			
			if len(tt.wantURLs) == 0 && len(endpoints) == 0 {
				return
			}

			if len(endpoints) != len(tt.wantURLs) {
				t.Errorf("ExtractJsluiceEndpoints() got %d endpoints, want %d", len(endpoints), len(tt.wantURLs))
				return
			}

			// Check if all expected URLs are found
			found := make(map[string]bool)
			for _, ep := range endpoints {
				found[ep.Endpoint] = true
			}

			for _, wantURL := range tt.wantURLs {
				if !found[wantURL] {
					t.Errorf("ExtractJsluiceEndpoints() missing expected URL: %s", wantURL)
				}
			}
		})
	}
}

func TestIsValidEndpoint(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid URL",
			input: "https://example.com/api",
			want:  true,
		},
		{
			name:  "valid path",
			input: "/api/users",
			want:  true,
		},
		{
			name:  "too short",
			input: "/",
			want:  false,
		},
		{
			name:  "data URI",
			input: "data:image/png;base64,iVBORw0KGgo...",
			want:  false,
		},
		{
			name:  "javascript URI",
			input: "javascript:void(0)",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidEndpoint(tt.input); got != tt.want {
				t.Errorf("isValidEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}
