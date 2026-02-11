package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
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
		input         string
		wantEndpoints []string
		wantTypes     []string
		wantMinCount  int
	}{
		{
			name:          "fetch call",
			input:         `fetch("/api/users")`,
			wantEndpoints: []string{"/api/users"},
			wantTypes:     []string{"fetch"},
		},
		{
			name:          "fetch with full URL",
			input:         `fetch("https://example.com/api/data")`,
			wantEndpoints: []string{"https://example.com/api/data"},
			wantTypes:     []string{"fetch"},
		},
		{
			name:          "XMLHttpRequest open",
			input:         `var xhr = new XMLHttpRequest(); xhr.open("GET", "/api/data");`,
			wantEndpoints: []string{"/api/data"},
		},
		{
			name:          "window.open",
			input:         `window.open("https://example.com/page")`,
			wantEndpoints: []string{"https://example.com/page"},
		},
		{
			name:          "string variable with URL path",
			input:         `var url = "/api/endpoint";`,
			wantEndpoints: []string{"/api/endpoint"},
			wantTypes:     []string{"string"},
		},
		{
			name:          "location.href assignment",
			input:         `window.location.href = "/dashboard/settings";`,
			wantEndpoints: []string{"/dashboard/settings"},
			wantTypes:     []string{"assignment"},
		},
		{
			name:          "img src assignment",
			input:         `document.getElementById("img").src = "/images/logo.png";`,
			wantEndpoints: []string{"/images/logo.png"},
		},
		{
			name:          "multiple endpoints",
			input:         `var a = "/api/users"; var b = "/api/posts"; fetch("/api/comments");`,
			wantEndpoints: []string{"/api/users", "/api/posts", "/api/comments"},
		},
		{
			name:          "object literal with URLs",
			input:         `var config = { apiUrl: "/api/v2/data", callback: "/auth/callback" };`,
			wantEndpoints: []string{"/api/v2/data", "/auth/callback"},
		},
		{
			name:          "array of URLs",
			input:         `var urls = ["/api/first", "/api/second"];`,
			wantEndpoints: []string{"/api/first", "/api/second"},
		},
		{
			name:          "template literal with URL",
			input:         "var url = `/api/users`;",
			wantEndpoints: []string{"/api/users"},
			wantTypes:     []string{"template"},
		},
		{
			name:          "function body with URL",
			input:         `function getData() { return fetch("/api/resources"); }`,
			wantEndpoints: []string{"/api/resources"},
		},
		{
			name:          "arrow function with URL",
			input:         `const getData = () => fetch("/api/items");`,
			wantEndpoints: []string{"/api/items"},
		},
		{
			name:          "conditional with URLs",
			input:         `var url = condition ? "/api/path-a" : "/api/path-b";`,
			wantEndpoints: []string{"/api/path-a", "/api/path-b"},
		},
		{
			name:          "no URLs found",
			input:         `var x = 42; var y = "hello world";`,
			wantEndpoints: nil,
		},
		{
			name:          "empty input",
			input:         ``,
			wantEndpoints: nil,
		},
		{
			name:          "deduplication",
			input:         `fetch("/api/users"); fetch("/api/users");`,
			wantEndpoints: []string{"/api/users"},
		},
		{
			name:         "malformed JS falls back to regex",
			input:        `this is not valid javascript but has '/api/endpoint.js' in it`,
			wantMinCount: 1,
		},
		{
			name:          "jQuery ajax",
			input:         `$.ajax({ url: "/api/search", method: "GET" });`,
			wantEndpoints: []string{"/api/search"},
		},
		{
			name:          "axios get",
			input:         `axios.get("/api/profile");`,
			wantEndpoints: []string{"/api/profile"},
		},
		{
			name:          "skip data URIs",
			input:         `var img = "data:image/png;base64,abc123";`,
			wantEndpoints: nil,
		},
		{
			name:          "skip javascript URIs",
			input:         `var x = "javascript:void(0)";`,
			wantEndpoints: nil,
		},
		{
			name:          "skip mailto URIs",
			input:         `var x = "mailto:test@example.com";`,
			wantEndpoints: nil,
		},
		{
			name:          "URL with query parameters",
			input:         `fetch("/api/search?q=test&page=1")`,
			wantEndpoints: []string{"/api/search?q=test&page=1"},
		},
		{
			name:          "ES6 import stripped gracefully",
			input:         "import React from 'react';\nvar url = '/api/data';",
			wantEndpoints: []string{"/api/data"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)

			if tt.wantMinCount > 0 {
				require.GreaterOrEqual(t, len(endpoints), tt.wantMinCount,
					"expected at least %d endpoints", tt.wantMinCount)
				return
			}

			if tt.wantEndpoints == nil {
				require.Empty(t, endpoints, "expected no endpoints")
				return
			}

			gotEndpoints := make([]string, len(endpoints))
			for i, ep := range endpoints {
				gotEndpoints[i] = ep.Endpoint
			}
			require.ElementsMatch(t, tt.wantEndpoints, gotEndpoints)

			// Verify types if specified
			if tt.wantTypes != nil {
				gotTypes := make([]string, len(endpoints))
				for i, ep := range endpoints {
					gotTypes[i] = ep.Type
				}
				require.ElementsMatch(t, tt.wantTypes, gotTypes)
			}
		})
	}
}

func TestIsURLLike(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/api/users", true},
		{"/path/to/resource", true},
		{"https://example.com/page", true},
		{"http://example.com/api", true},
		{"/api/search?q=test", true},
		{"", false},
		{"/", false},
		{"#", false},
		{".", false},
		{"..", false},
		{"data:image/png;base64,abc", false},
		{"javascript:void(0)", false},
		{"mailto:test@example.com", false},
		{"blob:http://example.com/uuid", false},
		{"hello world", false},
		{"just a string", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isURLLike(tt.input)
			require.Equal(t, tt.want, got, "isURLLike(%q)", tt.input)
		})
	}
}

func TestPreprocessES6(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips import statement",
			input: "import React from 'react';\nvar x = 1;",
			want:  "\nvar x = 1;",
		},
		{
			name:  "strips export default",
			input: "export default function foo() {}\nvar x = 1;",
			want:  "\nvar x = 1;",
		},
		{
			name:  "leaves normal code alone",
			input: "var x = 1;\nvar y = 2;",
			want:  "var x = 1;\nvar y = 2;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preprocessES6(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestRegexFallbackExtract(t *testing.T) {
	t.Run("extracts endpoints from malformed JS", func(t *testing.T) {
		input := `this is broken {{ but contains "/api/endpoint.js" and '/another/path.html' }}`
		endpoints := regexFallbackExtract(input)
		require.NotEmpty(t, endpoints, "expected regex fallback to find endpoints")

		found := false
		for _, ep := range endpoints {
			if ep.Endpoint == "/api/endpoint.js" || ep.Endpoint == "/another/path.html" {
				found = true
				break
			}
		}
		require.True(t, found, "expected to find at least one known endpoint")
	})

	t.Run("all results have regex type", func(t *testing.T) {
		input := `broken js with '/api/test.json' in it`
		endpoints := regexFallbackExtract(input)
		for _, ep := range endpoints {
			require.Equal(t, "regex", ep.Type, "fallback endpoints should have 'regex' type")
		}
	})
}
