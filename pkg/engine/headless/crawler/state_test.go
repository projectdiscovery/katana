package crawler

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer/simhash"
	"github.com/stretchr/testify/assert"
)

func TestPageFingerprint_Stability(t *testing.T) {

}

func TestPageFingerprint(t *testing.T) {
	tests := []struct {
		name        string
		html1       string
		html2       string
		shouldMatch bool
	}{
		{
			name: "same page different dynamic content",
			html1: `
                <html>
                    <head><title>Home</title></head>
                    <body>
                        <h2>Welcome John!</h2>
                        <nav>
                            <a href="/home">Home</a>
                            <a href="/profile">Profile</a>
                        </nav>
                    </body>
                </html>`,
			html2: `
                <html>
                    <head><title>Home</title></head>
                    <body>
                        <h2>Welcome Jane!</h2>
                        <nav>
                            <a href="/home">Home</a>
                            <a href="/profile">Profile</a>
                        </nav>
                    </body>
                </html>`,
			shouldMatch: true,
		},
		{
			name: "same form different values",
			html1: `
                <form action="/login" method="post">
                    <input type="text" name="username" value="user1"/>
                    <input type="password" name="password" value="pass1"/>
                </form>`,
			html2: `
                <form action="/login" method="post">
                    <input type="text" name="username" value="user2"/>
                    <input type="password" name="password" value="pass2"/>
                </form>`,
			shouldMatch: true,
		},
		{
			name: "different error messages",
			html1: `
                <div class="alert">Invalid password</div>`,
			html2: `
                <div class="alert">Account locked</div>`,
			shouldMatch: true,
		},
		{
			name: "different page structure",
			html1: `
                <div><h1>Page 1</h1><p>Content</p></div>`,
			html2: `
                <div><h2>Page 1</h2><div>Content</div></div>`,
			shouldMatch: false,
		},
	}

	getHash := func(html string) (string, error) {
		strippedDOM, err := getStrippedDOM(html)
		if err != nil {
			return "", errors.Wrap(err, "could not get stripped dom")
		}
		// Get sha256 hash of the stripped dom
		return sha256Hash(strippedDOM), nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1, err := getHash(tt.html1)
			assert.NoError(t, err)

			hash2, err := getHash(tt.html2)
			assert.NoError(t, err)

			if tt.shouldMatch {
				assert.Equal(t, hash1, hash2)
			} else {
				assert.NotEqual(t, hash1, hash2)
			}
		})
	}
}

func TestSimHashSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		html1     string
		html2     string
		threshold uint8
		similar   bool
	}{
		{
			name:      "identical pages",
			html1:     `<html><body><h1>Hello World</h1><p>Content here</p></body></html>`,
			html2:     `<html><body><h1>Hello World</h1><p>Content here</p></body></html>`,
			threshold: 4,
			similar:   true,
		},
		{
			name:      "pages with minor changes",
			html1:     `<html><body><h1>Hello World</h1><p>Content here</p><span>Time: 12:00</span></body></html>`,
			html2:     `<html><body><h1>Hello World</h1><p>Content here</p><span>Time: 12:01</span></body></html>`,
			threshold: 4,
			similar:   true,
		},
		{
			name:      "pages with different content",
			html1:     `<html><body><h1>Hello World</h1><p>Content here</p></body></html>`,
			html2:     `<html><body><h1>Goodbye World</h1><p>Different content</p><div>Extra stuff</div></body></html>`,
			threshold: 4,
			similar:   false,
		},
		{
			name:      "pages with dynamic IDs",
			html1:     `<html><body><div id="content-12345"><h1>Hello</h1></div></body></html>`,
			html2:     `<html><body><div id="content-67890"><h1>Hello</h1></div></body></html>`,
			threshold: 4,
			similar:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Normalize and compute SimHash
			norm1, err := domNormalizer.Apply(tt.html1)
			if err != nil {
				t.Fatalf("Failed to normalize html1: %v", err)
			}

			norm2, err := domNormalizer.Apply(tt.html2)
			if err != nil {
				t.Fatalf("Failed to normalize html2: %v", err)
			}

			hash1 := simhash.Fingerprint(strings.NewReader(norm1), 3)
			hash2 := simhash.Fingerprint(strings.NewReader(norm2), 3)

			distance := simhash.Distance(hash1, hash2)
			isSimilar := distance <= tt.threshold

			if isSimilar != tt.similar {
				t.Errorf("Expected similar=%v, got similar=%v (distance=%d)", tt.similar, isSimilar, distance)
				t.Logf("Hash1: %064b", hash1)
				t.Logf("Hash2: %064b", hash2)
				t.Logf("Normalized HTML1:\n%s", norm1)
				t.Logf("Normalized HTML2:\n%s", norm2)
			}
		})
	}
}
