package utils

import (
	"strings"
	"testing"
)

// realWorldJS simulates a realistic JavaScript file with various endpoint patterns.
var realWorldJS = `
// API client module
const BASE_URL = "https://api.example.com/v2";

function loadUserProfile(userId) {
    fetch("/api/users/" + userId + "/profile")
        .then(r => r.json())
        .then(data => {
            window.location.href = "/dashboard";
        });
}

async function submitForm(data) {
    const response = await fetch("/api/forms/submit", {
        method: "POST",
        body: JSON.stringify(data)
    });
    return response.json();
}

// jQuery AJAX calls
$(document).ready(function() {
    $.get("/api/notifications", function(data) {
        console.log(data);
    });
    $.post("/api/analytics/track", { event: "pageview" });
    $.ajax({ url: "/api/settings", method: "GET" });
    $.getJSON("/api/config.json", function(config) {
        window.open("/external/redirect?url=" + config.url);
    });
});

// XHR usage
var xhr = new XMLHttpRequest();
xhr.open("GET", "/api/legacy/data");
xhr.send();

// Dynamic imports
import("/modules/lazy-component.js").then(m => m.default());

// Location assignments
location.href = "/auth/login";
location.pathname = "/settings/profile";
location.search = "?tab=security";
location.replace("/auth/logout");
location.assign("/home");

// Template literals with expressions
var userId = 42;
var url = ` + "`" + `/api/users/${userId}/posts` + "`" + `;
fetch(` + "`" + `https://api.example.com/v2/items/${itemId}` + "`" + `);

// Various string URLs that should be detected
var config = {
    apiEndpoint: "/api/v1/resources",
    cdnUrl: "https://cdn.example.com/assets/main.js",
    wsEndpoint: "//ws.example.com/socket",
    authCallback: "/oauth/callback",
    healthCheck: "/api/health",
};

// Things that should NOT be detected as URLs
var mimeType = "application/json";
var cssSelector = ".container > .item";
var comment = "// this is not a URL";
var dataUri = "data:image/png;base64,abc123";
var mailTo = "mailto:test@example.com";
var year = "2024-01-15";
var jsProtocol = "javascript:void(0)";
`

func TestExtractEndpoints(t *testing.T) {
	endpoints := ExtractJsluiceEndpoints(realWorldJS)
	if len(endpoints) == 0 {
		t.Fatal("expected endpoints, got none")
	}

	found := make(map[string]string)
	for _, ep := range endpoints {
		found[ep.Endpoint] = ep.Type
	}

	// Structured patterns should be detected with correct types
	mustFind := map[string]string{
		"/api/forms/submit":          "fetch",
		"/api/notifications":         "jquery",
		"/api/analytics/track":       "jquery",
		"/api/config.json":           "jquery",
		"/api/legacy/data":           "xhr",
		"/modules/lazy-component.js": "import",
		"/auth/login":                "location",
		"/settings/profile":          "location",
		"/auth/logout":               "location",
		"/home":                      "location",
	}

	for url, wantType := range mustFind {
		gotType, ok := found[url]
		if !ok {
			t.Errorf("missing endpoint: %q (expected type %q)", url, wantType)
			continue
		}
		if gotType != wantType {
			t.Errorf("endpoint %q: got type %q, want %q", url, gotType, wantType)
		}
	}

	// String-detected URLs
	mustFindStrings := []string{
		"/api/v1/resources",
		"https://cdn.example.com/assets/main.js",
		"//ws.example.com/socket",
		"/oauth/callback",
		"/api/health",
	}
	for _, url := range mustFindStrings {
		if _, ok := found[url]; !ok {
			t.Errorf("missing string endpoint: %q", url)
		}
	}

	// False positives should NOT be present
	mustNotFind := []string{
		"application/json",
		".container > .item",
		"data:image/png;base64,abc123",
		"mailto:test@example.com",
		"javascript:void(0)",
	}
	for _, url := range mustNotFind {
		if _, ok := found[url]; ok {
			t.Errorf("false positive detected: %q", url)
		}
	}
}

func TestExtractEndpoints_TemplateStrings(t *testing.T) {
	js := "fetch(`/api/users/${userId}/profile`);"
	endpoints := ExtractJsluiceEndpoints(js)

	found := false
	for _, ep := range endpoints {
		if strings.Contains(ep.Endpoint, "/api/users/") && strings.Contains(ep.Endpoint, "/profile") {
			found = true
			if ep.Type != "fetch" {
				t.Errorf("template fetch: got type %q, want 'fetch'", ep.Type)
			}
		}
	}
	if !found {
		t.Error("template string in fetch not extracted")
	}
}

func TestExtractEndpoints_Empty(t *testing.T) {
	endpoints := ExtractJsluiceEndpoints("")
	if len(endpoints) != 0 {
		t.Errorf("empty input: got %d endpoints, want 0", len(endpoints))
	}

	endpoints = ExtractJsluiceEndpoints("var x = 42;")
	if len(endpoints) != 0 {
		t.Errorf("no-url input: got %d endpoints, want 0", len(endpoints))
	}
}

func TestMaybeURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/api/users", true},
		{"https://example.com/path", true},
		{"http://example.com", true},
		{"//cdn.example.com/lib.js", true},
		{"/oauth/callback", true},
		{"api/v1/data", true},
		{"hello", false},
		{".class", false},
		{"#id", false},
		{"application/json", false},
		{"data:image/png", false},
		{"javascript:void(0)", false},
		{"mailto:test@test.com", false},
		{"2024-01-15", false},
		{"ab", false},
	}
	for _, tt := range tests {
		got := maybeURL(tt.input)
		if got != tt.want {
			t.Errorf("maybeURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// largeJS builds a ~10KB JavaScript blob for benchmarking.
func buildLargeJS() string {
	var sb strings.Builder
	// Header
	sb.WriteString(`"use strict";
const API_BASE = "https://api.example.com/v3";
`)
	// Generate many endpoint patterns
	for i := 0; i < 50; i++ {
		sb.WriteString(`fetch("/api/resource/` + string(rune('a'+i%26)) + `");
`)
	}
	for i := 0; i < 20; i++ {
		sb.WriteString(`$.get("/jquery/endpoint/` + string(rune('a'+i%26)) + `");
`)
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(`xhr.open("GET", "/legacy/api/` + string(rune('a'+i%26)) + `");
`)
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(`location.href = "/redirect/` + string(rune('a'+i%26)) + `";
`)
	}
	// Padding with non-URL JS to simulate realistic file
	for i := 0; i < 100; i++ {
		sb.WriteString(`var handler` + string(rune('a'+i%26)) + ` = function(event) {
    console.log("handling event", event.type);
    if (event.target.classList.contains("active")) {
        event.preventDefault();
    }
};
`)
	}
	return sb.String()
}

var benchJS = buildLargeJS()

func BenchmarkExtractJsluiceEndpoints(b *testing.B) {
	// Warmup: preload grammar
	ExtractJsluiceEndpoints("var x = 1;")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ExtractJsluiceEndpoints(benchJS)
	}
}

func BenchmarkExtractJsluiceEndpoints_SmallJS(b *testing.B) {
	small := `fetch("/api/users"); $.get("/api/data"); location.href = "/login";`
	ExtractJsluiceEndpoints("var x = 1;")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ExtractJsluiceEndpoints(small)
	}
}

func BenchmarkExtractJsluiceEndpoints_RealWorld(b *testing.B) {
	ExtractJsluiceEndpoints("var x = 1;")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ExtractJsluiceEndpoints(realWorldJS)
	}
}
