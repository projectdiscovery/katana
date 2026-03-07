package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
)

// Parser interface for extracting links from HTML/JS content
type Parser interface {
	Parse(resp *navigation.Response) (*navigation.Response, error)
}

// DefaultParser is the pure-Go parser that doesn't require CGO
type DefaultParser struct {
	linkRegex *regexp.Regexp
}

// NewDefaultParser creates a new default parser
func NewDefaultParser() *DefaultParser {
	return &DefaultParser{
		linkRegex: regexp.MustCompile(`(?i)href=["']?([^"'>\s]+)`),
	}
}

// Parse extracts links from HTML content using goquery (pure Go, no CGO)
func (p *DefaultParser) Parse(resp *navigation.Response) (*navigation.Response, error) {
	if resp.Body == "" {
		return resp, nil
	}

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resp.Body))
	if err != nil {
		return resp, err
	}

	// Extract links from anchor tags
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			resp.Links = append(resp.Links, navigation.Link{
				URL:  href,
				Type: "anchor",
			})
		}
	})

	// Extract links from script tags
	doc.Find("script[src]").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			resp.Links = append(resp.Links, navigation.Link{
				URL:  src,
				Type: "script",
			})
		}
	})

	// Extract links from link tags (CSS, etc.)
	doc.Find("link[href]").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			resp.Links = append(resp.Links, navigation.Link{
				URL:  href,
				Type: "resource",
			})
		}
	})

	// Extract URLs from inline JavaScript using regex
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		matches := p.linkRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				resp.Links = append(resp.Links, navigation.Link{
					URL:  match[1],
					Type: "javascript",
				})
			}
		}
	})

	return resp, nil
}

// GetParser returns the appropriate parser based on build tags and availability
func GetParser() Parser {
	// Use tree-sitter parser if available and enabled, otherwise fall back to default
	return NewDefaultParser()
}
