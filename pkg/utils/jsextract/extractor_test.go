package jsextract

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractURLs_FetchCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "simple fetch",
			input: `fetch("/api/users")`,
			expected: []Endpoint{
				{URL: "/api/users", Type: "fetch"},
			},
		},
		{
			name:  "fetch with full URL",
			input: `fetch("https://api.example.com/data")`,
			expected: []Endpoint{
				{URL: "https://api.example.com/data", Type: "fetch"},
			},
		},
		{
			name:  "fetch with options",
			input: `fetch("/api/users", { method: "POST" })`,
			expected: []Endpoint{
				{URL: "/api/users", Type: "fetch"},
			},
		},
		{
			name:  "window.fetch",
			input: `window.fetch("/api/data")`,
			expected: []Endpoint{
				{URL: "/api/data", Type: "fetch"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_XHR(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "xhr open GET",
			input: `xhr.open("GET", "/api/users")`,
			expected: []Endpoint{
				{URL: "/api/users", Type: "xhr"},
			},
		},
		{
			name:  "xhr open POST",
			input: `xhr.open("POST", "/api/submit")`,
			expected: []Endpoint{
				{URL: "/api/submit", Type: "xhr"},
			},
		},
		{
			name: "XMLHttpRequest full example",
			input: `
				var xhr = new XMLHttpRequest();
				xhr.open("GET", "/api/data");
				xhr.send();
			`,
			expected: []Endpoint{
				{URL: "/api/data", Type: "xhr"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_LocationAssignment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "window.location",
			input: `window.location = "/dashboard"`,
			expected: []Endpoint{
				{URL: "/dashboard", Type: "location"},
			},
		},
		{
			name:  "window.location.href",
			input: `window.location.href = "/profile"`,
			expected: []Endpoint{
				{URL: "/profile", Type: "location"},
			},
		},
		{
			name:  "document.location",
			input: `document.location = "/page"`,
			expected: []Endpoint{
				{URL: "/page", Type: "location"},
			},
		},
		{
			name:  "location.href",
			input: `location.href = "https://example.com/redirect"`,
			expected: []Endpoint{
				{URL: "https://example.com/redirect", Type: "location"},
			},
		},
		{
			name:  "location.assign",
			input: `location.assign("/new-page")`,
			expected: []Endpoint{
				{URL: "/new-page", Type: "location"},
			},
		},
		{
			name:  "location.replace",
			input: `location.replace("/replaced")`,
			expected: []Endpoint{
				{URL: "/replaced", Type: "location"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_WindowOpen(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "window.open simple",
			input: `window.open("/popup")`,
			expected: []Endpoint{
				{URL: "/popup", Type: "window.open"},
			},
		},
		{
			name:  "window.open with target",
			input: `window.open("/external", "_blank")`,
			expected: []Endpoint{
				{URL: "/external", Type: "window.open"},
			},
		},
		{
			name:  "open function",
			input: `open("/new-window")`,
			expected: []Endpoint{
				{URL: "/new-window", Type: "window.open"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_StringLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "variable with URL path",
			input: `var apiUrl = "/api/v1/users"`,
			expected: []Endpoint{
				{URL: "/api/v1/users", Type: "string"},
			},
		},
		{
			name:  "const with full URL",
			input: `const endpoint = "https://api.example.com/data"`,
			expected: []Endpoint{
				{URL: "https://api.example.com/data", Type: "string"},
			},
		},
		{
			name:  "let declaration",
			input: `let url = "/api/posts"`,
			expected: []Endpoint{
				{URL: "/api/posts", Type: "string"},
			},
		},
		{
			name:  "multiple URLs",
			input: `var url1 = "/api/users"; var url2 = "/api/posts";`,
			expected: []Endpoint{
				{URL: "/api/users", Type: "string"},
				{URL: "/api/posts", Type: "string"},
			},
		},
		{
			name:  "URL with query string",
			input: `var url = "/search?q=test&page=1"`,
			expected: []Endpoint{
				{URL: "/search?q=test&page=1", Type: "string"},
			},
		},
		{
			name:  "relative paths",
			input: `var path1 = "./scripts/main.js"; var path2 = "../images/logo.png";`,
			expected: []Endpoint{
				{URL: "./scripts/main.js", Type: "string"},
				{URL: "../images/logo.png", Type: "string"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_TemplateLiterals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "simple template literal",
			input: "var url = `/api/users`",
			expected: []Endpoint{
				{URL: "/api/users", Type: "string"},
			},
		},
		{
			name:  "template with expression",
			input: "var url = `/api/users/${userId}`",
			expected: []Endpoint{
				{URL: "/api/users/EXPR", Type: "template"},
			},
		},
		{
			name:  "template with multiple expressions",
			input: "var url = `/api/${version}/users/${id}`",
			expected: []Endpoint{
				{URL: "/api/EXPR/users/EXPR", Type: "template"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_StringConcatenation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "simple concatenation",
			input: `var url = "/api" + "/users"`,
			expected: []Endpoint{
				{URL: "/api/users", Type: "string"},
			},
		},
		{
			name:  "concatenation with variable",
			input: `var url = "/api/users/" + userId`,
			expected: []Endpoint{
				{URL: "/api/users/EXPR", Type: "string"},
			},
		},
		{
			name:  "base URL concatenation",
			input: `var url = baseUrl + "/endpoint"`,
			expected: []Endpoint{
				{URL: "/endpoint", Type: "string"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_JQueryAjax(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "$.ajax with url property",
			input: `$.ajax({ url: "/api/data", method: "GET" })`,
			expected: []Endpoint{
				{URL: "/api/data", Type: "ajax"},
			},
		},
		{
			name:  "$.get",
			input: `$.get("/api/users")`,
			expected: []Endpoint{
				{URL: "/api/users", Type: "get"},
			},
		},
		{
			name:  "$.post",
			input: `$.post("/api/submit", data)`,
			expected: []Endpoint{
				{URL: "/api/submit", Type: "post"},
			},
		},
		{
			name:  "$.getJSON",
			input: `$.getJSON("/api/config.json")`,
			expected: []Endpoint{
				{URL: "/api/config.json", Type: "getJSON"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_SetAttribute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "setAttribute href",
			input: `element.setAttribute("href", "/link")`,
			expected: []Endpoint{
				{URL: "/link", Type: "setAttribute"},
			},
		},
		{
			name:  "setAttribute src",
			input: `img.setAttribute("src", "/images/photo.jpg")`,
			expected: []Endpoint{
				{URL: "/images/photo.jpg", Type: "setAttribute"},
			},
		},
		{
			name:     "setAttribute non-url attribute",
			input:    `element.setAttribute("class", "container")`,
			expected: []Endpoint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_FunctionBody(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain []string
	}{
		{
			name: "function declaration",
			input: `
				function loadData() {
					fetch("/api/data");
				}
			`,
			shouldContain: []string{"/api/data"},
		},
		{
			name: "arrow function",
			input: `
				const loadData = () => {
					return fetch("/api/users");
				}
			`,
			shouldContain: []string{"/api/users"},
		},
		{
			name:          "arrow function expression body",
			input:         `const getData = () => fetch("/api/quick")`,
			shouldContain: []string{"/api/quick"},
		},
		{
			name: "nested functions",
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
			name: "async function",
			input: `
				async function fetchData() {
					const response = await fetch("/api/async");
					return response.json();
				}
			`,
			shouldContain: []string{"/api/async"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.URL
			}
			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected)
			}
		})
	}
}

func TestExtractURLs_ObjectLiterals(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain []string
	}{
		{
			name: "config object",
			input: `
				var config = {
					apiUrl: "/api/v1",
					imageUrl: "/images/default.png"
				};
			`,
			shouldContain: []string{"/api/v1", "/images/default.png"},
		},
		{
			name: "routes object",
			input: `
				const routes = {
					home: "/",
					users: "/users",
					profile: "/profile"
				};
			`,
			shouldContain: []string{"/users", "/profile"},
		},
		{
			name: "nested object",
			input: `
				const api = {
					endpoints: {
						users: "/api/users",
						posts: "/api/posts"
					}
				};
			`,
			shouldContain: []string{"/api/users", "/api/posts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.URL
			}
			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected)
			}
		})
	}
}

func TestExtractURLs_ArrayLiterals(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain []string
	}{
		{
			name: "array of URLs",
			input: `
				const urls = ["/api/users", "/api/posts", "/api/comments"];
			`,
			shouldContain: []string{"/api/users", "/api/posts", "/api/comments"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.URL
			}
			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected)
			}
		})
	}
}

func TestExtractURLs_ControlFlow(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		shouldContain []string
	}{
		{
			name: "if statement",
			input: `
				if (condition) {
					fetch("/api/true");
				} else {
					fetch("/api/false");
				}
			`,
			shouldContain: []string{"/api/true", "/api/false"},
		},
		{
			name: "for loop",
			input: `
				for (var i = 0; i < 10; i++) {
					fetch("/api/item/" + i);
				}
			`,
			shouldContain: []string{"/api/item/EXPR"},
		},
		{
			name: "switch statement",
			input: `
				switch (type) {
					case "a":
						fetch("/api/a");
						break;
					case "b":
						fetch("/api/b");
						break;
				}
			`,
			shouldContain: []string{"/api/a", "/api/b"},
		},
		{
			name: "try catch",
			input: `
				try {
					fetch("/api/try");
				} catch (e) {
					fetch("/api/catch");
				} finally {
					fetch("/api/finally");
				}
			`,
			shouldContain: []string{"/api/try", "/api/catch", "/api/finally"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.URL
			}
			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected)
			}
		})
	}
}

func TestExtractURLs_InvalidURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:     "javascript protocol",
			input:    `var url = "javascript:alert(1)"`,
			expected: []Endpoint{},
		},
		{
			name:     "data URI",
			input:    `var url = "data:text/html,<h1>Hello</h1>"`,
			expected: []Endpoint{},
		},
		{
			name:     "mailto",
			input:    `var url = "mailto:test@example.com"`,
			expected: []Endpoint{},
		},
		{
			name:     "hash only",
			input:    `var url = "#section"`,
			expected: []Endpoint{},
		},
		{
			name:     "template syntax mustache",
			input:    `var url = "{{baseUrl}}/api"`,
			expected: []Endpoint{},
		},
		{
			name:     "template syntax ejs",
			input:    `var url = "<%=baseUrl%>/api"`,
			expected: []Endpoint{},
		},
		{
			name:     "blob URL",
			input:    `var url = "blob:https://example.com/uuid"`,
			expected: []Endpoint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestExtractURLs_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		minExpected   int
		shouldContain []string
	}{
		{
			name: "React component",
			input: `
				import React, { useEffect, useState } from 'react';

				function UserList() {
					const [users, setUsers] = useState([]);

					useEffect(() => {
						fetch('/api/users')
							.then(res => res.json())
							.then(data => setUsers(data));
					}, []);

					const deleteUser = (id) => {
						fetch('/api/users/' + id, { method: 'DELETE' });
					};

					return null;
				}
			`,
			minExpected:   1,
			shouldContain: []string{"/api/users"},
		},
		{
			name: "API client",
			input: `
				const API_BASE = '/api/v2';

				async function getUsers() {
					const response = await fetch(API_BASE + '/users');
					return response.json();
				}

				async function createUser(data) {
					return fetch('/api/v2/users', {
						method: 'POST',
						body: JSON.stringify(data)
					});
				}
			`,
			minExpected:   1,
			shouldContain: []string{"/api/v2/users"},
		},
		{
			name: "Event handlers",
			input: `
				document.getElementById('btn').onclick = function() {
					window.location.href = '/dashboard';
				};

				document.querySelector('form').onsubmit = function() {
					fetch('/api/submit', { method: 'POST' });
				};
			`,
			minExpected:   2,
			shouldContain: []string{"/dashboard", "/api/submit"},
		},
		{
			name: "Class component",
			input: `
				class ApiService {
					constructor() {
						this.baseUrl = '/api/v1';
					}

					getUsers() {
						return fetch(this.baseUrl + '/users');
					}

					getPost(id) {
						return fetch('/api/v1/posts/' + id);
					}
				}
			`,
			minExpected:   1,
			shouldContain: []string{"/api/v1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			require.GreaterOrEqual(t, len(endpoints), tt.minExpected,
				"Expected at least %d endpoints, got %d", tt.minExpected, len(endpoints))

			extractedURLs := make([]string, len(endpoints))
			for i, ep := range endpoints {
				extractedURLs[i] = ep.URL
			}

			for _, expected := range tt.shouldContain {
				assert.Contains(t, extractedURLs, expected,
					"Expected to find URL %s in extracted URLs", expected)
			}
		})
	}
}

func TestExtractURLs_ParseError(t *testing.T) {
	// Test that invalid JS falls back to regex extraction
	input := `
		this is not valid { javascript
		but it contains "/api/fallback" as a string
	`
	endpoints := ExtractURLs(input)
	// Should still extract via regex fallback
	found := false
	for _, ep := range endpoints {
		if ep.URL == "/api/fallback" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should extract URL via regex fallback on parse error")
}

func TestExtractURLs_EmptyInput(t *testing.T) {
	endpoints := ExtractURLs("")
	assert.Empty(t, endpoints)
}

func TestExtractURLs_Deduplication(t *testing.T) {
	input := `
		fetch("/api/users");
		fetch("/api/users");
		var url = "/api/users";
	`
	endpoints := ExtractURLs(input)

	// Count occurrences of /api/users
	count := 0
	for _, ep := range endpoints {
		if ep.URL == "/api/users" {
			count++
		}
	}
	assert.Equal(t, 1, count, "Should deduplicate URLs")
}

func TestExtractURLs_PropertyAssignment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Endpoint
	}{
		{
			name:  "element.src assignment",
			input: `img.src = "/images/photo.jpg"`,
			expected: []Endpoint{
				{URL: "/images/photo.jpg", Type: "assignment"},
			},
		},
		{
			name:  "element.href assignment",
			input: `link.href = "/styles/main.css"`,
			expected: []Endpoint{
				{URL: "/styles/main.css", Type: "assignment"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs(tt.input)
			assert.ElementsMatch(t, tt.expected, endpoints)
		})
	}
}

func TestIsValidURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid absolute path", "/api/users", true},
		{"valid relative path", "./script.js", true},
		{"valid full URL", "https://example.com/api", true},
		{"javascript protocol", "javascript:void(0)", false},
		{"data URI", "data:text/html,test", false},
		{"mailto", "mailto:test@test.com", false},
		{"empty string", "", false},
		{"single char", "/", false},
		{"hash only", "#section", false},
		{"template syntax", "{{url}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLikelyURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"absolute path", "/api/users", true},
		{"relative dot path", "./scripts/main.js", true},
		{"relative dotdot path", "../images/logo.png", true},
		{"full https URL", "https://api.example.com/data", true},
		{"full http URL", "http://localhost:8080/api", true},
		{"API path pattern", "api/v1/users", true},
		{"file with extension", "config.json", true},
		{"path with query", "/search?q=test", true},
		{"plain text", "hello world", false},
		{"single word", "test", false},
		{"too long", string(make([]byte, 3000)), false},
		{"graphql path", "/graphql/query", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLikelyURL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractStringValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string literal",
			input:    `"/api/users"`,
			expected: "/api/users",
		},
		{
			name:     "concatenation",
			input:    `"/api" + "/users"`,
			expected: "/api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractURLs("var x = " + tt.input)
			if tt.expected != "" {
				require.NotEmpty(t, endpoints)
				assert.Equal(t, tt.expected, endpoints[0].URL)
			}
		})
	}
}

func BenchmarkExtractURLs(b *testing.B) {
	jsCode := `
		const API_URL = '/api/v1';
		fetch(API_URL + '/users');
		fetch('/api/posts');
		window.location.href = '/dashboard';
		$.ajax({ url: '/api/data' });
		xhr.open('GET', '/api/config');

		function loadData() {
			return fetch('/api/load');
		}

		const routes = {
			home: '/',
			users: '/users',
			settings: '/settings'
		};
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractURLs(jsCode)
	}
}

func BenchmarkExtractURLs_LargeFile(b *testing.B) {
	// Simulate a larger JS file
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString(`fetch("/api/endpoint` + string(rune('0'+i%10)) + `");`)
		sb.WriteString(`var url` + string(rune('0'+i%10)) + ` = "/path/to/resource";`)
	}
	jsCode := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractURLs(jsCode)
	}
}
