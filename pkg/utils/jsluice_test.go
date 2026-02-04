package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			name: "jquery-3.6.0.min.js",
			path: "jquery-3.6.0.min.js",
			want: true,
		},
		{
			name: "app.js",
			path: "app.js",
			want: false,
		},
		{
			name: "main.js",
			path: "main.js",
			want: false,
		},
		{
			name: "react.production.min.js",
			path: "react.production.min.js",
			want: true,
		},
		{
			name: "vue.min.js",
			path: "vue.min.js",
			want: true,
		},
		{
			name: "lodash.min.js",
			path: "lodash.min.js",
			want: true,
		},
		{
			name: "custom-app.js",
			path: "custom-app.js",
			want: false,
		},
		{
			name: "node_modules/package/index.js",
			path: "node_modules/package/index.js",
			want: true,
		},
		{
			name: "vendor/scripts.js",
			path: "vendor/scripts.js",
			want: true,
		},
		{
			name: "bootstrap.bundle.min.js",
			path: "bootstrap.bundle.min.js",
			want: true,
		},
		{
			name: "angular.min.js",
			path: "angular.min.js",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPathCommonJSLibraryFile(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractJsluiceEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		minExpected   int
		shouldContain []string
	}{
		{
			name:          "fetch call",
			input:         `fetch("/api/users")`,
			minExpected:   1,
			shouldContain: []string{"/api/users"},
		},
		{
			name:          "xhr open",
			input:         `xhr.open("GET", "/api/data")`,
			minExpected:   1,
			shouldContain: []string{"/api/data"},
		},
		{
			name:          "location assignment",
			input:         `window.location.href = "/dashboard"`,
			minExpected:   1,
			shouldContain: []string{"/dashboard"},
		},
		{
			name:          "string literal URL",
			input:         `var apiUrl = "/api/v1/endpoint"`,
			minExpected:   1,
			shouldContain: []string{"/api/v1/endpoint"},
		},
		{
			name: "multiple endpoints",
			input: `
				fetch("/api/users");
				fetch("/api/posts");
				var config = { url: "/api/config" };
			`,
			minExpected:   2,
			shouldContain: []string{"/api/users", "/api/posts"},
		},
		{
			name:          "full URL",
			input:         `fetch("https://api.example.com/data")`,
			minExpected:   1,
			shouldContain: []string{"https://api.example.com/data"},
		},
		{
			name:          "empty input",
			input:         "",
			minExpected:   0,
			shouldContain: []string{},
		},
		{
			name:          "no URLs",
			input:         "var x = 1; console.log(x);",
			minExpected:   0,
			shouldContain: []string{},
		},
		{
			name:          "window.open",
			input:         `window.open("/popup", "_blank")`,
			minExpected:   1,
			shouldContain: []string{"/popup"},
		},
		{
			name:          "location.replace",
			input:         `location.replace("/new-page")`,
			minExpected:   1,
			shouldContain: []string{"/new-page"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)
			require.GreaterOrEqual(t, len(endpoints), tt.minExpected)

			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.Endpoint
			}

			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected)
			}
		})
	}
}

func TestExtractJsluiceEndpoints_Type(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedType string
	}{
		{
			name:         "fetch type",
			input:        `fetch("/api/users")`,
			expectedType: "fetch",
		},
		{
			name:         "xhr type",
			input:        `xhr.open("GET", "/api/data")`,
			expectedType: "xhr",
		},
		{
			name:         "location type",
			input:        `window.location.href = "/page"`,
			expectedType: "location",
		},
		{
			name:         "window.open type",
			input:        `window.open("/popup")`,
			expectedType: "window.open",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)
			require.NotEmpty(t, endpoints)
			assert.Equal(t, tt.expectedType, endpoints[0].Type)
		})
	}
}

func TestExtractJsluiceEndpoints_ComplexScenarios(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain []string
	}{
		{
			name: "nested function",
			input: `
				function outer() {
					function inner() {
						fetch("/api/inner");
					}
					fetch("/api/outer");
				}
			`,
			shouldContain: []string{"/api/inner", "/api/outer"},
		},
		{
			name: "arrow function",
			input: `
				const loadData = () => {
					fetch("/api/arrow");
				};
			`,
			shouldContain: []string{"/api/arrow"},
		},
		{
			name: "object with URLs",
			input: `
				const config = {
					apiUrl: "/api/config",
					imageUrl: "/images/logo.png"
				};
			`,
			shouldContain: []string{"/api/config", "/images/logo.png"},
		},
		{
			name: "jQuery ajax",
			input: `
				$.ajax({
					url: "/api/jquery",
					method: "POST"
				});
			`,
			shouldContain: []string{"/api/jquery"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)
			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.Endpoint
			}

			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected)
			}
		})
	}
}

func BenchmarkExtractJsluiceEndpoints(b *testing.B) {
	jsCode := `
		const API_URL = '/api/v1';
		fetch(API_URL + '/users');
		fetch('/api/posts');
		window.location.href = '/dashboard';
		$.ajax({ url: '/api/data' });
		xhr.open('GET', '/api/config');
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractJsluiceEndpoints(jsCode)
	}
}
