package utils

import (
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

// DefaultPromotionThreshold is the number of distinct children a trie node
// must accumulate before being promoted to a parameter node.
const DefaultPromotionThreshold = 10

// DefaultMaxHosts is the maximum number of hosts tracked by the LRU cache.
// When exceeded, the least recently used host's trie is evicted.
const DefaultMaxHosts = 10000

// PathTrie is a per-host adaptive trie that tracks unique path segments
// at each position. When a node accumulates more distinct children than
// the promotion threshold, it is promoted to a parameter node and all
// future values at that position are collapsed.
//
// Host tracking is bounded by an LRU cache to prevent unbounded memory
// growth during large crawls.
type PathTrie struct {
	mu        sync.Mutex
	roots     *lru.Cache[string, *trieNode]
	threshold int
}

type trieNode struct {
	children   map[string]*trieNode
	paramChild *trieNode
	promoted   bool
}

// NewPathTrie creates a new PathTrie with the given promotion threshold.
// If threshold is <= 0, DefaultPromotionThreshold is used.
func NewPathTrie(threshold int) *PathTrie {
	if threshold <= 0 {
		threshold = DefaultPromotionThreshold
	}
	cache, _ := lru.New[string, *trieNode](DefaultMaxHosts)
	return &PathTrie{
		roots:     cache,
		threshold: threshold,
	}
}

// Fingerprint walks the trie for the given host and segments, returning
// a new slice where promoted positions are replaced with "{param}".
// Non-promoted segments are registered in the trie for future cardinality tracking.
func (t *PathTrie) Fingerprint(host string, segments []string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	root, ok := t.roots.Get(host)
	if !ok {
		root = &trieNode{children: make(map[string]*trieNode)}
		t.roots.Add(host, root)
	}

	result := make([]string, len(segments))
	current := root

	for i, seg := range segments {
		if current.promoted {
			result[i] = "{param}"
			if current.paramChild == nil {
				current.paramChild = &trieNode{children: make(map[string]*trieNode)}
			}
			current = current.paramChild
			continue
		}

		child, exists := current.children[seg]
		if !exists {
			child = &trieNode{children: make(map[string]*trieNode)}
			current.children[seg] = child

			if len(current.children) > t.threshold {
				current.promoted = true
				current.paramChild = &trieNode{children: make(map[string]*trieNode)}
				current.children = nil
				result[i] = "{param}"
				current = current.paramChild
				continue
			}
		}

		result[i] = seg
		current = child
	}

	return result
}
