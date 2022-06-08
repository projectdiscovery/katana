package utils

import (
	"strings"
)

// ParseLinkTag parses link tag values returning found urls
//
// Inspired from: https://github.com/tomnomnom/linkheader
func ParseLinkTag(value string) []string {
	urls := make([]string, 0)

	for _, chunk := range strings.Split(value, ",") {
		for _, piece := range strings.Split(chunk, ";") {
			piece = strings.Trim(piece, " ")
			if piece == "" {
				continue
			}
			if piece[0] == '<' && piece[len(piece)-1] == '>' {
				urls = append(urls, strings.Trim(piece, "<>"))
				continue
			}
		}
	}
	return urls
}

// ParseRefreshTag parses refresh tag values returning found urls
func ParseRefreshTag(value string) string {
	chunks := strings.Split(value, "url=")
	if len(chunks) < 2 {
		return ""
	}
	chunk := chunks[1]
	chunk = strings.TrimSuffix(chunk, ";")
	if chunk == "" {
		return ""
	}
	return chunk
}
