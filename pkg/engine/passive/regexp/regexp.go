package regexp

import (
	"regexp"
	"strings"
)

var re, _ = regexp.Compile(`(?:http|https)?://(?:www\.)?[a-zA-Z0-9./?=_%:-]*`)

func Extract(text string) []string {
	matches := re.FindAllString(text, -1)
	for i, match := range matches {
		matches[i] = strings.ToLower(match)
	}
	return matches
}
