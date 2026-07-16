package utils

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
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
	{regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`), "{uuid}", nil},
	{regexp.MustCompile(`^[0-9a-fA-F]{64}$`), "{sha256}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{40}$`), "{sha1}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{32}$`), "{md5}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{24}$`), "{oid}", containsHexLetter},
	{regexp.MustCompile(`^[0-9a-fA-F]{8,}$`), "{hex}", containsHexLetter},
	{regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), "{date}", nil},
	{regexp.MustCompile(`^\d{10}(\d{3})?$`), "{ts}", nil},
	{regexp.MustCompile(`^\d+$`), "{num}", nil},
}

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
	if path == "" || path == "/" {
		return buildFingerprint(u, path)
	}

	// Split path into segments, preserving leading slash
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return buildFingerprint(u, "/")
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
	if strings.HasSuffix(path, "/") {
		fingerprintedPath += "/"
	}

	return buildFingerprint(u, fingerprintedPath)
}

// buildFingerprint reconstructs the URL with the fingerprinted path
// and sorted query keys (values dropped).
func buildFingerprint(u *url.URL, path string) string {
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
