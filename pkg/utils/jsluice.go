package utils

import (
	"github.com/BishopFox/jsluice"
)

type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts jsluice endpoints from a given string.
//
// We use tomnomnom and bishopfox's jsluice to extract endpoints from javascript
// files.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	analyzer := jsluice.NewAnalyzer([]byte(data))

	// TODO: add new user url matchers
	// analyzer.AddURLMatcher(matcher)

	var endpoints []JSLuiceEndpoint
	foundURLs := analyzer.GetURLs()

	for _, url := range foundURLs {
		url := url
		endpoints = append(endpoints, JSLuiceEndpoint{
			Endpoint: url.URL,
			Type:     url.Type,
		})
	}
	return endpoints
}
