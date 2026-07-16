package similarity

import (
	"bytes"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
)

const (
	minTokensForSimilarity = 5
	shingleSize            = 3
)

// Document is normalized page content ready for similarity scoring.
type Document struct {
	Tokens   []string
	Shingles []string
}

// Normalize extracts comparable text features from a response body.
// HTML is stripped of scripts/styles/chrome; non-HTML is tokenized as plain text.
// ok is false when there is not enough signal for similarity scoring.
func Normalize(body []byte) (Document, bool) {
	text := extractText(body)
	tokens := tokenize(text)
	if len(tokens) < minTokensForSimilarity {
		return Document{}, false
	}
	return Document{
		Tokens:   tokens,
		Shingles: shingles(tokens, shingleSize),
	}, true
}

func extractText(body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}

	looksHTML := bytes.Contains(bytes.ToLower(trimmed[:min(len(trimmed), 512)]), []byte("<html")) ||
		bytes.Contains(bytes.ToLower(trimmed[:min(len(trimmed), 512)]), []byte("<!doctype")) ||
		bytes.Contains(trimmed, []byte("<body")) ||
		bytes.Contains(trimmed, []byte("<div")) ||
		bytes.Contains(trimmed, []byte("<p"))

	if !looksHTML {
		return string(trimmed)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(trimmed))
	if err != nil {
		return string(trimmed)
	}

	doc.Find("script, style, noscript, svg, template").Remove()

	root := doc.Find("main, [role=main]").First()
	if root.Length() == 0 {
		// Fall back to all articles when no main landmark exists so multi-article
		// index/pagination pages are compared on their full content.
		root = doc.Find("article")
	}
	if root.Length() == 0 {
		root = doc.Find("body")
	}
	if root.Length() == 0 {
		root = doc.Selection
	}
	root.Find("nav, header, footer, aside").Remove()
	return root.Text()
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 2 {
			continue
		}
		// Lowercase and clone so corpus retention does not pin the full page buffer.
		out = append(out, strings.Clone(strings.ToLower(f)))
	}
	return out
}

func shingles(tokens []string, n int) []string {
	if n <= 0 || len(tokens) < n {
		// Fall back to tokens so short docs still get a fingerprint.
		cp := make([]string, len(tokens))
		copy(cp, tokens)
		return cp
	}
	out := make([]string, 0, len(tokens)-n+1)
	for i := 0; i <= len(tokens)-n; i++ {
		out = append(out, strings.Join(tokens[i:i+n], " "))
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
