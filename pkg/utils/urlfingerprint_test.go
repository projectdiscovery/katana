package utils

import (
	"fmt"
	"strings"
	"testing"
)

func TestContainsHexLetter(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abcdef01", true},
		{"ABCDEF01", true},
		{"1234abcd", true},
		{"12345678", false},
		{"00000000", false},
		{"0000000a", true},
		{"", false},
		{"g", false}, // not a hex letter
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := containsHexLetter(tt.input); got != tt.want {
				t.Errorf("containsHexLetter(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeSegment(t *testing.T) {
	tests := []struct {
		segment string
		want    string
		matched bool
	}{
		// UUID
		{"550e8400-e29b-41d4-a716-446655440000", "{uuid}", true},
		{"550E8400-E29B-41D4-A716-446655440000", "{uuid}", true},
		// UUID with all-numeric groups still matches (dashes are the anchor)
		{"12345678-1234-1234-1234-123456789012", "{uuid}", true},

		// SHA256
		{"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "{sha256}", true},
		// 64-digit pure numeric should NOT match sha256 — falls to {num}
		{"1234567890123456789012345678901234567890123456789012345678901234", "{num}", true},

		// SHA1
		{"da39a3ee5e6b4b0d3255bfef95601890afd80709", "{sha1}", true},
		// 40-digit pure numeric should NOT match sha1 — falls to {ts} or {num}
		{"1234567890123456789012345678901234567890", "{num}", true},

		// MD5
		{"d41d8cd98f00b204e9800998ecf8427e", "{md5}", true},
		// 32-digit pure numeric should NOT match md5 — falls to {num}
		{"12345678901234567890123456789012", "{num}", true},

		// ObjectId (MongoDB 24 hex chars)
		{"507f1f77bcf86cd799439011", "{oid}", true},
		// 24-digit pure numeric — falls to {num}
		{"123456789012345678901234", "{num}", true},

		// Long hex (>= 8 chars, requires hex letter)
		{"abcdef01", "{hex}", true},
		{"1234abcd5678", "{hex}", true},
		{"DEADBEEF", "{hex}", true},
		// 8-digit pure numeric — NOT hex, falls to {num}
		{"12345678", "{num}", true},
		// 9-digit pure numeric — NOT hex, falls to {num}
		{"123456789", "{num}", true},

		// ISO date
		{"2024-01-15", "{date}", true},
		{"2023-12-31", "{date}", true},
		{"1999-01-01", "{date}", true},
		// Not a date (wrong format) — pure digits, no hex letters, falls to {num}
		{"20240115", "{num}", true},

		// Timestamp (10 digits)
		{"1704067200", "{ts}", true},
		// Timestamp (13 digits)
		{"1704067200000", "{ts}", true},
		// 11 digits — not a timestamp, falls to {num}
		{"17040672001", "{num}", true},
		// 14 digits — not a timestamp, falls to {num}
		{"17040672000001", "{num}", true},

		// Numeric
		{"123", "{num}", true},
		{"0", "{num}", true},
		{"999999", "{num}", true},
		{"1", "{num}", true},

		// Non-matching
		{"users", "users", false},
		{"api", "api", false},
		{"v1", "v1", false},
		{"v2", "v2", false},
		{"my-awesome-post", "my-awesome-post", false},
		{"image123.jpg", "image123.jpg", false},
		{"style.css", "style.css", false},
		{"index.html", "index.html", false},
		{"abcdef", "abcdef", false},  // 6-char hex, below 8-char threshold
		{"feedback", "feedback", false}, // valid hex chars but also contains non-hex
		{"deadbeef-cafe", "deadbeef-cafe", false}, // dash in wrong position, not UUID format
		{"", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.segment, func(t *testing.T) {
			got, matched := normalizeSegment(tt.segment)
			if got != tt.want || matched != tt.matched {
				t.Errorf("normalizeSegment(%q) = (%q, %v), want (%q, %v)",
					tt.segment, got, matched, tt.want, tt.matched)
			}
		})
	}
}

func TestFingerprintURL_RegexOnly(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "numeric path segments",
			url:  "https://example.com/api/v1/users/123/posts/456",
			want: "https://example.com/api/v1/users/{num}/posts/{num}",
		},
		{
			name: "uuid in path",
			url:  "https://example.com/product/550e8400-e29b-41d4-a716-446655440000",
			want: "https://example.com/product/{uuid}",
		},
		{
			name: "date in path",
			url:  "https://example.com/archive/2024-01-15/article",
			want: "https://example.com/archive/{date}/article",
		},
		{
			name: "query params sorted and values dropped",
			url:  "https://example.com/search?z=1&a=2&m=3",
			want: "https://example.com/search?a&m&z",
		},
		{
			name: "no variable segments unchanged",
			url:  "https://example.com/about/team",
			want: "https://example.com/about/team",
		},
		{
			name: "root path",
			url:  "https://example.com/",
			want: "https://example.com/",
		},
		{
			name: "empty path",
			url:  "https://example.com",
			want: "https://example.com",
		},
		{
			name: "mixed pattern types in one path",
			url:  "https://example.com/users/42/posts/da39a3ee5e6b4b0d3255bfef95601890afd80709",
			want: "https://example.com/users/{num}/posts/{sha1}",
		},
		{
			name: "trailing slash preserved",
			url:  "https://example.com/api/v1/users/123/",
			want: "https://example.com/api/v1/users/{num}/",
		},
		{
			name: "timestamp in path",
			url:  "https://example.com/events/1704067200",
			want: "https://example.com/events/{ts}",
		},
		// Scheme and port variations
		{
			name: "http scheme",
			url:  "http://example.com/items/99",
			want: "http://example.com/items/{num}",
		},
		{
			name: "url with port",
			url:  "https://example.com:8443/api/users/42",
			want: "https://example.com:8443/api/users/{num}",
		},
		// Fragment should be stripped by url.Parse (not included in fingerprint)
		{
			name: "fragment stripped",
			url:  "https://example.com/page/123#section",
			want: "https://example.com/page/{num}",
		},
		// Combined path normalization + query normalization
		{
			name: "variable path with query params",
			url:  "https://example.com/users/42/posts?sort=date&page=1",
			want: "https://example.com/users/{num}/posts?page&sort",
		},
		// Multiple different pattern types
		{
			name: "uuid then numeric then date",
			url:  "https://example.com/obj/550e8400-e29b-41d4-a716-446655440000/rev/5/date/2024-01-15",
			want: "https://example.com/obj/{uuid}/rev/{num}/date/{date}",
		},
		{
			name: "md5 hash in path",
			url:  "https://cdn.example.com/assets/d41d8cd98f00b204e9800998ecf8427e/image.png",
			want: "https://cdn.example.com/assets/{md5}/image.png",
		},
		{
			name: "sha256 hash in path",
			url:  "https://example.com/blobs/e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			want: "https://example.com/blobs/{sha256}",
		},
		{
			name: "mongodb objectid in path",
			url:  "https://example.com/docs/507f1f77bcf86cd799439011",
			want: "https://example.com/docs/{oid}",
		},
		{
			name: "long hex token in path",
			url:  "https://example.com/verify/abcdef0123456789",
			want: "https://example.com/verify/{hex}",
		},
		{
			name: "13-digit timestamp",
			url:  "https://example.com/snapshot/1704067200000",
			want: "https://example.com/snapshot/{ts}",
		},
		// Edge cases
		{
			name: "single segment numeric",
			url:  "https://example.com/42",
			want: "https://example.com/{num}",
		},
		{
			name: "query with no path segments",
			url:  "https://example.com/?q=test",
			want: "https://example.com/?q",
		},
		{
			name: "single query param",
			url:  "https://example.com/search?q=hello",
			want: "https://example.com/search?q",
		},
		{
			name: "file extension not affected",
			url:  "https://example.com/assets/image123.jpg",
			want: "https://example.com/assets/image123.jpg",
		},
		{
			name: "deeply nested numeric ids",
			url:  "https://example.com/a/1/b/2/c/3/d/4",
			want: "https://example.com/a/{num}/b/{num}/c/{num}/d/{num}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FingerprintURL(tt.url, nil)
			if got != tt.want {
				t.Errorf("FingerprintURL(%q, nil)\n  got  = %q\n  want = %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestFingerprintURL_Idempotency(t *testing.T) {
	urls := []string{
		"https://example.com/users/123/posts/456",
		"https://example.com/product/550e8400-e29b-41d4-a716-446655440000",
		"https://example.com/search?z=1&a=2",
		"https://example.com/about/team",
	}
	for _, rawURL := range urls {
		first := FingerprintURL(rawURL, nil)
		second := FingerprintURL(first, nil)
		if first != second {
			t.Errorf("not idempotent for %q:\n  first  = %q\n  second = %q", rawURL, first, second)
		}
	}
}

func TestFingerprintURL_SimilarURLsCollapse(t *testing.T) {
	// The core use case: structurally identical URLs should produce the same fingerprint
	groups := [][]string{
		{
			"https://example.com/users/1/profile",
			"https://example.com/users/2/profile",
			"https://example.com/users/999/profile",
		},
		{
			"https://example.com/product/550e8400-e29b-41d4-a716-446655440000",
			"https://example.com/product/6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		},
		{
			"https://example.com/search?q=foo&page=1",
			"https://example.com/search?page=5&q=bar",
		},
	}
	for _, group := range groups {
		fingerprints := make(map[string]bool)
		for _, u := range group {
			fp := FingerprintURL(u, nil)
			fingerprints[fp] = true
		}
		if len(fingerprints) != 1 {
			t.Errorf("URLs in group did not collapse to single fingerprint: %v → %v", group, fingerprints)
		}
	}
}

func TestFingerprintURL_DifferentURLsStayDistinct(t *testing.T) {
	// Structurally different URLs should NOT collapse
	pairs := [][2]string{
		{
			"https://example.com/users/123/profile",
			"https://example.com/users/123/settings",
		},
		{
			"https://example.com/api/v1/users",
			"https://example.com/api/v2/users",
		},
		{
			"https://example.com/search?q=test",
			"https://example.com/search?q=test&page=1",
		},
	}
	for _, pair := range pairs {
		fp1 := FingerprintURL(pair[0], nil)
		fp2 := FingerprintURL(pair[1], nil)
		if fp1 == fp2 {
			t.Errorf("structurally different URLs collapsed:\n  %q → %q\n  %q → %q", pair[0], fp1, pair[1], fp2)
		}
	}
}

func TestFingerprintURL_WithTrie_PromotionLifecycle(t *testing.T) {
	trie := NewPathTrie(0)
	host := "https://example.com"

	// Before promotion: each slug is kept as-is
	for i := range DefaultPromotionThreshold {
		fp := FingerprintURL(fmt.Sprintf("%s/blog/post-%d", host, i), trie)
		expected := fmt.Sprintf("%s/blog/post-%d", host, i)
		if fp != expected {
			t.Fatalf("before promotion: got %q, want %q", fp, expected)
		}
	}

	// Trigger promotion with one more distinct slug
	FingerprintURL(host+"/blog/the-trigger", trie)

	// After promotion: all new slugs should collapse
	got := FingerprintURL(host+"/blog/never-seen-before", trie)
	want := host + "/blog/{param}"
	if got != want {
		t.Errorf("after promotion: got %q, want %q", got, want)
	}

	// Previously seen slugs also collapse after promotion
	got = FingerprintURL(host+"/blog/post-0", trie)
	if got != want {
		t.Errorf("previously seen slug after promotion: got %q, want %q", got, want)
	}
}

func TestFingerprintURL_WithTrie_RegexSegmentsDontInflateTrie(t *testing.T) {
	trie := NewPathTrie(0)

	// Feed many numeric IDs — regex catches them before trie sees them
	for i := range 100 {
		FingerprintURL(fmt.Sprintf("https://example.com/items/%d/details", i), trie)
	}

	// A non-numeric slug should NOT be promoted since all prior values
	// were normalized to "{num}" by regex before reaching the trie
	result := FingerprintURL("https://example.com/items/brand-new-slug/details", trie)
	if strings.Contains(result, "{param}") {
		t.Errorf("regex-detected segments leaked to trie: got %q", result)
	}
}

func TestFingerprintURL_WithTrie_MultipleHosts(t *testing.T) {
	trie := NewPathTrie(0)

	// Promote /users/* on host A
	for i := range DefaultPromotionThreshold + 1 {
		FingerprintURL(fmt.Sprintf("https://a.com/users/user-%d", i), trie)
	}

	// Host B should be unaffected
	got := FingerprintURL("https://b.com/users/alice", trie)
	want := "https://b.com/users/alice"
	if got != want {
		t.Errorf("host isolation: got %q, want %q", got, want)
	}

	// Host A should still collapse
	got = FingerprintURL("https://a.com/users/new-user", trie)
	want = "https://a.com/users/{param}"
	if got != want {
		t.Errorf("host A after promotion: got %q, want %q", got, want)
	}
}

func TestFingerprintURL_WithTrie_DeepPromotion(t *testing.T) {
	trie := NewPathTrie(0)

	// Promote only the username segment, not others
	for i := range DefaultPromotionThreshold + 1 {
		FingerprintURL(fmt.Sprintf("https://example.com/api/users/user-%d/posts", i), trie)
	}

	got := FingerprintURL("https://example.com/api/users/new-user/posts", trie)
	want := "https://example.com/api/users/{param}/posts"
	if got != want {
		t.Errorf("deep promotion: got %q, want %q", got, want)
	}

	// The "api" and "posts" segments should NOT be promoted
	got = FingerprintURL("https://example.com/api/users/another/settings", trie)
	if !strings.HasPrefix(got, "https://example.com/api/users/{param}/") {
		t.Errorf("static segments should stay: got %q", got)
	}
}

func TestFingerprintURL_WithTrie_CombinedRegexAndTrie(t *testing.T) {
	trie := NewPathTrie(0)

	// Promote blog slugs
	for i := range DefaultPromotionThreshold + 1 {
		FingerprintURL(fmt.Sprintf("https://example.com/blog/slug-%d", i), trie)
	}

	// URL with both regex-detectable (numeric) and trie-promoted (slug) segments
	got := FingerprintURL("https://example.com/blog/my-article", trie)
	want := "https://example.com/blog/{param}"
	if got != want {
		t.Errorf("trie promotion: got %q, want %q", got, want)
	}

	// Numeric IDs should still use regex placeholder, not trie
	got = FingerprintURL("https://example.com/items/42", trie)
	want = "https://example.com/items/{num}"
	if got != want {
		t.Errorf("regex on separate path: got %q, want %q", got, want)
	}
}

func TestFingerprintURL_InvalidURL(t *testing.T) {
	// Malformed URL should return the original string
	inputs := []string{
		"://invalid",
		"",
	}
	for _, raw := range inputs {
		got := FingerprintURL(raw, nil)
		if raw == "" {
			// url.Parse("") succeeds with empty result
			continue
		}
		if got != raw {
			t.Errorf("FingerprintURL(%q) = %q, want original returned", raw, got)
		}
	}
}

func TestFingerprintURL_NilTrieSameAsRegexOnly(t *testing.T) {
	urls := []string{
		"https://example.com/users/123",
		"https://example.com/about",
		"https://example.com/search?q=test&page=1",
	}
	for _, u := range urls {
		withNil := FingerprintURL(u, nil)
		withEmpty := FingerprintURL(u, NewPathTrie(0))
		if withNil != withEmpty {
			t.Errorf("nil trie vs empty trie differ for %q:\n  nil   = %q\n  empty = %q", u, withNil, withEmpty)
		}
	}
}
