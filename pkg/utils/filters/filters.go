package filters

// Filter is an interface implemented by deduplication mechanism
type Filter interface {
	// Close closes the filter and releases associated resources
	Close()
	// UniqueURL specifies whether a URL is unique
	UniqueURL(url string) bool
	// UniqueContent specifies whether a content is unique
	// Deduplication is done by hashing of the response data.
	//
	// TODO: Consider levenshtein length / keyword based hashing
	// to account for dynamic response content.
	UniqueContent(content []byte) bool
}
