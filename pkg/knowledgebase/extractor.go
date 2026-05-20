// Package knowledgebase defines the Extractor interface implemented by
// per-class detectors that contribute structured facts to a response's
// KnowledgeBase map. Each extractor owns a single top-level key, named by
// Name(), to keep outputs collision-free across detectors.
package knowledgebase

import "net/http"

// Extractor mines structured facts from a crawled response. Extractors that
// only need the body can ignore req and resp; extractors that classify by
// request shape (method, headers, URL) use them.
type Extractor interface {
	Name() string
	Extract(body string, req *http.Request, resp *http.Response) map[string]any
}
