// Package knowledgebase defines the Extractor interface implemented by
// per-class detectors that contribute structured facts to a response's
// KnowledgeBase map. Each extractor owns a single top-level key, named by
// Name(), to keep outputs collision-free across detectors.
package knowledgebase

import "net/http"

// Extractor mines structured facts from a crawled response. Extractors that
// only need the body can ignore req and resp; extractors that classify by
// request shape (method, headers, URL) use them.
//
// Implementations MUST treat body, req, and resp as read-only: do not mutate
// req/resp headers, URL, or body, and do not read resp.Body. The response
// body has already been drained upstream and is passed as the body argument;
// resp is supplied for status, header, and Request access only.
type Extractor interface {
	Name() string
	Extract(body string, req *http.Request, resp *http.Response) map[string]any
}
