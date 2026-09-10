package cartography

import (
	"sync"

	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer/simhash"
)

// ExplosionBudget limits how many near-duplicate locations are fully explored.
type ExplosionBudget struct {
	mu         sync.Mutex
	maxPerCluster int
	maxHamming    int
	clusters      []clusterRep
	counts        map[uint64]int // rep simhash -> accepted count
}

type clusterRep struct {
	fp uint64
}

// NewExplosionBudget creates a budget. maxPerCluster defaults to 1, maxHamming to 3.
func NewExplosionBudget(maxPerCluster, maxHamming int) *ExplosionBudget {
	if maxPerCluster <= 0 {
		maxPerCluster = 1
	}
	if maxHamming < 0 {
		maxHamming = DefaultMaxHamming
	}
	return &ExplosionBudget{
		maxPerCluster: maxPerCluster,
		maxHamming:    maxHamming,
		counts:        make(map[uint64]int),
	}
}

// Allow reports whether a location with the given simhash should be fully processed.
// First hit in a cluster is allowed; further near-dupes are rejected once the budget is spent.
func (b *ExplosionBudget) Allow(fp uint64) bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, rep := range b.clusters {
		if int(simhash.Distance(rep.fp, fp)) <= b.maxHamming {
			if b.counts[rep.fp] >= b.maxPerCluster {
				return false
			}
			b.counts[rep.fp]++
			return true
		}
	}
	b.clusters = append(b.clusters, clusterRep{fp: fp})
	b.counts[fp] = 1
	return true
}

// ClusterCount returns how many distinct fingerprint clusters were seen.
func (b *ExplosionBudget) ClusterCount() int {
	if b == nil {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clusters)
}
