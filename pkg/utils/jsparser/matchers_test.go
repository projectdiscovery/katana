package jsparser

import "testing"

func TestIsPathCommonJSLibraryFile(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		match bool
	}{
		{name: "jquery", path: "/static/js/jquery.min.js", match: true},
		{name: "react", path: "/assets/react.production.min.js", match: true},
		{name: "custom app", path: "/assets/app.bundle.js", match: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPathCommonJSLibraryFile(tt.path)
			if got != tt.match {
				t.Fatalf("expected %v, got %v", tt.match, got)
			}
		})
	}
}

func TestIsLikelyEndpoint(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "https://example.com/api/users", want: true},
		{value: "wss://example.com/socket", want: true},
		{value: "/api/users", want: true},
		{value: "./graphql", want: true},
		{value: "../v1/login", want: true},
		{value: "api/users", want: true},
		{value: "rest/user.json", want: true},
		{value: "graphql", want: true},
		{value: "data:text/plain,hello", want: false},
		{value: "javascript:void(0)", want: false},
		{value: "#", want: false},
		{value: "hello world", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := IsLikelyEndpoint(tt.value)
			if got != tt.want {
				t.Fatalf("expected %v, got %v for %q", tt.want, got, tt.value)
			}
		})
	}
}

func TestClassifyEndpoint(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "https://example.com/api", want: "url"},
		{value: "wss://example.com/socket", want: "websocket"},
		{value: "/api/users", want: "path"},
		{value: "api/users", want: "path"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := ClassifyEndpoint(tt.value)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestDedupeEndpoints(t *testing.T) {
	input := []Endpoint{
		{Endpoint: "/api/users", Type: "path"},
		{Endpoint: "/api/users", Type: "path"},
		{Endpoint: " /api/users ", Type: "path"},
		{Endpoint: "https://example.com/api", Type: "url"},
	}

	got := DedupeEndpoints(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 endpoints, got %d: %#v", len(got), got)
	}
}
