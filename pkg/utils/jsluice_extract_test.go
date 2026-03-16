package utils

import (
	"testing"
)

func TestExtractJsluiceEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantURLs map[string]string // endpoint -> type
	}{
		{
			name:  "string literal URL",
			input: `var url = "/api/v1/users";`,
			wantURLs: map[string]string{
				"/api/v1/users": "stringLiteral",
			},
		},
		{
			name:  "fetch call",
			input: `fetch("/api/data");`,
			wantURLs: map[string]string{
				"/api/data": "fetch",
			},
		},
		{
			name:  "window.open",
			input: `window.open("https://example.com/page");`,
			wantURLs: map[string]string{
				"https://example.com/page": "window.open",
			},
		},
		{
			name:  "location assignment",
			input: `location.href = "/redirect/path";`,
			wantURLs: map[string]string{
				"/redirect/path": "locationAssignment",
			},
		},
		{
			name:  "location.replace",
			input: `window.location.replace("/new/page");`,
			wantURLs: map[string]string{
				"/new/page": "locationReplacement",
			},
		},
		{
			name:  "jQuery get",
			input: `$.get("/api/items");`,
			wantURLs: map[string]string{
				"/api/items": "$.get",
			},
		},
		{
			name:  "multiple URLs",
			input: `fetch("/api/a"); var x = "/api/b.json"; window.open("https://test.example.com/c");`,
			wantURLs: map[string]string{
				"/api/a":                       "fetch",
				"/api/b.json":                  "stringLiteral",
				"https://test.example.com/c": "window.open",
			},
		},
		{
			name:     "no URLs in plain code",
			input:    `var x = 42; console.log("hello world");`,
			wantURLs: map[string]string{},
		},
		{
			name:     "data: scheme filtered",
			input:    `var x = "data:image/png;base64,abc";`,
			wantURLs: map[string]string{},
		},
		{
			name:  "template literal URL",
			input: "var url = `/api/v2/items`;",
			wantURLs: map[string]string{
				"/api/v2/items": "stringLiteral",
			},
		},
		{
			name:  "nested function with URL",
			input: `function loadData() { return fetch("/api/nested/endpoint"); }`,
			wantURLs: map[string]string{
				"/api/nested/endpoint": "fetch",
			},
		},
		{
			name:     "fallback regex on invalid JS",
			input:    `this is not valid javascript but has "/api/fallback" in it`,
			wantURLs: map[string]string{"/api/fallback": "regex"},
		},
		{
			name:  "xhr.open with dynamic method",
			input: `var method = "POST"; xhr.open(method, "/api/submit");`,
			wantURLs: map[string]string{
				"/api/submit": "XMLHttpRequest.open",
			},
		},
		{
			name:  "$.ajax with object literal",
			input: `$.ajax({url: "/api/login", method: "POST"});`,
			wantURLs: map[string]string{
				"/api/login": "$.ajax",
			},
		},
		{
			name:  "$.ajax with string argument",
			input: `$.ajax("/api/simple");`,
			wantURLs: map[string]string{
				"/api/simple": "$.ajax",
			},
		},
		{
			name:  "template literal with dynamic host extracts path",
			input: "var url = `https://${host}/api/data`;",
			wantURLs: map[string]string{
				"/api/data": "stringLiteral",
			},
		},
		{
			name:  "class expression with URL in method",
			input: `var C = class { loadData() { fetch("/api/class-endpoint"); } };`,
			wantURLs: map[string]string{
				"/api/class-endpoint": "fetch",
			},
		},
		{
			name:  "jQuery.ajax with object literal",
			input: `jQuery.ajax({url: "/api/jquery-login"});`,
			wantURLs: map[string]string{
				"/api/jquery-login": "$.ajax",
			},
		},
		{
			name:  "$.post call",
			input: `$.post("/api/update");`,
			wantURLs: map[string]string{
				"/api/update": "$.post",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)
			found := make(map[string]string)
			for _, ep := range endpoints {
				found[ep.Endpoint] = ep.Type
			}

			for wantURL, wantType := range tt.wantURLs {
				gotType, ok := found[wantURL]
				if !ok {
					t.Errorf("expected URL %q (type %s) not found in results: %+v", wantURL, wantType, endpoints)
					continue
				}
				if gotType != wantType {
					t.Errorf("URL %q: got type %q, want %q", wantURL, gotType, wantType)
				}
			}

			if len(tt.wantURLs) == 0 && len(endpoints) > 0 {
				t.Errorf("expected no URLs but got %+v", endpoints)
			}
		})
	}
}

func TestMaybeURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/api/v1/users", true},
		{"https://example.com/path", true},
		{"hello world", false},
		{"not-a-url", false},
		{"/path/to/file.js", true},
		{"example.com/path?key=val", true},
		{"/APP.JS", true},
		{"/path/to/FILE.CSS", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := maybeURL(tt.input); got != tt.want {
				t.Errorf("maybeURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
