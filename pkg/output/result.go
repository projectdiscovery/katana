package output

import "time"

// Result is a result structure for the crawler
type Result struct {
	// Timestamp is the current timestamp
	Timestamp time.Time `json:"timestamp,omitempty"`
	// Method is the method for the result
	Method string `json:"method,omitempty"`
	// Body contains the body for the request
	Body string `json:"body,omitempty"`
	// URL is the URL of the result
	URL string `json:"endpoint,omitempty"`
	// Source is the source for the result
	Source string `json:"source,omitempty"`
	// Tag is the tag for the result
	Tag string `json:"tag,omitempty"`
	// Attribute is the attribute for the result
	Attribute string `json:"attribute,omitempty"`
	// customField matched output
	CustomFields map[string][]string `json:"-"`
}
