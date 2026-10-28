package normalizer

import (
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// DefaultDOMTransformations is default list of CSS selectors to remove from the DOM.
var DefaultDOMTransformations = []string{
	"style, script, path",           // remove script and style tags
	"input[type='hidden']",          // remove hidden inputs
	"meta[content]",                 // remove meta tags with content
	"link[rel='stylesheet']",        // remove stylesheet links
	"svg",                           // remove svg
	"grammarly-desktop-integration", // remove grammarly
	"div[class*='banner'], div[id*='banner'], div[class*='pixel'], div[id*='pixel']", // remove banner and pixel divs
	"input[name*='csrf'], input[name*='token']",                                      // remove csrf and token inputs
}

// NoChildrenDomTransformations removes all elements with no children
var NoChildrenDomTransformations = []string{
	"div",    // remove divs with no children
	"span",   // remove spans with no children
	"form",   // remove forms with no children
	"iframe", // remove iframes
}

// DOMNormalizer is a normalizer for DOM content
type DOMNormalizer struct {
	customTransformations []domTransformationFunc
}

// NewDOMNormalizer returns a new DOMNormalizer
//
// transformations is a list of CSS selectors to remove from the DOM.
func NewDOMNormalizer() *DOMNormalizer {
	var customTransformations []domTransformationFunc
	for _, t := range DefaultDOMTransformations {
		t := t
		customTransformations = append(customTransformations, func(doc *goquery.Document) {
			doc.Find(t).Each(func(_ int, s *goquery.Selection) {
				s.Remove()
			})
		})
	}

	customTransformations = append(customTransformations, removeAdContainers)

	for _, t := range NoChildrenDomTransformations {
		t := t
		customTransformations = append(customTransformations, func(doc *goquery.Document) {
			doc.Find(t).Each(func(_ int, s *goquery.Selection) {
				if s.Children().Length() == 0 && strings.TrimSpace(s.Text()) == "" {
					if node := s.Get(0); node != nil && len(node.Attr) == 0 {
						s.Remove()
					}
				}
			})
		})
	}

	return &DOMNormalizer{customTransformations: customTransformations}
}

// Apply applies the normalizers to the given content
func (d *DOMNormalizer) Apply(content string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return "", err
	}
	// Apply custom transformations first (selector-based removals, etc.)
	for _, f := range d.customTransformations {
		f(doc)
	}
	// Apply selection based transformations once at the root (recursive helpers will traverse)
	for _, f := range selectionBasedTransformationFuncs {
		f(doc.Selection)
	}
	result, err := doc.Html()
	if err != nil {
		return "", err
	}
	return result, nil
}

// adWords are matched as whole class and id words, plus a camelCase
// boundary (adContainer) and a trailing number (ad1). A substring match
// also hits header, padded or download and strips the real page content,
// which makes different pages of one site share a state.
var adWords = map[string]struct{}{"ad": {}, "ads": {}, "advert": {}, "advertisement": {}, "adsbygoogle": {}}

func removeAdContainers(doc *goquery.Document) {
	doc.Find("div").Each(func(_ int, s *goquery.Selection) {
		if hasAdWord(s.AttrOr("class", "")) || hasAdWord(s.AttrOr("id", "")) {
			s.Remove()
		}
	})
}

func hasAdWord(value string) bool {
	for _, word := range adTokens(value) {
		if _, ok := adWords[word]; ok {
			return true
		}
	}
	return false
}

// adTokens splits class and id text on separators and camelCase boundaries.
// A token that ends in digits also contributes its letter stem, so ad1
// matches ad without treating admin or pad1 as ads.
func adTokens(value string) []string {
	var b strings.Builder
	var prev rune
	started := false
	for _, r := range value {
		if started && unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
			b.WriteByte(' ')
		}
		b.WriteRune(unicode.ToLower(r))
		prev = r
		started = true
	}
	parts := strings.FieldsFunc(b.String(), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		words = append(words, part)
		stem := strings.TrimRightFunc(part, unicode.IsDigit)
		if stem != "" && stem != part {
			words = append(words, stem)
		}
	}
	return words
}

// domTransformationFunc does required transformation on document.
type domTransformationFunc func(doc *goquery.Document)

type selectionTransformationFunc func(s *goquery.Selection)

var selectionBasedTransformationFuncs = []selectionTransformationFunc{
	removeCommentsDomTransformationFunc,              // remove comments
	removeClassIDDataAttributesDomTransformationFunc, // remove class, id and data attributes
}

func removeComments(n *html.Node) {
	if n.Type == html.CommentNode {
		n.Parent.RemoveChild(n)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		removeComments(c)
	}
}

func removeCommentsDomTransformationFunc(s *goquery.Selection) {
	if n := s.Get(0); n != nil {
		removeComments(n)
	}
}

var attributes = []string{
	"class",
	"id",
	"name",
	"href",
	"style",
	"width",
	"height",
	"src",
	"nowrap",
	"target",
	"valign",
	"cellpadding",
	"cellspacing",
	"value",
	"placeholder",
	"title",
	"alt",
}

func removeClassIDDataAttributesDomTransformationFunc(s *goquery.Selection) {
	removeAttributes(s)
	// Handle children
	s.Children().Each(func(_ int, child *goquery.Selection) {
		removeClassIDDataAttributesDomTransformationFunc(child)
	})
}

func removeAttributes(s *goquery.Selection) {
	for _, attr := range attributes {
		s.RemoveAttr(attr)
	}

	for _, node := range s.Nodes {
		for _, attr := range node.Attr {
			attr := attr
			if strings.HasPrefix(attr.Key, "data-") || strings.HasPrefix(attr.Key, "aria-") || strings.HasPrefix(attr.Key, "js") {
				s.RemoveAttr(attr.Key)
			}
		}
	}
}
