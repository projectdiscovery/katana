package normalizer

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	htmlpkg "golang.org/x/net/html"
)

var whiteSpacesRegex = regexp.MustCompile(`[\r\n]+|\s+`)

type Normalizer struct {
	dom  *DOMNormalizer
	text *TextNormalizer
}

// New returns a new Normalizer
func New() (*Normalizer, error) {
	textNormalizer, err := NewTextNormalizer()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create text normalizer")
	}
	domNormalizer := NewDOMNormalizer()
	return &Normalizer{
		dom:  domNormalizer,
		text: textNormalizer,
	}, nil
}

// Apply applies the normalizers to the given content
//
// It normalizes the given content by:
// - Applying the DOM normalizer
// - Applying the text normalizer
// - Denormalizing it
func (n *Normalizer) Apply(text string) (string, error) {
	first := normalizeDocument(text)

	firstpass, err := n.dom.Apply(first)
	if err != nil {
		return "", errors.Wrap(err, "failed to apply DOM normalizer")
	}

	secondpass, err := stripTextContent(firstpass)
	if err != nil {
		return "", errors.Wrap(err, "failed to strip text content")
	}

	thirdpass := n.text.Apply(secondpass)

	fourthpass := normalizeDocument(thirdpass)
	return fourthpass, nil
}

// normalizeDocument normalizes the given document by:
// - Lowercasing it
// - URL decoding it
// - HTML entity decoding it
// - Replacing all whitespace variations with a space
// - Trimming the document whitespaces
func normalizeDocument(text string) string {
	// Lowercase the document
	lowercased := strings.ToLower(text)

	// Convert hexadecimal escape sequences to HTML entities
	converted := convertHexEscapeSequencesToEntities(lowercased)
	unescaped := html.UnescapeString(converted)

	// URL Decode and HTML entity decode the document to standardize it.
	urlDecoded, err := url.QueryUnescape(unescaped)
	if err != nil {
		urlDecoded = unescaped
	}

	// Replace all whitespaces with a space
	normalized := whiteSpacesRegex.ReplaceAllString(urlDecoded, " ")

	// Trim the document to remove leading and trailing whitespaces
	return strings.Trim(normalized, " \r\n\t")
}

func replaceHexEscapeSequence(match string) string {
	// Remove the '\x' prefix
	code := strings.TrimPrefix(match, "\\x")
	// Parse the hexadecimal code to an integer
	value, err := strconv.ParseInt(code, 16, 32)
	if err != nil {
		// If there's an error, return the original match
		return match
	}
	// Return the corresponding HTML entity
	return fmt.Sprintf("&#x%x;", value)
}

// Define the regex pattern to match hexadecimal escape sequences
var pattern = regexp.MustCompile(`\\x[0-9a-fA-F]{2}`)

func convertHexEscapeSequencesToEntities(input string) string {
	return pattern.ReplaceAllStringFunc(input, func(match string) string {
		return replaceHexEscapeSequence(match)
	})
}

func stripTextContent(content string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}

	doc.Find("h1, h2, h3, h4, h5, h6, p, span, div, td, th, li, a").Each(func(_ int, s *goquery.Selection) {
		removeTextNodesFromSelection(s)
	})

	result, err := doc.Html()
	if err != nil {
		return "", err
	}
	return result, nil
}

func removeTextNodesFromSelection(s *goquery.Selection) {
	node := s.Get(0)
	if node == nil {
		return
	}
	for c := node.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == htmlpkg.TextNode {
			node.RemoveChild(c)
		}
		c = next
	}
}
