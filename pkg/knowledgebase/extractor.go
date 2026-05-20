// Package knowledgebase defines the Extractor interface implemented by
// per-class detectors that contribute structured facts to a response's
// KnowledgeBase map. Each extractor owns a single top-level key, named by
// Name(), to keep outputs collision-free across detectors.
package knowledgebase

type Extractor interface {
	Name() string
	Extract(body string) map[string]any
}
