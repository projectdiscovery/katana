package similarity

import "math"

// termCounts maps term -> raw frequency in a document.
type termCounts map[string]int

func countTerms(tokens []string) termCounts {
	counts := make(termCounts, len(tokens))
	for _, t := range tokens {
		counts[t]++
	}
	return counts
}

type lexicalDoc struct {
	id     uint64
	counts termCounts
	length int
}

type lexicalCorpus struct {
	docs      []lexicalDoc
	docFreq   map[string]int
	avgDocLen float64
	totalLen  int
	nextID    uint64
}

func newLexicalCorpus() *lexicalCorpus {
	return &lexicalCorpus{
		docFreq: make(map[string]int),
	}
}

func (c *lexicalCorpus) Len() int {
	return len(c.docs)
}

// Add stores a document and returns its stable ID.
func (c *lexicalCorpus) Add(tokens []string) uint64 {
	counts := countTerms(tokens)
	length := 0
	for _, n := range counts {
		length += n
	}
	id := c.nextID
	c.nextID++
	c.docs = append(c.docs, lexicalDoc{id: id, counts: counts, length: length})
	c.totalLen += length
	for term := range counts {
		c.docFreq[term]++
	}
	if len(c.docs) > 0 {
		c.avgDocLen = float64(c.totalLen) / float64(len(c.docs))
	}
	return id
}

// EvictOldest removes the oldest document, repairs docFreq, and returns its stable ID.
func (c *lexicalCorpus) EvictOldest() (id uint64, ok bool) {
	if len(c.docs) == 0 {
		return 0, false
	}
	old := c.docs[0]
	c.docs = c.docs[1:]
	c.totalLen -= old.length
	for term := range old.counts {
		c.docFreq[term]--
		if c.docFreq[term] <= 0 {
			delete(c.docFreq, term)
		}
	}
	if len(c.docs) > 0 {
		c.avgDocLen = float64(c.totalLen) / float64(len(c.docs))
	} else {
		c.avgDocLen = 0
	}
	return old.id, true
}

func (c *lexicalCorpus) idf(term string) float64 {
	df := c.docFreq[term]
	n := float64(len(c.docs) + 1) // +1 for the query/candidate document
	return math.Log((n)/(1+float64(df))) + 1
}

// MaxCosine returns the best cosine score and the matched document's stable ID.
// ok is false when the corpus is empty or there is no usable query.
func (c *lexicalCorpus) MaxCosine(tokens []string) (score float64, id uint64, ok bool) {
	if len(c.docs) == 0 || len(tokens) == 0 {
		return 0, 0, false
	}
	query := countTerms(tokens)
	queryLen := 0
	for _, n := range query {
		queryLen += n
	}
	if queryLen == 0 {
		return 0, 0, false
	}

	best := 0.0
	var bestID uint64
	found := false
	for _, doc := range c.docs {
		s := cosineTFIDF(query, queryLen, doc.counts, doc.length, c)
		if !found || s > best {
			best = s
			bestID = doc.id
			found = true
		}
	}
	return best, bestID, found
}

func cosineTFIDF(a termCounts, aLen int, b termCounts, bLen int, c *lexicalCorpus) float64 {
	if aLen == 0 || bLen == 0 {
		return 0
	}
	var dot, normA, normB float64
	seen := make(map[string]struct{}, len(a)+len(b))
	for t := range a {
		seen[t] = struct{}{}
	}
	for t := range b {
		seen[t] = struct{}{}
	}
	for term := range seen {
		idf := c.idf(term)
		va := float64(a[term]) / float64(aLen) * idf
		vb := float64(b[term]) / float64(bLen) * idf
		dot += va * vb
		normA += va * va
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// MaxBM25 returns the best normalized BM25 score and the matched document's stable ID.
func (c *lexicalCorpus) MaxBM25(tokens []string) (score float64, id uint64, ok bool) {
	if len(c.docs) == 0 || len(tokens) == 0 {
		return 0, 0, false
	}
	query := countTerms(tokens)
	queryLen := 0
	for _, n := range query {
		queryLen += n
	}
	if queryLen == 0 {
		return 0, 0, false
	}
	self := bm25Score(query, query, queryLen, c)
	if self <= 0 {
		return 0, 0, false
	}

	best := 0.0
	var bestID uint64
	found := false
	for _, doc := range c.docs {
		raw := bm25Score(query, doc.counts, doc.length, c)
		norm := raw / self
		if norm > 1 {
			norm = 1
		}
		if !found || norm > best {
			best = norm
			bestID = doc.id
			found = true
		}
	}
	return best, bestID, found
}

func bm25Score(query, doc termCounts, docLen int, c *lexicalCorpus) float64 {
	if docLen == 0 || c.avgDocLen == 0 {
		return 0
	}
	var score float64
	for term, qf := range query {
		tf := float64(doc[term])
		if tf == 0 {
			continue
		}
		idf := c.idf(term)
		denom := tf + bm25K1*(1-bm25B+bm25B*float64(docLen)/c.avgDocLen)
		score += idf * (tf * (bm25K1 + 1) / denom) * float64(qf)
	}
	return score
}
