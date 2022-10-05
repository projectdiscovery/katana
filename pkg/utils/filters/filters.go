package filters

// Filter is an interface implemented by deduplication mechanism
type Filter interface {
	// Close closes the filter and releases associated resources
	Close()
	// Unique specifies whether a URL is unique
	Unique(url string) bool
}
