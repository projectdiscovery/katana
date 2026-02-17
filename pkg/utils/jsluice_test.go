package utils

import "testing"

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
		name       string
		jsCode     string
		wantCount  int
		wantFirst  string
		wantType   string
	}{
		{
			name:      "fetch call",
			jsCode:    `fetch("/api/users")`,
			wantCount: 1,
			wantFirst: "/api/users",
			wantType:  "fetch",
		},
		{
			name:      "string literal URL",
			jsCode:    `var url = "https://example.com/api/data"`,
			wantCount: 1,
			wantFirst: "https://example.com/api/data",
			wantType:  "stringLiteral",
		},
		{
			name:      "location assignment",
			jsCode:    `location.href = "/login"`,
			wantCount: 1,
			wantFirst: "/login",
			wantType:  "locationAssignment",
		},
		{
			name:      "window.open",
			jsCode:    `window.open("/dashboard")`,
			wantCount: 1,
			wantFirst: "/dashboard",
			wantType:  "window.open",
		},
		{
			name:      "jQuery get",
			jsCode:    `$.get("/api/items")`,
			wantCount: 1,
			wantFirst: "/api/items",
			wantType:  "$.get",
		},
		{
			name:      "XMLHttpRequest open",
			jsCode:    `xhr.open("GET", "/api/data")`,
			wantCount: 1,
			wantFirst: "/api/data",
			wantType:  "xhr.open",
		},
		{
			name:      "no endpoints in plain code",
			jsCode:    `var x = 1 + 2; console.log(x);`,
			wantCount: 0,
		},
		{
			name:      "filter data: URLs",
			jsCode:    `var img = "data:image/png;base64,abc123"`,
			wantCount: 0,
		},
		{
			name:      "string concatenation",
			jsCode:    `location.href = "/api/" + userId + "/profile"`,
			wantCount: 3, // concatenated URL + individual string literals
			wantFirst: "/api/EXPR/profile",
			wantType:  "locationAssignment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := ExtractJsluiceEndpoints(tt.jsCode)
			if len(endpoints) != tt.wantCount {
				t.Errorf("ExtractJsluiceEndpoints() returned %d endpoints, want %d. Got: %v", len(endpoints), tt.wantCount, endpoints)
				return
			}
			if tt.wantCount > 0 {
				if endpoints[0].Endpoint != tt.wantFirst {
					t.Errorf("ExtractJsluiceEndpoints() first endpoint = %q, want %q", endpoints[0].Endpoint, tt.wantFirst)
				}
				if endpoints[0].Type != tt.wantType {
					t.Errorf("ExtractJsluiceEndpoints() first type = %q, want %q", endpoints[0].Type, tt.wantType)
				}
			}
		})
	}
}
