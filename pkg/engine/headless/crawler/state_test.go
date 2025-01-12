package crawler

import (
	"testing"

	"github.com/pkg/errors"
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
