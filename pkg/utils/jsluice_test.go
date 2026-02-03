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
		{
			name: "ES module with import",
			jsCode: `import React from 'react';
const API_URL = "https://api.example.com/data";
export default function App() {
  fetch(API_URL);
}`,
			wantEndpoints: 1,
			checkContains: []string{"https://api.example.com/data"},
		},
		{
			name: "ES module with multiple imports and exports",
			jsCode: `import { useState } from 'react';
import axios from 'axios';

const endpoint = "/api/users";
const baseUrl = "https://example.com";

export function getData() {
  return axios.get(endpoint);
}

export default baseUrl;`,
			wantEndpoints: 2,
			checkContains: []string{"/api/users", "https://example.com"},
		},
		{
			name: "ES module with export default",
			jsCode: `export default {
  apiUrl: "https://api.example.com/v1",
  endpoint: "/api/config"
};`,
			wantEndpoints: 2,
			checkContains: []string{"https://api.example.com/v1", "/api/config"},
		},
		{
			name:          "regex-only matches in invalid JS",
			jsCode:        `some random text with https://example.com/path and "/api/endpoint" that isn't valid JS`,
			wantEndpoints: 2,
			checkContains: []string{"https://example.com/path", "/api/endpoint"},
		},
		{
			name: "mixed valid JS and regex patterns",
			jsCode: `var x = "https://example.com/api";
// Comment with https://comment.example.com/path
var y = /regex/;`,
			wantEndpoints: 2,
			checkContains: []string{"https://example.com/api", "https://comment.example.com/path"},
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

func TestPreprocessModuleCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "import with from",
			input:    `import React from 'react';`,
			expected: ``,
		},
		{
			name:     "import with destructuring",
			input:    `import { useState, useEffect } from 'react';`,
			expected: ``,
		},
		{
			name:     "import side effect",
			input:    `import './styles.css';`,
			expected: ``,
		},
		{
			name:     "export default",
			input:    `export default function App() {}`,
			expected: `function App() {}`,
		},
		{
			name:     "export const",
			input:    `export const API_URL = "test";`,
			expected: `const API_URL = "test";`,
		},
		{
			name: "mixed import and export",
			input: `import React from 'react';
const x = 1;
export default x;`,
			expected: `
const x = 1;
x;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessModuleCode(tt.input)
			if result != tt.expected {
				t.Errorf("preprocessModuleCode() = %q, want %q", result, tt.expected)
			}
		})
	}
}
