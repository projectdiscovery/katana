package utils

import (
	"strings"
	"sync"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// endpoint represents an extracted URL/API endpoint.
type endpoint struct {
	URL    string
	Type   string
	Source string
}

var (
	jsLang     *gotreesitter.Language
	jsLangOnce sync.Once
)

func getJSLang() *gotreesitter.Language {
	jsLangOnce.Do(func() {
		jsLang = grammars.JavascriptLanguage()
	})
	return jsLang
}

// extractEndpoints parses JavaScript source and returns all detected endpoints.
func extractEndpoints(source []byte) []endpoint {
	lang := getJSLang()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(source)
	if err != nil {
		return nil
	}
	defer tree.Release()

	seen := make(map[string]struct{})
	var results []endpoint

	structuredURLs := make(map[string]struct{})
	walkStructured(tree.RootNode(), lang, source, &results, seen, structuredURLs)
	walkStrings(tree.RootNode(), lang, source, &results, seen, structuredURLs)

	return results
}

// walkStructured does a DFS looking for call_expression and assignment_expression nodes.
func walkStructured(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte, results *[]endpoint, seen, structuredURLs map[string]struct{}) {
	nodeType := node.Type(lang)

	switch nodeType {
	case "call_expression":
		if eps := matchCallExpression(node, lang, src); len(eps) > 0 {
			for _, ep := range eps {
				if _, dup := seen[ep.URL]; !dup {
					seen[ep.URL] = struct{}{}
					structuredURLs[ep.URL] = struct{}{}
					*results = append(*results, ep)
				}
			}
		}
	case "assignment_expression":
		if eps := matchAssignmentExpression(node, lang, src); len(eps) > 0 {
			for _, ep := range eps {
				if _, dup := seen[ep.URL]; !dup {
					seen[ep.URL] = struct{}{}
					structuredURLs[ep.URL] = struct{}{}
					*results = append(*results, ep)
				}
			}
		}
	}

	for i := 0; i < node.NamedChildCount(); i++ {
		walkStructured(node.NamedChild(i), lang, src, results, seen, structuredURLs)
	}
}

// walkStrings does a DFS looking for string-like nodes with URL-like content.
func walkStrings(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte, results *[]endpoint, seen, structuredURLs map[string]struct{}) {
	nodeType := node.Type(lang)
	switch nodeType {
	case "string_fragment":
		tryAddStringURL(node.Text(src), results, seen, structuredURLs)
	case "template_string":
		if node.NamedChildCount() > 0 {
			resolved := resolveExpr(node, lang, src)
			tryAddStringURL(resolved, results, seen, structuredURLs)
			return
		}
	case "binary_expression":
		resolved := resolveExpr(node, lang, src)
		tryAddStringURL(resolved, results, seen, structuredURLs)
		return
	}

	for i := 0; i < node.NamedChildCount(); i++ {
		walkStrings(node.NamedChild(i), lang, src, results, seen, structuredURLs)
	}
}

func tryAddStringURL(text string, results *[]endpoint, seen, structuredURLs map[string]struct{}) bool {
	if _, already := structuredURLs[text]; already {
		return false
	}
	if _, dup := seen[text]; dup || !maybeURL(text) {
		return false
	}
	seen[text] = struct{}{}
	*results = append(*results, endpoint{
		URL:  text,
		Type: "string",
	})
	return true
}

// --- Pattern matchers ---

func matchCallExpression(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []endpoint {
	if node.NamedChildCount() < 2 {
		return nil
	}
	fn := node.NamedChild(0)
	args := node.NamedChild(1)

	fnType := fn.Type(lang)
	if args.Type(lang) != "arguments" {
		return nil
	}

	switch fnType {
	case "identifier":
		return matchSimpleCall(fn, args, lang, src)
	case "import":
		url := firstStringArg(args, lang, src)
		if url == "" {
			return nil
		}
		return []endpoint{{URL: url, Type: "import", Source: node.Text(src)}}
	case "member_expression":
		return matchMemberCall(fn, args, lang, src)
	}
	return nil
}

func matchSimpleCall(fn, args *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []endpoint {
	name := fn.Text(src)
	switch name {
	case "fetch", "import":
		url := firstStringArg(args, lang, src)
		if url == "" {
			return nil
		}
		return []endpoint{{URL: url, Type: name, Source: fn.Parent().Text(src)}}
	}
	return nil
}

func matchMemberCall(fn, args *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []endpoint {
	obj, method := memberParts(fn, lang, src)

	switch {
	case obj == "window" && method == "open":
		url := firstStringArg(args, lang, src)
		if url == "" {
			return nil
		}
		return []endpoint{{URL: url, Type: "window.open", Source: fn.Parent().Text(src)}}
	case method == "open":
		url := nthStringArg(args, 1, lang, src)
		if url == "" {
			return nil
		}
		return []endpoint{{URL: url, Type: "xhr", Source: fn.Parent().Text(src)}}
	case (obj == "$" || obj == "jQuery") && isJQueryMethod(method):
		url := firstStringArg(args, lang, src)
		if url == "" {
			return nil
		}
		return []endpoint{{URL: url, Type: "jquery", Source: fn.Parent().Text(src)}}
	case (obj == "location" || obj == "window") && (method == "replace" || method == "assign"):
		url := firstStringArg(args, lang, src)
		if url == "" {
			return nil
		}
		return []endpoint{{URL: url, Type: "location", Source: fn.Parent().Text(src)}}
	}
	return nil
}

func matchAssignmentExpression(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) []endpoint {
	if node.NamedChildCount() < 2 {
		return nil
	}
	left := node.NamedChild(0)
	right := node.NamedChild(1)

	if left.Type(lang) != "member_expression" {
		return nil
	}
	obj, prop := memberParts(left, lang, src)
	if !isLocationAssignment(obj, prop) {
		return nil
	}
	url := extractStringValue(right, lang, src)
	if url == "" {
		return nil
	}
	return []endpoint{{URL: url, Type: "location", Source: node.Text(src)}}
}

func isLocationAssignment(obj, prop string) bool {
	return (obj == "location" || obj == "window") &&
		(prop == "href" || prop == "pathname" || prop == "search")
}

func isJQueryMethod(method string) bool {
	switch method {
	case "get", "post", "ajax", "getJSON":
		return true
	}
	return false
}

func memberParts(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) (obj, prop string) {
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		switch child.Type(lang) {
		case "identifier":
			obj = child.Text(src)
		case "property_identifier":
			prop = child.Text(src)
		}
	}
	return
}

func firstStringArg(args *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	return nthStringArg(args, 0, lang, src)
}

func nthStringArg(args *gotreesitter.Node, n int, lang *gotreesitter.Language, src []byte) string {
	if args.NamedChildCount() <= n {
		return ""
	}
	return extractStringValue(args.NamedChild(n), lang, src)
}

func extractStringValue(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	switch node.Type(lang) {
	case "string_fragment":
		return node.Text(src)
	case "string":
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child.Type(lang) == "string_fragment" {
				return child.Text(src)
			}
		}
	case "binary_expression", "template_string":
		return resolveExpr(node, lang, src)
	}
	return ""
}

// resolveExpr walks a binary_expression or template_string, building a resolved
// URL string. Dynamic parts become "EXPR".
func resolveExpr(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	switch node.Type(lang) {
	case "string_fragment":
		return node.Text(src)
	case "string":
		for i := 0; i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child.Type(lang) == "string_fragment" {
				return child.Text(src)
			}
		}
		return ""
	case "binary_expression":
		if node.NamedChildCount() < 2 {
			return ""
		}
		return resolveExpr(node.NamedChild(0), lang, src) + resolveExpr(node.NamedChild(1), lang, src)
	case "template_string":
		return resolveTemplateString(node, lang, src)
	case "template_substitution":
		return "EXPR"
	default:
		return "EXPR"
	}
}

func resolveTemplateString(node *gotreesitter.Node, lang *gotreesitter.Language, src []byte) string {
	start := node.StartByte() + 1
	end := node.EndByte() - 1

	if node.NamedChildCount() == 0 {
		if start < end && int(end) <= len(src) {
			return string(src[start:end])
		}
		return ""
	}

	var result string
	cursor := start
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child.Type(lang) != "template_substitution" {
			continue
		}
		if child.StartByte() > cursor {
			result += string(src[cursor:child.StartByte()])
		}
		result += "EXPR"
		cursor = child.EndByte()
	}
	if cursor < end && int(end) <= len(src) {
		result += string(src[cursor:end])
	}
	return result
}

// --- URL heuristic ---

func maybeURL(s string) bool {
	if len(s) < 5 {
		return false
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return true
	}
	if strings.HasPrefix(s, "//") && len(s) > 2 && s[2] != '/' {
		return true
	}
	if s[0] == '/' && len(s) > 1 && s[1] != '/' && s[1] != '*' {
		return true
	}
	if isFalsePositive(s) {
		return false
	}
	if strings.Contains(s, "/") && !strings.HasPrefix(s, "/*") {
		parts := strings.SplitN(s, "/", 3)
		if len(parts) >= 2 && len(parts[0]) > 0 && len(parts[1]) > 0 {
			return true
		}
	}
	return false
}

func isFalsePositive(s string) bool {
	if s[0] == '.' || s[0] == '#' || s[0] == ':' {
		return true
	}
	if strings.Contains(s, "${") {
		return true
	}
	if strings.HasPrefix(s, "text/") || strings.HasPrefix(s, "application/") ||
		strings.HasPrefix(s, "image/") || strings.HasPrefix(s, "multipart/") {
		return true
	}
	if strings.HasPrefix(s, "data:") || strings.HasPrefix(s, "javascript:") ||
		strings.HasPrefix(s, "mailto:") {
		return true
	}
	if len(s) >= 4 && isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2]) && isDigit(s[3]) {
		return true
	}
	return false
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
