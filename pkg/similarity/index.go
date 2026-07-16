package similarity

import (
	"fmt"
	"strings"
	"sync"

	"github.com/projectdiscovery/gologger"
)

// Mode selects the content similarity backend.
type Mode string

const (
	ModeSimHash Mode = "simhash"
	ModeTFIDF   Mode = "tfidf"
	ModeBM25    Mode = "bm25"
)

const (
	DefaultMode            = ModeSimHash
	DefaultHammingDistance = 3
	DefaultScoreThreshold  = 0.85
	DefaultBudget          = 1
	DefaultMaxDocuments    = 1000
)

// Config configures a content similarity index.
type Config struct {
	Mode            Mode
	HammingDistance int
	ScoreThreshold  float64
	Budget          int
	MaxDocuments    int
}

// Stats are cumulative Accept counters.
type Stats struct {
	Processed int64
	Accepted  int64
	Filtered  int64
}

type simRep struct {
	id uint64
	fp uint64
}

// Index applies optional page-content similarity with a per-cluster parse budget.
type Index struct {
	cfg Config

	mu       sync.Mutex
	corpus   *lexicalCorpus
	sigs     []simRep
	nextID   uint64
	clusters map[string]int // stable clusterID -> accepted count
	stats    Stats
}

// ParseMode validates a mode string.
func ParseMode(s string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(s))) {
	case "", ModeSimHash:
		return ModeSimHash, nil
	case ModeTFIDF:
		return ModeTFIDF, nil
	case ModeBM25:
		return ModeBM25, nil
	default:
		return "", fmt.Errorf("unknown similarity mode %q (want simhash, tfidf, or bm25)", s)
	}
}

// New creates an Index. Invalid/zero config fields are filled with defaults.
func New(cfg Config) (*Index, error) {
	if cfg.Mode == "" {
		cfg.Mode = DefaultMode
	}
	mode, err := ParseMode(string(cfg.Mode))
	if err != nil {
		return nil, err
	}
	cfg.Mode = mode
	if cfg.HammingDistance <= 0 {
		cfg.HammingDistance = DefaultHammingDistance
	}
	if cfg.ScoreThreshold <= 0 || cfg.ScoreThreshold > 1 {
		cfg.ScoreThreshold = DefaultScoreThreshold
	}
	if cfg.Budget <= 0 {
		cfg.Budget = DefaultBudget
	}
	if cfg.MaxDocuments <= 0 {
		cfg.MaxDocuments = DefaultMaxDocuments
	}
	return &Index{
		cfg:      cfg,
		corpus:   newLexicalCorpus(),
		clusters: make(map[string]int),
	}, nil
}

func clusterKey(mode Mode, id uint64) string {
	return fmt.Sprintf("%s:%d", mode, id)
}

// Accept reports whether the page should be fully processed (parsed/enqueued).
// true = accept; false = skip as similar beyond cluster budget.
// Pages with too little signal are always accepted.
func (i *Index) Accept(body []byte) bool {
	if i == nil {
		return true
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	i.stats.Processed++

	doc, ok := Normalize(body)
	if !ok {
		i.stats.Accepted++
		return true
	}

	clusterID, matched := i.matchLocked(doc)
	if matched {
		if i.clusters[clusterID] >= i.cfg.Budget {
			i.stats.Filtered++
			gologger.Debug().Msgf("[similarity:%s] filtered cluster=%s budget=%d", i.cfg.Mode, clusterID, i.cfg.Budget)
			return false
		}
		i.clusters[clusterID]++
		i.stats.Accepted++
		return true
	}

	clusterID = i.rememberLocked(doc)
	i.clusters[clusterID] = 1
	i.stats.Accepted++
	return true
}

func (i *Index) matchLocked(doc Document) (clusterID string, matched bool) {
	switch i.cfg.Mode {
	case ModeSimHash:
		fp := SimHash64(doc.Shingles)
		bestDist := 64
		var bestID uint64
		found := false
		for _, existing := range i.sigs {
			d := HammingDistance(fp, existing.fp)
			if !found || d < bestDist {
				bestDist = d
				bestID = existing.id
				found = true
			}
		}
		if found && bestDist <= i.cfg.HammingDistance {
			return clusterKey(ModeSimHash, bestID), true
		}
		return "", false
	case ModeTFIDF:
		score, id, ok := i.corpus.MaxCosine(doc.Tokens)
		if ok && score >= i.cfg.ScoreThreshold {
			return clusterKey(ModeTFIDF, id), true
		}
		return "", false
	case ModeBM25:
		score, id, ok := i.corpus.MaxBM25(doc.Tokens)
		if ok && score >= i.cfg.ScoreThreshold {
			return clusterKey(ModeBM25, id), true
		}
		return "", false
	default:
		return "", false
	}
}

// rememberLocked stores a new representative and returns its stable cluster ID.
func (i *Index) rememberLocked(doc Document) string {
	switch i.cfg.Mode {
	case ModeSimHash:
		for len(i.sigs) >= i.cfg.MaxDocuments {
			old := i.sigs[0]
			delete(i.clusters, clusterKey(ModeSimHash, old.id))
			i.sigs = i.sigs[1:]
		}
		id := i.nextID
		i.nextID++
		i.sigs = append(i.sigs, simRep{id: id, fp: SimHash64(doc.Shingles)})
		return clusterKey(ModeSimHash, id)
	case ModeTFIDF, ModeBM25:
		for i.corpus.Len() >= i.cfg.MaxDocuments {
			if id, ok := i.corpus.EvictOldest(); ok {
				delete(i.clusters, clusterKey(i.cfg.Mode, id))
			}
		}
		id := i.corpus.Add(doc.Tokens)
		return clusterKey(i.cfg.Mode, id)
	default:
		return "unknown"
	}
}

// Stats returns a snapshot of counters.
func (i *Index) Stats() Stats {
	if i == nil {
		return Stats{}
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stats
}

// Mode returns the configured mode.
func (i *Index) Mode() Mode {
	if i == nil {
		return ""
	}
	return i.cfg.Mode
}

// Close releases index resources.
func (i *Index) Close() {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.sigs = nil
	i.corpus = newLexicalCorpus()
	i.clusters = make(map[string]int)
	i.nextID = 0
}
