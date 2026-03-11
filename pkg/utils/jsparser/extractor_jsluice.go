//go:build cgo

package jsparser

import "github.com/BishopFox/jsluice"

type jsluiceExtractor struct{}

// CGO_ENABLED=1 i.e. enabled, using jsluiceExtractor
func New() Extractor {
	return &jsluiceExtractor{}
}

func (e *jsluiceExtractor) Name() string {
	return "jsluice"
}

func (e *jsluiceExtractor) Extract(data string) []Endpoint {
	analyzer := jsluice.NewAnalyzer([]byte(data))

	var endpoints []Endpoint
	for _, url := range analyzer.GetURLs() {
		endpoint := NormalizeEndpoint(url.URL)
		if endpoint == "" {
			continue
		}
		endpoints = append(endpoints, Endpoint{
			Endpoint: endpoint,
			Type:     url.Type,
		})
	}

	return DedupeEndpoints(endpoints)
}
