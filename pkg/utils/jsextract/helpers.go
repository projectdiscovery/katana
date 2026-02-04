package jsextract

import (
	"regexp"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/unistring"
)

var (
	// URL pattern matchers
	urlSchemeRegex   = regexp.MustCompile(`^https?://`)
	pathRegex        = regexp.MustCompile(`^[./]`)
	apiPathRegex     = regexp.MustCompile(`^/?(api|v\d+|graphql|rest|ws|ajax|rpc)/`)
	fileExtRegex     = regexp.MustCompile(`\.(html?|php|aspx?|jsp|json|xml|do|action|cgi|pl|py|rb|go|js|css|wasm)(\?|#|$)`)
	queryStringRegex = regexp.MustCompile(`\?[a-zA-Z0-9_]+=`)

	// Invalid URL patterns to filter out
	invalidPrefixes = []string{
		"javascript:", "data:", "mailto:", "tel:", "vbscript:", "blob:", "about:",
	}

	// Template syntax patterns that indicate non-URLs
	templatePatterns = []string{
		"{{", "}}", "${", "<%", "%>", "{%", "%}", "#{",
	}

	// URL-related attribute names
	urlAttributes = map[string]bool{
		"href": true, "src": true, "action": true, "data": true,
		"poster": true, "background": true, "cite": true, "formaction": true,
		"srcset": true, "content": true, "url": true, "path": true,
	}

	// Location-related object names
	locationObjects = map[string]bool{
		"location": true, "window": true, "document": true, "self": true, "top": true,
	}

	// Regex patterns for fallback extraction
	fallbackPatterns = []*regexp.Regexp{
		regexp.MustCompile(`["'` + "`]" + `(https?://[^\s"'` + "`" + `<>]{5,})["'` + "`]"),
		regexp.MustCompile(`["'` + "`]" + `(/[a-zA-Z0-9_\-./]+(?:\?[^\s"'` + "`" + `<>]*)?)["'` + "`]"),
		regexp.MustCompile(`["'` + "`]" + `(\./[a-zA-Z0-9_\-./]+)["'` + "`]"),
		regexp.MustCompile(`["'` + "`]" + `(\.\./[a-zA-Z0-9_\-./]+)["'` + "`]"),
	}
)

// extractStringValue extracts string value from an expression.
func extractStringValue(expr ast.Expression) string {
	if expr == nil {
		return ""
	}

	switch e := expr.(type) {
	case *ast.StringLiteral:
		return e.Value.String()
	case *ast.TemplateLiteral:
		return buildTemplateString(e, true)
	case *ast.BinaryExpression:
		return handleBinaryExpression(e)
	case *ast.Identifier:
		// Return empty for variable references
		return ""
	}
	return ""
}

// buildTemplateString builds a string from template literal parts.
func buildTemplateString(tmpl *ast.TemplateLiteral, withPlaceholders bool) string {
	if tmpl == nil || len(tmpl.Elements) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, elem := range tmpl.Elements {
		sb.WriteString(elem.Literal)
		if withPlaceholders && i < len(tmpl.Expressions) {
			sb.WriteString("EXPR")
		}
	}
	return sb.String()
}

// handleBinaryExpression handles string concatenation.
func handleBinaryExpression(e *ast.BinaryExpression) string {
	if e.Operator.String() != "+" {
		return ""
	}

	left := extractStringValue(e.Left)
	right := extractStringValue(e.Right)

	if left == "" && right == "" {
		return ""
	}
	if left == "" {
		left = "EXPR"
	}
	if right == "" {
		right = "EXPR"
	}
	return left + right
}

// getExpressionName gets the name from an expression (identifier or member).
func getExpressionName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Name.String()
	case *ast.DotExpression:
		return e.Identifier.Name.String()
	case *ast.BracketExpression:
		return extractStringValue(e.Member)
	}
	return ""
}

// getPropertyKeyName gets the key name from an object property key.
func getPropertyKeyName(key ast.Expression) string {
	switch k := key.(type) {
	case *ast.Identifier:
		return k.Name.String()
	case *ast.StringLiteral:
		return k.Value.String()
	}
	return ""
}

// uniToString converts unistring.String to string.
func uniToString(s unistring.String) string {
	return s.String()
}

// isURLAttribute checks if an attribute name is URL-related.
func isURLAttribute(attr string) bool {
	return urlAttributes[strings.ToLower(attr)]
}

// isLocationObject checks if an object name is location-related.
func isLocationObject(name string) bool {
	return locationObjects[strings.ToLower(name)]
}

// isValidURL performs basic URL validation.
func isValidURL(url string) bool {
	if url == "" || len(url) < 2 {
		return false
	}

	// Check for invalid prefixes
	lowerURL := strings.ToLower(url)
	for _, prefix := range invalidPrefixes {
		if strings.HasPrefix(lowerURL, prefix) {
			return false
		}
	}

	// Check for template syntax patterns
	for _, pattern := range templatePatterns {
		if strings.Contains(url, pattern) {
			return false
		}
	}

	// Reject hash-only references
	if strings.HasPrefix(url, "#") {
		return false
	}

	// Reject URLs that are too short to be meaningful
	if len(strings.Trim(url, "/")) < 1 {
		return false
	}

	return true
}

// isLikelyURL checks if a string looks like a URL or path.
func isLikelyURL(s string) bool {
	if len(s) < 2 || len(s) > 2048 {
		return false
	}

	// Must pass basic validation
	if !isValidURL(s) {
		return false
	}

	// Check for common URL patterns
	if urlSchemeRegex.MatchString(s) {
		return true
	}
	if pathRegex.MatchString(s) {
		return true
	}
	if apiPathRegex.MatchString(s) {
		return true
	}
	if fileExtRegex.MatchString(s) {
		return true
	}
	if queryStringRegex.MatchString(s) {
		return true
	}

	// Check if it contains path separators with meaningful content
	if strings.Contains(s, "/") && len(s) > 3 {
		parts := strings.Split(s, "/")
		validParts := 0
		for _, part := range parts {
			if len(part) > 0 && !strings.HasPrefix(part, "EXPR") {
				validParts++
			}
		}
		if validParts >= 2 {
			return true
		}
	}

	return false
}

// extractURLsRegex is a fallback regex-based URL extractor for invalid JS.
func extractURLsRegex(jsCode string) []Endpoint {
	var endpoints []Endpoint
	seen := make(map[string]bool)

	for _, re := range fallbackPatterns {
		matches := re.FindAllStringSubmatch(jsCode, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				url := strings.TrimSpace(match[1])
				if url != "" && !seen[url] && isValidURL(url) {
					seen[url] = true
					endpoints = append(endpoints, Endpoint{URL: url, Type: "regex"})
				}
			}
		}
	}

	return endpoints
}
