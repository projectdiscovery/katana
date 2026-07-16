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

type lexicalCorpus struct {
	docs      []termCounts
	docFreq   map[string]int
	docLen    []int
	avgDocLen float64
	totalLen  int
}

func newLexicalCorpus() *lexicalCorpus {
	return &lexicalCorpus{
		docFreq: make(map[string]int),
	}
}

func (c *lexicalCorpus) Len() int {
	return len(c.docs)
}

func (c *lexicalCorpus) Add(tokens []string) int {
	counts := countTerms(tokens)
	length := 0
	for _, n := range counts {
		length += n
	}
	id := len(c.docs)
	c.docs = append(c.docs, counts)
	c.docLen = append(c.docLen, length)
	c.totalLen += length
	for term := range counts {
		c.docFreq[term]++
	}
	if len(c.docs) > 0 {
		c.avgDocLen = float64(c.totalLen) / float64(len(c.docs))
	}
	return id
}

// EvictOldest removes the oldest document and repairs docFreq.
func (c *lexicalCorpus) EvictOldest() {
	if len(c.docs) == 0 {
		return
	}
	old := c.docs[0]
	oldLen := c.docLen[0]
	c.docs = c.docs[1:]
	c.docLen = c.docLen[1:]
	c.totalLen -= oldLen
	for term := range old {
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
}

func (c *lexicalCorpus) idf(term string) float64 {
	df := c.docFreq[term]
	n := float64(len(c.docs) + 1) // +1 for the query/candidate document
	return math.Log((n)/(1+float64(df))) + 1
}

func (c *lexicalCorpus) MaxCosine(tokens []string) (float64, int) {
	if len(c.docs) == 0 || len(tokens) == 0 {
		return 0, -1
	}
	query := countTerms(tokens)
	queryLen := 0
	for _, n := range query {
		queryLen += n
	}
	if queryLen == 0 {
		return 0, -1
	}

	best := 0.0
	bestID := -1
	for id, doc := range c.docs {
		score := cosineTFIDF(query, queryLen, doc, c.docLen[id], c)
		if score > best {
			best = score
			bestID = id
		}
	}
	return best, bestID
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

func (c *lexicalCorpus) MaxBM25(tokens []string) (float64, int) {
	if len(c.docs) == 0 || len(tokens) == 0 {
		return 0, -1
	}
	query := countTerms(tokens)
	queryLen := 0
	for _, n := range query {
		queryLen += n
	}
	if queryLen == 0 {
		return 0, -1
	}
	// Normalize by the query's self-score so unrelated docs stay low.
	self := bm25Score(query, query, queryLen, c)
	if self <= 0 {
		return 0, -1
	}

	best := 0.0
	bestID := -1
	for id, doc := range c.docs {
		raw := bm25Score(query, doc, c.docLen[id], c)
		norm := raw / self
		if norm > 1 {
			norm = 1
		}
		if norm > best {
			best = norm
			bestID = id
		}
	}
	return best, bestID
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
