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
		name        string
		input       string
		wantURLs    []string // URLs that MUST be present
		excludeURLs []string // URLs that MUST NOT be present
	}{
		{
			name:     "fetch single quotes",
			input:    `fetch('/api/data')`,
			wantURLs: []string{"/api/data"},
		},
		{
			name:     "fetch double quotes",
			input:    `fetch("/api/users")`,
			wantURLs: []string{"/api/users"},
		},
		{
			name:     "fetch absolute URL",
			input:    `fetch('https://example.com/api')`,
			wantURLs: []string{"https://example.com/api"},
		},
		{
			name:     "XHR open",
			input:    `xhr.open('GET', '/api/items')`,
			wantURLs: []string{"/api/items"},
		},
		{
			name:     "location assign",
			input:    `location.assign('/login')`,
			wantURLs: []string{"/login"},
		},
		{
			name:     "location replace",
			input:    `location.replace('/logout')`,
			wantURLs: []string{"/logout"},
		},
		{
			name:     "location href assignment",
			input:    `location.href = '/dashboard'`,
			wantURLs: []string{"/dashboard"},
		},
		{
			name:     "window open",
			input:    `window.open('/popup')`,
			wantURLs: []string{"/popup"},
		},
		{
			name:     "jQuery get",
			input:    `$.get('/api/list', function(data) {})`,
			wantURLs: []string{"/api/list"},
		},
		{
			name:     "jQuery post",
			input:    `$.post('/api/submit', {data: 1})`,
			wantURLs: []string{"/api/submit"},
		},
		{
			name:     "string literal absolute path",
			input:    `var url = '/api/endpoint';`,
			wantURLs: []string{"/api/endpoint"},
		},
		{
			name:     "string literal relative path",
			input:    `var url = './assets/data.json';`,
			wantURLs: []string{"./assets/data.json"},
		},
		{
			name:     "string literal https URL",
			input:    `var base = "https://api.example.com/v1";`,
			wantURLs: []string{"https://api.example.com/v1"},
		},
		{
			name:        "skip line comment — URL must not be extracted from comment",
			input:       "// fetch(\"/api/secret\")\nfetch(\"/api/real\")",
			wantURLs:    []string{"/api/real"},
			excludeURLs: []string{"/api/secret"},
		},
		{
			name:        "skip block comment — URL must not be extracted from comment",
			input:       "/* fetch('/api/secret') */\nfetch('/api/real')",
			wantURLs:    []string{"/api/real"},
			excludeURLs: []string{"/api/secret"},
		},
		{
			name:        "skip multi-line block comment",
			input:       "/*\n * Old endpoint: /api/v1/users\n */\nfetch('/api/v2/users')",
			wantURLs:    []string{"/api/v2/users"},
			excludeURLs: []string{"/api/v1/users"},
		},
		{
			name:        "string literal after comment on same line",
			input:       "var x = 1; // old: /api/old\nvar url = '/api/new';",
			wantURLs:    []string{"/api/new"},
			excludeURLs: []string{"/api/old"},
		},
		{
			name:     "no URLs in non-URL strings",
			input:    `var x = "hello world"; var y = 'foo';`,
			wantURLs: nil,
		},
		{
			name:     "skip w3.org noise",
			input:    `var ns = "http://www.w3.org/1999/xhtml";`,
			wantURLs: nil,
		},
		{
			name:     "skip data URI",
			input:    `var img = "data:image/png;base64,abc";`,
			wantURLs: nil,
		},
		{
			name:        "template literal with fetch",
			input:       "fetch(`/api/users/${id}`)",
			wantURLs:    []string{"/api/users/${id}"},
			excludeURLs: []string{"/api/usersEXPR"},
		},
		{
			name:     "protocol-relative URL",
			input:    `var cdn = "//cdn.example.com/script.js";`,
			wantURLs: []string{"//cdn.example.com/script.js"},
		},
		{
			name:     "plain relative path",
			input:    `fetch("api/v1/users")`,
			wantURLs: []string{"api/v1/users"},
		},
		{
			name:     "plain relative path in string literal",
			input:    `var endpoint = "admin/settings/config";`,
			wantURLs: []string{"admin/settings/config"},
		},
		{
			name:        "MIME types not extracted as endpoints",
			input:       `var headers = {"Content-Type": "application/json", "Accept": "text/html"};`,
			wantURLs:    nil,
			excludeURLs: []string{"application/json", "text/html"},
		},
		{
			name:        "MIME type with params not extracted",
			input:       `var ct = "text/html; charset=utf-8";`,
			wantURLs:    nil,
			excludeURLs: []string{"text/html; charset=utf-8", "text/html"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)
			got := make(map[string]bool, len(endpoints))
			for _, ep := range endpoints {
				got[ep.Endpoint] = true
			}
			// Check all required URLs are present
			for _, want := range tt.wantURLs {
				if !got[want] {
					t.Errorf("ExtractJsluiceEndpoints() missing expected URL %q; got %v", want, endpoints)
				}
			}
			// Check that excluded URLs are absent
			for _, excl := range tt.excludeURLs {
				if got[excl] {
					t.Errorf("ExtractJsluiceEndpoints() incorrectly extracted URL from comment: %q", excl)
				}
			}
		})
	}
}

func TestStripJSComments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "line comment removed",
			input: "var x = 1; // this is a comment\nvar y = 2;",
			want:  "var x = 1; \nvar y = 2;",
		},
		{
			name:  "block comment removed",
			input: "var x = /* comment */ 1;",
			want:  "var x =  1;",
		},
		{
			name:  "string literal preserved",
			input: `var url = "http://example.com";`,
			want:  `var url = "http://example.com";`,
		},
		{
			name:  "comment-looking content inside string preserved",
			input: `var s = "// not a comment";`,
			want:  `var s = "// not a comment";`,
		},
		{
			name:  "block comment inside string preserved",
			input: `var s = "/* not a comment */";`,
			want:  `var s = "/* not a comment */";`,
		},
		{
			name:  "unterminated block comment consumed",
			input: "var x = 1; /* unterminated",
			want:  "var x = 1; ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripJSComments(tt.input); got != tt.want {
				t.Errorf("stripJSComments() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsLikelyURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/api/data", true},
		{"https://example.com", true},
		{"http://example.com/path", true},
		{"//cdn.example.com/script.js", true},
		{"./relative/path", true},
		{"../parent/path", true},
		{"api/v1/users", true},
		{"assets/js/app.js", true},
		{"hello world", false},
		{"data:image/png;base64,abc", false},
		{"http://www.w3.org/1999/xhtml", false},
		{"", false},
		{"/", false},
		// MIME types must be rejected (false positives)
		{"application/json", false},
		{"application/xml", false},
		{"text/plain", false},
		{"text/html", false},
		{"image/png", false},
		{"image/jpeg", false},
		{"audio/mpeg", false},
		{"video/mp4", false},
		{"font/woff2", false},
		{"multipart/form-data", false},
		{"application/octet-stream", false},
		{"text/html; charset=utf-8", false},
		{"application/x-www-form-urlencoded", false},
		// MIME-like but with dots/paths should still be accepted
		{"application/api.v1.json", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isLikelyURL(tt.input); got != tt.want {
				t.Errorf("isLikelyURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
