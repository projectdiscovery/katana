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
	// IsCycle attempts to detect if the current URL is a cycle
	// until graph navigation is implemented, the only ways to discard a potential
	// loop cycle are
	// - implementing upper hard limit to the URL length (https://bugs.chromium.org/p/chromium/issues/detail?id=69227 => 2Mb)
	// - Heuristically find the longest repeating substring and set a max threshold of how many max times it should repeat (eg. 10)
	// Todo: This should be replace with graph cycle detection => https://github.com/projectdiscovery/katana/pull/174
	IsCycle(url string) bool
}
