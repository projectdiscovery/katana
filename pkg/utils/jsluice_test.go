package utils

import (
	"strings"
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
		name         string
		input        string
		wantURLs     []string
		wantCount    int // expected exact count of endpoints; 0 means skip count check
		unwantedURLs []string
	}{
		{
			name:      "fetch call",
			input:     `fetch("/api/users")`,
			wantURLs:  []string{"/api/users"},
			wantCount: 1,
		},
		{
			name:      "XMLHttpRequest open",
			input:     `var xhr = new XMLHttpRequest(); xhr.open("GET", "/api/data");`,
			wantURLs:  []string{"/api/data"},
			wantCount: 1,
		},
		{
			name:      "window.open",
			input:     `window.open("https://example.com/page");`,
			wantURLs:  []string{"https://example.com/page"},
			wantCount: 1,
		},
		{
			name:      "location.href assignment",
			input:     `location.href = "/dashboard/home";`,
			wantURLs:  []string{"/dashboard/home"},
			wantCount: 1,
		},
		{
			name:      "string variable with URL",
			input:     `var apiURL = "https://api.example.com/v1/endpoint";`,
			wantURLs:  []string{"https://api.example.com/v1/endpoint"},
			wantCount: 1,
		},
		{
			name:      "string variable with path",
			input:     `var path = "/api/v2/resources";`,
			wantURLs:  []string{"/api/v2/resources"},
			wantCount: 1,
		},
		{
			name:      "multiple endpoints",
			input:     `fetch("/api/users"); fetch("/api/products");`,
			wantURLs:  []string{"/api/users", "/api/products"},
			wantCount: 2,
		},
		{
			name:      "object literal with URL values",
			input:     `var config = { baseURL: "https://api.example.com", path: "/v1/users" };`,
			wantURLs:  []string{"https://api.example.com", "/v1/users"},
			wantCount: 2,
		},
		{
			name:      "new WebSocket",
			input:     `var ws = new WebSocket("https://ws.example.com/socket");`,
			wantURLs:  []string{"https://ws.example.com/socket"},
			wantCount: 1,
		},
		{
			name:      "new URL",
			input:     `var u = new URL("https://example.com/path/to/resource");`,
			wantURLs:  []string{"https://example.com/path/to/resource"},
			wantCount: 1,
		},
		{
			name:      "jQuery ajax",
			input:     `$.ajax("/api/search");`,
			wantURLs:  []string{"/api/search"},
			wantCount: 1,
		},
		{
			name:      "deduplication",
			input:     `fetch("/api/data"); fetch("/api/data");`,
			wantURLs:  []string{"/api/data"},
			wantCount: 1,
		},
		{
			name:     "malformed JS falls back to regex",
			input:    `{{{ broken js "/api/fallback/endpoint" }}}`,
			wantURLs: []string{"/api/fallback/endpoint"},
		},
		{
			name:      "array of URLs",
			input:     `var urls = ["/api/one", "/api/two", "/api/three"];`,
			wantURLs:  []string{"/api/one", "/api/two", "/api/three"},
			wantCount: 3,
		},
		{
			name:      "conditional expression",
			input:     `var url = flag ? "/api/v1/items" : "/api/v2/items";`,
			wantURLs:  []string{"/api/v1/items", "/api/v2/items"},
			wantCount: 2,
		},
		{
			name:      "function declaration with URLs",
			input:     `function loadData() { return fetch("/api/load"); }`,
			wantURLs:  []string{"/api/load"},
			wantCount: 1,
		},
		{
			name:      "arrow function with URL",
			input:     `var getData = () => fetch("/api/get-data");`,
			wantURLs:  []string{"/api/get-data"},
			wantCount: 1,
		},
		{
			name:      "src assignment",
			input:     `img.src = "/images/logo.png";`,
			wantURLs:  []string{"/images/logo.png"},
			wantCount: 1,
		},
		{
			name:      "no URLs in plain code",
			input:     `var x = 42; console.log(x);`,
			wantCount: 0,
		},
		{
			name:      "absolute URL in variable",
			input:     `var endpoint = "https://api.service.io/v3/query?key=abc";`,
			wantURLs:  []string{"https://api.service.io/v3/query?key=abc"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.input)

			// Create a map of found URLs
			foundURLs := make(map[string]bool)
			for _, ep := range endpoints {
				foundURLs[ep.Endpoint] = true
			}

			// Check that all expected URLs are found
			for _, wantURL := range tt.wantURLs {
				if !foundURLs[wantURL] {
					t.Errorf("ExtractJsluiceEndpoints() missing expected URL %q, got %v", wantURL, endpointStrings(endpoints))
				}
			}

			// Verify exact count to detect false positives
			if tt.wantCount > 0 && len(endpoints) != tt.wantCount {
				t.Errorf("ExtractJsluiceEndpoints() returned %d endpoints, want %d; got %v", len(endpoints), tt.wantCount, endpointStrings(endpoints))
			}

			// Verify no unwanted URLs are extracted
			for _, unwanted := range tt.unwantedURLs {
				if foundURLs[unwanted] {
					t.Errorf("ExtractJsluiceEndpoints() extracted unwanted URL %q", unwanted)
				}
			}
		})
	}
}

func TestIsURLLike(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/path", true},
		{"/api/v1/users", true},
		{"./relative/path", true},
		{"../parent/path", true},
		{"//cdn.example.com/lib.js", true},
		{"just a string", false},
		{"x", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isURLLike(tt.input); got != tt.want {
				t.Errorf("isURLLike(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPreprocessES6(t *testing.T) {
	input := `import foo from 'bar';
export default function hello() { return "/api/test"; }`
	result := preprocessES6(input)
	if result == input {
		t.Error("preprocessES6 should have modified the input")
	}
	// Verify URL strings are preserved after preprocessing
	if !strings.Contains(result, `"/api/test"`) {
		t.Error("preprocessES6 should preserve URL strings")
	}
	// Verify import statement is removed
	if strings.Contains(result, "import foo") {
		t.Error("preprocessES6 should remove import statements")
	}

	// Test named exports are stripped
	namedExportInput := `export { foo, bar };
var x = "/api/named";`
	namedResult := preprocessES6(namedExportInput)
	if strings.Contains(namedResult, "export {") {
		t.Error("preprocessES6 should remove named export statements")
	}
	if !strings.Contains(namedResult, `"/api/named"`) {
		t.Error("preprocessES6 should preserve URL strings around named exports")
	}

	// Test re-exports are stripped
	reExportInput := `export * from './module';
export { default as foo } from './other';
var y = "/api/reexport";`
	reExportResult := preprocessES6(reExportInput)
	if strings.Contains(reExportResult, "export *") {
		t.Error("preprocessES6 should remove star re-export statements")
	}
	if strings.Contains(reExportResult, "export {") {
		t.Error("preprocessES6 should remove named re-export statements")
	}
	if !strings.Contains(reExportResult, `"/api/reexport"`) {
		t.Error("preprocessES6 should preserve URL strings around re-exports")
	}
}

// endpointStrings is a test helper that extracts endpoint strings for error messages.
func endpointStrings(eps []JSLuiceEndpoint) []string {
	var result []string
	for _, ep := range eps {
		result = append(result, ep.Endpoint)
	}
	return result
}
