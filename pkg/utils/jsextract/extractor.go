// Package jsextract provides JavaScript URL extraction using AST parsing.
// It extracts URLs and endpoints from JavaScript code by analyzing the AST
// for common patterns like fetch(), XMLHttpRequest, location assignments, etc.
package jsextract

import (
	"github.com/dop251/goja/parser"
)

// Endpoint represents an extracted URL endpoint from JavaScript code.
type Endpoint struct {
	URL  string
	Type string
}

// ExtractURLs parses JavaScript code and extracts URLs/endpoints.
// It analyzes the AST for common URL patterns including:
//   - fetch() calls
//   - XMLHttpRequest.open() calls
//   - window.location assignments
//   - window.open() calls
//   - String literals containing URL patterns
//   - Template literals with URL patterns
func ExtractURLs(jsCode string) []Endpoint {
	if jsCode == "" {
		return nil
	}

	program, err := parser.ParseFile(nil, "", jsCode, 0)
	if err != nil {
		// Fallback to regex-based extraction on parse error
		return extractURLsRegex(jsCode)
	}

	visitor := &urlVisitor{
		endpoints: make([]Endpoint, 0),
		seen:      make(map[string]bool),
	}
	visitor.walkProgram(program)

	return visitor.endpoints
}
