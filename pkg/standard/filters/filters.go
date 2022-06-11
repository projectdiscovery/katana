package filters

// Filter is an interface implemented by deduplication mechanism
type Filter interface {
	// Unique specifies whether a URL is unique
	Unique(url string) bool
}
