package utils

import (
	"regexp"
)

var (
	BodyA0 = `(?:`
	BodyB0 = `(`
	BodyC0 = `(?:[\.]{1,2}/[A-Za-z0-9\-_/\\?&@\.?=%]+)`
	BodyC1 = `|(https?://[A-Za-z0-9_\-\.]+([\.]{0,2})?\/[A-Za-z0-9\-_/\\?&@\.?=%]+)`
	BodyC2 = `|(/[A-Za-z0-9\-_/\\?&@\.%]+\.(aspx?|action|cfm|cgi|do|pl|css|x?html?|js(p|on)?|pdf|php5?|py|rss))`
	BodyC3 = `|([A-Za-z0-9\-_?&@\.%]+/[A-Za-z0-9/\\\-_?&@\.%]+\.(aspx?|action|cfm|cgi|do|pl|css|x?html?|js(p|on)?|pdf|php5?|py|rss))`
	BodyB1 = `)`
	BodyA1 = `)`
	// pageBodyRegex extracts endpoints from page body
	pageBodyRegex = regexp.MustCompile(BodyA0 + BodyB0 + BodyC0 + BodyC1 + BodyC2 + BodyC3 + BodyB1 + BodyA1)

	JsA0 = `(?:"|'|\s)`
	JsB0 = `(`
	JsC0 = `((https?://[A-Za-z0-9_\-\.]+(:\d{1,5})?)+([\.]{1,2})?/[A-Za-z0-9/\-_\.\\%]+([\?|#][^"']+)?)`
	JsC1 = `|((\.{1,2}/)?[a-zA-Z0-9\-_/\\%]+\.(aspx?|js(on|p)?|html|php5?|html|action|do)([\?|#][^"']+)?)`
	JsC2 = `|((\.{0,2}/)[a-zA-Z0-9\-_/\\%]+(/|\\)[a-zA-Z0-9\-_]{3,}([\?|#][^"|']+)?)`
	JsC3 = `|((\.{0,2})[a-zA-Z0-9\-_/\\%]{3,}/)`
	JsB1 = `)`
	JsA1 = `(?:"|'|\s)`
	// relativeEndpointsRegex is the regex to find endpoints in js files.
	relativeEndpointsRegex = regexp.MustCompile(JsA0 + JsB0 + JsC0 + JsC1 + JsC2 + JsC3 + JsB1 + JsA1)
)

// ExtractBodyEndpoints extracts body endpoints from a data item
func ExtractBodyEndpoints(data string) []string {
	matches := []string{}
	unique := make(map[string]struct{})

	relativeMatches := pageBodyRegex.FindAllStringSubmatch(data, -1)
	for _, match := range relativeMatches {
		if len(match) < 2 {
			continue
		}
		if _, ok := unique[match[1]]; ok {
			continue
		}
		unique[match[1]] = struct{}{}
		matches = append(matches, match[1])
	}
	return matches
}

// ExtractRelativeEndpoints extracts relative endpoints from a data item
func ExtractRelativeEndpoints(data string) []string {
	matches := []string{}
	unique := make(map[string]struct{})

	relativeMatches := relativeEndpointsRegex.FindAllStringSubmatch(data, -1)
	for _, match := range relativeMatches {
		if len(match) < 2 {
			continue
		}
		if _, ok := unique[match[1]]; ok {
			continue
		}

		unique[match[1]] = struct{}{}
		matches = append(matches, match[1])
	}
	return matches
}
