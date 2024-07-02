package tfidf

import (
	"math"
	"strings"
	"sync"
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

    wordCount := make(map[string]int)
    for _, word := range words {
        word = strings.ToLower(word)
        wordCount[word]++
    }

    var scores []float64
    for range t.documents {
        score := 0.0
        for word, count := range wordCount {
            tf := float64(count) / float64(len(words))
            idf := math.Log(float64(t.totalDocs) / (1 + float64(t.docFreq[word])))
            score += tf * idf
        }
        scores = append(scores, score)
    }
    return scores
}
