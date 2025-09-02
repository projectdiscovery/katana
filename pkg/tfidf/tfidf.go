package tfidf

import (
	"math"
	"strings"
	"sync"
)

const (
	// MaxDocuments limits the number of documents stored to prevent unbounded memory growth
	MaxDocuments = 1000
)

type TfIdf struct {
	documents map[string]map[string]int
	docFreq   map[string]int
	totalDocs int
	mutex     sync.Mutex
}

func New() *TfIdf {
	return &TfIdf{
		documents: make(map[string]map[string]int),
		docFreq:   make(map[string]int),
	}
}

func (t *TfIdf) AddDocument(docID string, words []string) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Prevent unbounded memory growth by limiting stored documents
	if len(t.documents) >= MaxDocuments {
		// Remove the oldest document (simple FIFO approach)
		// In a production system, you might want LRU or other strategies
		for oldDocID := range t.documents {
			delete(t.documents, oldDocID)
			break // Remove just one document
		}
	}

	if len(words) == 0 {
		return // Skip empty documents
	}

	// Safety limit: prevent storing overly large documents
	const MaxWordsPerDocument = 50000
	if len(words) > MaxWordsPerDocument {
		words = words[:MaxWordsPerDocument]
	}

	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(word)
		wordCount[word]++
	}

	t.documents[docID] = wordCount
	t.totalDocs++

	for word := range wordCount {
		t.docFreq[word]++
	}
}

func (t *TfIdf) Calculate(words []string) []float64 {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if len(t.documents) == 0 || len(words) == 0 {
		return []float64{} // No existing documents to compare against or empty input
	}

	// Safety limit: skip very large documents to prevent excessive memory usage
	const MaxWordsPerDocument = 50000
	if len(words) > MaxWordsPerDocument {
		// Still process, but limit to first N words
		words = words[:MaxWordsPerDocument]
	}

	newDocWordCount := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(word)
		newDocWordCount[word]++
	}

	var similarities []float64

	// Compare new document against each existing document
	for _, existingDoc := range t.documents {
		similarity := t.calculateCosineSimilarity(newDocWordCount, existingDoc, len(words))
		similarities = append(similarities, similarity)
	}

	return similarities
}

// calculateCosineSimilarity computes cosine similarity between two documents
func (t *TfIdf) calculateCosineSimilarity(doc1 map[string]int, doc2 map[string]int, doc1WordCount int) float64 {
	// Calculate TF-IDF vectors for both documents
	doc1TfIdf := make(map[string]float64)
	doc2TfIdf := make(map[string]float64)

	// Get all unique words from both documents
	allWords := make(map[string]struct{})
	for word := range doc1 {
		allWords[word] = struct{}{}
	}
	for word := range doc2 {
		allWords[word] = struct{}{}
	}

	doc2WordCount := 0
	for _, count := range doc2 {
		doc2WordCount += count
	}

	// Safety check for division by zero
	if doc1WordCount == 0 || doc2WordCount == 0 {
		return 0 // Cannot calculate similarity for documents with no words
	}

	// Calculate TF-IDF for each word
	for word := range allWords {
		// Document 1 TF-IDF
		tf1 := float64(doc1[word]) / float64(doc1WordCount)
		idf := math.Log(float64(t.totalDocs+1) / (1 + float64(t.docFreq[word]))) // +1 for the new document
		doc1TfIdf[word] = tf1 * idf

		// Document 2 TF-IDF
		tf2 := float64(doc2[word]) / float64(doc2WordCount)
		doc2TfIdf[word] = tf2 * idf
	}

	// Calculate cosine similarity
	var dotProduct, norm1, norm2 float64

	for word := range allWords {
		val1 := doc1TfIdf[word]
		val2 := doc2TfIdf[word]

		dotProduct += val1 * val2
		norm1 += val1 * val1
		norm2 += val2 * val2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}
