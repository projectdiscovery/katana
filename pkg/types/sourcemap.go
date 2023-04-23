package types

// SourceMap is a struct for source map
type SourceMap	 struct {
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent"`
}
