//go:build !(386 || windows)

package parser

import (
	"net/http"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	urlutil "github.com/projectdiscovery/utils/url"
	"github.com/stretchr/testify/require"
)

func TestScriptJSFileJsluiceParser(t *testing.T) {
	newResponse := func(t *testing.T, rawURL, contentType, body string) *navigation.Response {
		t.Helper()
		parsed, err := urlutil.Parse(rawURL)
		require.NoError(t, err)
		header := http.Header{}
		if contentType != "" {
			header.Set("Content-Type", contentType)
		}
		return &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}, Header: header}, Body: body}
	}
	const jsBody = `fetch("/api/v1/users");`
	const cssBody = `.b{content:"/api/v1/not-an-endpoint"}`

	tests := []struct {
		name, url, contentType, body string
		wantParsed                   bool
	}{
		{"js extension", "https://example.com/app.js", "", jsBody, true},
		{"javascript content type", "https://example.com/bundle", "application/javascript", jsBody, true},
		{"css extension with css content type", "https://example.com/styles.css", "text/css", cssBody, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scriptJSFileJsluiceParser(newResponse(t, tt.url, tt.contentType, tt.body))
			require.Equal(t, tt.wantParsed, len(got) > 0, "unexpected jsluice result: %v", got)
		})
	}
}
