//go:build !jsluice || 386 || windows

package utils

import (
	"regexp"
	"strings"
)

var (
	// URL patterns for extracting endpoints from JavaScript
	urlPattern = regexp.MustCompile(`(?i)(?:"|'|` + "`" + `)((?:https?:)?//[^\s"'` + "`" + `<>]+|/[^\s"'` + "`" + `<>]+)(?:"|'|` + "`" + `)`)
	
	// API endpoint patterns
	apiPattern = regexp.MustCompile(`(?i)(?:api|endpoint|url|path|route)[\s]*[:=][\s]*(?:"|'|` + "`" + `)([^"'` + "`" + `]+)(?:"|'|` + "`" + `)`)
)

type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript using pure Go regex.
// This is a fallback implementation when jsluice (which requires CGO) is not available.
//
// Note: This implementation uses regex patterns and may not be as accurate as jsluice,
// but it eliminates the CGO dependency for cross-platform compilation.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	var endpoints []JSLuiceEndpoint
	seen := make(map[string]bool)

	// Extract URLs using URL pattern
	urlMatches := urlPattern.FindAllStringSubmatch(data, -1)
	for _, match := range urlMatches {
		if len(match) > 1 {
			url := match[1]
			if !seen[url] && isValidEndpoint(url) {
				seen[url] = true
				endpoints = append(endpoints, JSLuiceEndpoint{
					Endpoint: url,
					Type:     "url",
				})
			}
		}
	}

	// Extract API endpoints
	apiMatches := apiPattern.FindAllStringSubmatch(data, -1)
	for _, match := range apiMatches {
		if len(match) > 1 {
			endpoint := match[1]
			if !seen[endpoint] && isValidEndpoint(endpoint) {
				seen[endpoint] = true
				endpoints = append(endpoints, JSLuiceEndpoint{
					Endpoint: endpoint,
					Type:     "api",
				})
			}
		}
	}

	return endpoints
}

// isValidEndpoint checks if the extracted string is a valid endpoint
func isValidEndpoint(s string) bool {
	// Filter out common false positives
	if len(s) < 2 {
		return false
	}
	
	// Normalize to lowercase for case-insensitive comparison
	lower := strings.ToLower(s)
	
	// Skip data URIs and javascript URIs
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "javascript:") {
		return false
	}
	
	// Skip very long strings (likely not URLs)
	if len(s) > 2048 {
		return false
	}
	
	return true
}
