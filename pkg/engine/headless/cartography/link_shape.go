package cartography

import (
	"regexp"
	"strings"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// numberedName matches names that end in a counter, such as ember123 or
// el-4821: what frameworks give elements they create.
var numberedName = regexp.MustCompile(`^[A-Za-z]*\d{3,}$`)

// PathShape returns path with each variable segment, such as a number, a
// UUID, a hex ID or a long token, replaced by "{v}". Segments that name a
// route stay, so /product/123 and /product/456 share a shape while
// /product/123 and /user/123 do not.
func PathShape(path string) string {
	if path == "" {
		return ""
	}
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if VariableSegment(seg) {
			segments[i] = "{v}"
		}
	}
	return strings.Join(segments, "/")
}

// VariableSegment reports whether a path segment is a value rather than a
// route name: all digits, a UUID, a hex ID of 8 or more characters, an
// alphanumeric token of 16 or more characters with letters and digits, or a
// URL-safe token (see urlSafeToken).
func VariableSegment(seg string) bool {
	if seg == "" {
		return false
	}
	if allOf(seg, isDigit) || uuidPattern.MatchString(seg) || hexID(seg) || alnumToken(seg) {
		return true
	}
	return urlSafeToken(seg)
}

func hexID(seg string) bool {
	return len(seg) >= 8 && strings.IndexFunc(seg, isDigit) >= 0 && allOf(seg, isHex)
}

func alnumToken(seg string) bool {
	return len(seg) >= 16 && mixed(seg) && allOf(seg, func(r rune) bool { return isDigit(r) || isLetter(r) })
}

// urlSafeToken reports whether seg is a token of URL-safe characters (letters,
// digits, "-" and "_"), such as abC123_def456_ghi789. Split on "-" and "_",
// it has two or more parts that mix letters and digits, or one part that is a
// hex ID or an alphanumeric token. Named routes such as release-notes-2024 or
// v2-api-docs have at most one mixed part, so they stay.
func urlSafeToken(seg string) bool {
	if !allOf(seg, func(r rune) bool { return isDigit(r) || isLetter(r) || r == '-' || r == '_' }) {
		return false
	}
	parts := strings.FieldsFunc(seg, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) < 2 {
		return false
	}
	mixedParts := 0
	for _, part := range parts {
		if hexID(part) || alnumToken(part) {
			return true
		}
		if mixed(part) {
			mixedParts++
		}
	}
	return mixedParts >= 2
}

// mixed reports whether s has both letters and digits.
func mixed(s string) bool {
	return strings.IndexFunc(s, isDigit) >= 0 && strings.IndexFunc(s, isLetter) >= 0
}

// GeneratedID reports whether an element ID looks made by a framework or a
// counter, so it can change on the next visit: a React useId (":r1:"), or an
// ID with a variable part or a counter, such as item-123, ember4821 or
// btn-9f8e7d6c.
func GeneratedID(id string) bool {
	if id == "" {
		return false
	}
	if len(id) > 2 && strings.HasPrefix(id, ":") && strings.HasSuffix(id, ":") {
		return true
	}
	parts := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '_' || r == ':' || r == '.' })
	for _, part := range parts {
		// A short number, as in col-md-6 or step-2, is a name.
		if allOf(part, isDigit) && len(part) < 3 {
			continue
		}
		if VariableSegment(part) || numberedName.MatchString(part) {
			return true
		}
	}
	return false
}

// cssHasGeneratedID reports whether a CSS path holds an ID that looks
// generated, on the element or on one of its ancestors. An escaped ID, such
// as one that starts with a digit, counts as generated.
func cssHasGeneratedID(css string) bool {
	for _, step := range strings.Split(css, " > ") {
		_, id, ok := strings.Cut(step, "#")
		if !ok {
			continue
		}
		if strings.Contains(id, `\`) || GeneratedID(id) {
			return true
		}
	}
	return false
}

func allOf(s string, fn func(rune) bool) bool {
	for _, r := range s {
		if !fn(r) {
			return false
		}
	}
	return true
}

func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isLetter(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }
func isHex(r rune) bool    { return isDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') }
