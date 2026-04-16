package utils

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// segmentPattern defines a regex pattern and its replacement placeholder
// for identifying variable path segments.
type segmentPattern struct {
	regex       *regexp.Regexp
	placeholder string
	// validate provides an additional check beyond the regex match.
	// When nil, the regex match alone is sufficient.
	validate func(string) bool
}

// containsHexLetter returns true if the string has at least one a-f/A-F character.
// Pure-numeric strings that happen to be ≥8 digits should fall through to
// timestamp/numeric patterns instead of matching as hex.
func containsHexLetter(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			return true
		}
	}
	return false
}

// segmentPatterns are tested in order from most specific to most general.
// Each pattern is anchored (^...$) to match complete path segments only.
var segmentPatterns = []segmentPattern{
	// Exact-length hashes and identifiers
	{regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`), "{uuid}", nil},
	{regexp.MustCompile(`^[0-9a-fA-F]{64}$`), "{sha256}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{40}$`), "{sha1}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{32}$`), "{md5}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{24}$`), "{oid}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{8,}$`), "{hex}", containsHexLetter},
	// Base64-encoded tokens (JWTs, verification tokens, etc.)
	{regexp.MustCompile(`^[A-Za-z0-9_-]{20,}={0,2}$`), "{base64}", containsLetterAndDigit},
	// Dates and timestamps
	{regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), "{date}", nil},
	{regexp.MustCompile(`^\d{10}(\d{3})?$`), "{ts}", nil},
	// Slugs with numeric suffix: my-article-12345, report-67890
	{regexp.MustCompile(`^.+-\d{3,}$`), "{slug}", nil},
	// Pure numeric (catchall)
	{regexp.MustCompile(`^\d+$`), "{num}", nil},
}

// containsLetterAndDigit returns true if the string contains at least one letter
// and at least one digit. This avoids false-matching pure-alpha path segments
// (like "settings") as base64 tokens.
func containsLetterAndDigit(s string) bool {
	var hasLetter, hasDigit bool
	for _, c := range s {
		if unicode.IsLetter(c) {
			hasLetter = true
		}
		if unicode.IsDigit(c) {
			hasDigit = true
		}
		if hasLetter && hasDigit {
			return true
		}
	}
	return false
}

// doubleSlashRe matches two or more consecutive slashes.
var doubleSlashRe = regexp.MustCompile(`/{2,}`)

// normalizeSegment checks a single path segment against heuristic patterns.
// Returns the placeholder if matched, or the original segment if no pattern matches.
func normalizeSegment(segment string) (string, bool) {
	for _, p := range segmentPatterns {
		if p.regex.MatchString(segment) {
			if p.validate != nil && !p.validate(segment) {
				continue
			}
			return p.placeholder, true
		}
	}
	return segment, false
}

// FingerprintURL produces a structural fingerprint of the given URL by:
// 1. Replacing variable path segments (IDs, UUIDs, hashes, dates) with placeholders
// 2. Using the adaptive trie (if provided) to detect learned parameter positions
// 3. Dropping query parameter values, keeping only sorted keys
//
// When trie is nil, only Layer 1 regex-based normalization is applied.
func FingerprintURL(rawURL string, trie *PathTrie) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	path := u.Path

	// Normalize: collapse double slashes and lowercase the path for consistent trie lookups
	path = doubleSlashRe.ReplaceAllString(path, "/")
	path = strings.ToLower(path)

	if path == "" || path == "/" {
		return buildFingerprint(u, path, "")
	}

	// Split path into segments, preserving leading slash
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return buildFingerprint(u, "/", "")
	}
	segments := strings.Split(trimmed, "/")

	// Layer 1: heuristic regex normalization
	for i, seg := range segments {
		if placeholder, matched := normalizeSegment(seg); matched {
			segments[i] = placeholder
		}
	}

	// Layer 2: adaptive trie normalization
	if trie != nil {
		segments = trie.Fingerprint(u.Hostname(), segments)
	}

	fingerprintedPath := "/" + strings.Join(segments, "/")
	if strings.HasSuffix(u.Path, "/") {
		fingerprintedPath += "/"
	}

	// Handle fragment-based routing (SPA hash routes like #/users/123)
	var fragmentFingerprint string
	if u.Fragment != "" && strings.HasPrefix(u.Fragment, "/") {
		fragTrimmed := strings.Trim(u.Fragment, "/")
		if fragTrimmed != "" {
			fragSegments := strings.Split(strings.ToLower(fragTrimmed), "/")
			for i, seg := range fragSegments {
				if placeholder, matched := normalizeSegment(seg); matched {
					fragSegments[i] = placeholder
				}
			}
			if trie != nil {
				fragSegments = trie.Fingerprint(u.Hostname()+"#", fragSegments)
			}
			fragmentFingerprint = "#/" + strings.Join(fragSegments, "/")
		}
	}

	return buildFingerprint(u, fingerprintedPath, fragmentFingerprint)
}

// buildFingerprint reconstructs the URL with the fingerprinted path,
// sorted query keys (values dropped), and optional fragment fingerprint.
func buildFingerprint(u *url.URL, path, fragment string) string {
	var b strings.Builder
	if u.Scheme != "" {
		b.WriteString(u.Scheme)
		b.WriteString("://")
	}
	b.WriteString(u.Host)
	b.WriteString(path)

	if u.RawQuery != "" {
		keys := sortedQueryKeys(u.Query())
		if len(keys) > 0 {
			b.WriteByte('?')
			b.WriteString(strings.Join(keys, "&"))
		}
	}

	if fragment != "" {
		b.WriteString(fragment)
	}

	return b.String()
}

// sortedQueryKeys extracts and sorts query parameter keys.
func sortedQueryKeys(params url.Values) []string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
