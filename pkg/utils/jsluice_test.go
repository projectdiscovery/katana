package utils

import (
	"testing"
)

func TestIsPathCommonJSLibraryFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "jquery.js",
			args: args{
				path: "jquery.js",
			},
			want: true,
		},
		{
			name: "app.js",
			args: args{
				path: "app.js",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathCommonJSLibraryFile(tt.args.path); got != tt.want {
				t.Errorf("IsPathCommonJSLibraryFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractJsluiceEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURLs []string
	}{
		{
			name:     "fetch call",
			input:    `fetch("/api/users")`,
			wantURLs: []string{"/api/users"},
		},
		{
			name:     "window.open",
			input:    `window.open("https://example.com/page")`,
			wantURLs: []string{"https://example.com/page"},
		},
		{
			name:     "location.href assignment",
			input:    `document.location.href = "/login"`,
			wantURLs: []string{"/login"},
		},
		{
			name:     "XMLHttpRequest",
			input:    `xhr.open("GET", "/api/data")`,
			wantURLs: []string{"/api/data"},
		},
		{
			name:     "variable assignment",
			input:    `var apiUrl = "/api/v1/users"`,
			wantURLs: []string{"/api/v1/users"},
		},
		{
			name:     "multiple URLs",
			input:    `fetch("/api/users"); fetch("/api/posts")`,
			wantURLs: []string{"/api/users", "/api/posts"},
		},
		{
			name:     "WebSocket",
			input:    `new WebSocket("wss://example.com/ws")`,
			wantURLs: []string{"wss://example.com/ws"},
		},
		{
			name:     "URL constructor",
			input:    `new URL("https://example.com/path")`,
			wantURLs: []string{"https://example.com/path"},
		},
		{
			name:     "jQuery ajax",
			input:    `$.ajax({url: "/api/data", method: "GET"})`,
			wantURLs: []string{"/api/data"},
		},
		{
			name:     "string literal URL",
			input:    `const url = "https://api.example.com/v1"`,
			wantURLs: []string{"https://api.example.com/v1"},
		},
		{
			name:     "no URLs",
			input:    `const x = 5; console.log("hello")`,
			wantURLs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)

			// Create a map of found URLs for easier checking
			foundURLs := make(map[string]bool)
			for _, ep := range endpoints {
				foundURLs[ep.Endpoint] = true
			}

			// Check that all expected URLs are found
			for _, wantURL := range tt.wantURLs {
				if !foundURLs[wantURL] {
					t.Errorf("ExtractJsluiceEndpoints() missing expected URL %q, got %v", wantURL, endpoints)
				}
			}

			// Check that no unexpected URLs were extracted
			if len(endpoints) != len(tt.wantURLs) {
				t.Errorf("ExtractJsluiceEndpoints() returned %d endpoints, want %d", len(endpoints), len(tt.wantURLs))
			}
		})
	}
}

func TestExtractJsluiceEndpoints_ComplexJS(t *testing.T) {
	// Test with more complex JavaScript code
	js := `
		function loadData() {
			fetch("/api/users")
				.then(response => response.json())
				.then(data => {
					window.location.href = "/dashboard";
				});
		}

		class ApiClient {
			constructor() {
				this.baseUrl = "https://api.example.com";
			}

			async getUsers() {
				return fetch(this.baseUrl + "/users");
			}
		}

		const socket = new WebSocket("wss://realtime.example.com/socket");
	`

	endpoints := ExtractJsluiceEndpoints(js)

	expectedURLs := []string{
		"/api/users",
		"/dashboard",
		"https://api.example.com",
		"EXPR/users",
		"/users",
		"wss://realtime.example.com/socket",
	}

	foundURLs := make(map[string]bool)
	for _, ep := range endpoints {
		foundURLs[ep.Endpoint] = true
	}

	for _, wantURL := range expectedURLs {
		if !foundURLs[wantURL] {
			t.Errorf("Missing expected URL %q in complex JS test", wantURL)
		}
	}

	if len(endpoints) != len(expectedURLs) {
		t.Errorf("ExtractJsluiceEndpoints() returned %d endpoints, want %d", len(endpoints), len(expectedURLs))
	}
}
