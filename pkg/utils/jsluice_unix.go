//go:build !windows

package utils

import "github.com/BishopFox/jsluice"

func extractJsluiceEndpoints(data string) []JSLuiceEndpoint {
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
