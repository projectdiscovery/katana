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
		name          string
		jsCode        string
		wantEndpoints int
		checkContains []string
	}{
		{
			name:          "simple URL",
			jsCode:        `var url = "https://example.com/api/users";`,
			wantEndpoints: 1,
			checkContains: []string{"https://example.com/api/users"},
		},
		{
			name:          "multiple URLs",
			jsCode:        `fetch("https://api.example.com/data"); var path = "/api/endpoint";`,
			wantEndpoints: 2,
			checkContains: []string{"https://api.example.com/data", "/api/endpoint"},
		},
		{
			name:          "API paths",
			jsCode:        `const apiPath = "/api/v1/users"; const another = "/api/v2/posts";`,
			wantEndpoints: 2,
			checkContains: []string{"/api/v1/users", "/api/v2/posts"},
		},
		{
			name:          "protocol-relative URL",
			jsCode:        `var cdn = "//cdn.example.com/script.js";`,
			wantEndpoints: 1,
			checkContains: []string{"//cdn.example.com/script.js"},
		},
		{
			name:          "complex JavaScript",
			jsCode:        `function getData() { return fetch("https://example.com/data"); } var config = { endpoint: "/api/config" };`,
			wantEndpoints: 2,
			checkContains: []string{"https://example.com/data", "/api/config"},
		},
		{
			name:          "backtick template strings",
			jsCode:        "const url = `https://example.com/api/v1`; const path = `/api/test`;",
			wantEndpoints: 2,
			checkContains: []string{"https://example.com/api/v1", "/api/test"},
		},
		{
			name:          "empty string",
			jsCode:        ``,
			wantEndpoints: 0,
			checkContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.jsCode)

			if len(endpoints) < len(tt.checkContains) {
				t.Errorf("ExtractJsluiceEndpoints() returned %d endpoints, expected at least %d", len(endpoints), len(tt.checkContains))
			}

			// Check if all expected endpoints are present
			found := make(map[string]bool)
			for _, ep := range endpoints {
				found[ep.Endpoint] = true
			}

			for _, expected := range tt.checkContains {
				if !found[expected] {
					t.Errorf("ExtractJsluiceEndpoints() missing expected endpoint: %s", expected)
					t.Logf("Found endpoints: %v", endpoints)
				}
			}
		})
	}
}
