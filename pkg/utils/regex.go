package utils

import "regexp"

var (
	// relativeEndpointsRegex is the regex to find endpoints in js files.
	relativeEndpointsRegex = regexp.MustCompile(
		`(?:"|'| )((?:(?:(?:https?:\/\/[A-Za-z0-9_\-\.]+)?(?:[\.]{0,2})?\/[A-Za-z0-9\/\-_\.]+)|[A-Za-z0-9\-_\/]+\.(?:aspx?|js(?:on|p)?|html|php5?|html|action|do)(?:[\?|#][^"|']+)?))(?:"|'| |)`,
	)
)

// ExtractRelativeEndpoints extracts relative endpoints from a data item
func ExtractRelativeEndpoints(data string) []string {
	matches := []string{}
	relativeMatches := relativeEndpointsRegex.FindAllStringSubmatch(data, -1)
	for _, match := range relativeMatches {
		if len(match) < 2 {
			continue
		}
		matches = append(matches, match[1])
	}
	return matches
}
