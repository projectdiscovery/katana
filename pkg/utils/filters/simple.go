package filters

import "sync"

// Simple is a simple unique URL filter.
//
// The URLs are maintained in a global sync.Map for
// deduplication and no normalization is performed.
type Simple struct {
	data sync.Map
}

// NewSimple returns a new simple filter
func NewSimple() *Simple {
	return &Simple{data: sync.Map{}}
}

// Unique returns true if the URL is unique
func (s *Simple) Unique(url string) bool {
	_, loaded := s.data.LoadOrStore(url, struct{}{})
	return !loaded
}
