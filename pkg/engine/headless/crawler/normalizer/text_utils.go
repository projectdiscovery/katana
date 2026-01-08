package normalizer

import (
	"fmt"
	"regexp"
	"slices"
)

// DefaultTextPatterns is a list of regex patterns for the text normalizer
var DefaultTextPatterns = []string{
	// emailAddress
	`\b(?i)[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`,
	// ipAddress
	`\b(?:25[0-5]|2[0-4]\d|1?\d?\d)(?:\.(?:25[0-5]|2[0-4]\d|1?\d?\d)){3}\b`,
	// uuid
	`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
	// relativeDates
	`\b(?:[0-9]{1,2}\s(?:days?|weeks?|months?|years?)\s(?:ago|from\s+now))\b`,
	// priceAmounts (no leading \b due to currency symbols)
	`[\$€£¥]\s*\d+(?:\.\d{1,2})?\b`,
	// phoneNumbers
	`\b\+?\d{7,15}\b`,
	// ssnNumbers
	`\b\d{3}-\d{2}-\d{4}\b`,
	// timestampRegex
	`\b(?:(?:[0-9]{4}-[0-9]{2}-[0-9]{2})|(?:(?:[0-9]{2}\/){2}[0-9]{4}))\s(?:[0-9]{2}:[0-9]{2}:[0-9]{2})\b`,
}

// TextNormalizer is a normalizer for text
type TextNormalizer struct {
	// patterns is a list of regex patterns for the text normalizer
	patterns []*regexp.Regexp
}

// NewTextNormalizer returns a new TextNormalizer
//
// patterns is a list of regex patterns for the text normalizer
// DefaultTextPatterns is used if patterns is nil. See DefaultTextPatterns for more info.
func NewTextNormalizer() (*TextNormalizer, error) {
	patterns := slices.Clone(DefaultTextPatterns)
	patterns = append(patterns, dateTimePatterns...)

	var compiledPatterns []*regexp.Regexp
	for _, pattern := range patterns {
		pattern := pattern
		compiledPattern, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("error compiling pattern %s: %v", pattern, err)
		}
		compiledPatterns = append(compiledPatterns, compiledPattern)
	}
	return &TextNormalizer{patterns: compiledPatterns}, nil
}

// Apply applies the patterns to the text and returns the normalized text
func (n *TextNormalizer) Apply(text string) string {
	for _, pattern := range n.patterns {
		pattern := pattern
		text = pattern.ReplaceAllString(text, "")
	}
	return text
}
