package filters

import (
	"crypto/md5"
	"encoding/hex"

	"github.com/projectdiscovery/hmap/store/hybrid"
)

// Simple is a simple unique URL filter.
//
// The URLs are maintained in a global sync.Map for
// deduplication and no normalization is performed.
type Simple struct {
	data *hybrid.HybridMap
}

// NewSimple returns a new simple filter
func NewSimple() (*Simple, error) {
	hmap, err := hybrid.New(hybrid.DefaultDiskOptions)
	if err != nil {
		return nil, err
	}
	return &Simple{data: hmap}, nil
}

// UniqueURL returns true if the URL is unique
func (s *Simple) UniqueURL(url string) bool {
	_, found := s.data.Get(url)
	if found {
		return false
	}
	_ = s.data.Set(url, nil)
	return true
}

// UniqueContent returns true if the content is unique
func (s *Simple) UniqueContent(data []byte) bool {
	hash := md5.Sum([]byte(data))
	encoded := hex.EncodeToString(hash[:])

	_, found := s.data.Get(encoded)
	if found {
		return false
	}
	_ = s.data.Set(encoded, nil)
	return true
}

// Close closes the filter and relases associated resources
func (s *Simple) Close() {
	_ = s.data.Close()
}
