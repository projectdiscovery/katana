package extractor

import (
	"regexp"
	"strings"
)

// RegexUrlExtractor is a concrete implementation of the UrlExtractor interface, using regex for extraction.
type RegexUrlExtractor struct {
	extractor *regexp.Regexp
}

// NewRegexUrlExtractor creates a new regular expression to extract urls
func NewRegexUrlExtractor() (*RegexUrlExtractor, error) {
	extractor, err := regexp.Compile(`(?:http|https)?://(?:www\.)?[a-zA-Z0-9./?=_%:-]*`)
	if err != nil {
		return nil, err
	}
	return &RegexUrlExtractor{extractor: extractor}, nil
}

// Extract implements the UrlExtractor interface, using the regex to find urls in the given text.
func (re *RegexUrlExtractor) Extract(text string) []string {
	matches := re.extractor.FindAllString(text, -1)
	for i, match := range matches {
		matches[i] = strings.ToLower(match)
	}
	return matches
}
