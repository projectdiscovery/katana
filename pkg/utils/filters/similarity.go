package filters

import (
	"strings"
	"sync"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/hmap/store/hybrid"
	"github.com/projectdiscovery/katana/pkg/tfidf"
)

// SimilarityFilter implements content similarity detection using TF-IDF
type SimilarityFilter struct {
	urlMap              *hybrid.HybridMap
	contentMap          *hybrid.HybridMap
	tfidfModel          *tfidf.TfIdf
	similarityThreshold float64
	mutex               sync.RWMutex
	// Statistics
	totalProcessed  int64
	similarFiltered int64
	uniqueDocuments int64
	// Current URL context for verbose logging
	currentURL string
}

// NewSimilarity creates a new similarity filter
func NewSimilarity(threshold float64) (*SimilarityFilter, error) {
	urlMap, err := hybrid.New(hybrid.DefaultDiskOptions)
	if err != nil {
		return nil, err
	}
	contentMap, err := hybrid.New(hybrid.DefaultDiskOptions)
	if err != nil {
		return nil, err
	}

	return &SimilarityFilter{
		urlMap:              urlMap,
		contentMap:          contentMap,
		tfidfModel:          tfidf.New(),
		similarityThreshold: threshold,
	}, nil
}

// UniqueURL returns true if the URL is unique
func (s *SimilarityFilter) UniqueURL(url string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, found := s.urlMap.Get(url)
	if found {
		return false
	}
	_ = s.urlMap.Set(url, nil)
	return true
}

// UniqueContent returns true if the content is unique based on similarity analysis
func (s *SimilarityFilter) UniqueContent(data []byte) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.totalProcessed++

	// Safety check for extremely large content
	const MaxContentSize = 10 * 1024 * 1024 // 10MB limit
	if len(data) > MaxContentSize {
		gologger.Debug().Msgf("[similarity] Content too large (%d bytes), considering unique", len(data))
		s.uniqueDocuments++
		return true // Don't process extremely large content
	}

	content := string(data)
	words := strings.Fields(content)
	if len(words) == 0 {
		gologger.Debug().Msg("[similarity] Empty content, considering unique")
		s.uniqueDocuments++
		return true // Empty content is considered unique
	}

	// Calculate TF-IDF similarity scores against existing documents with panic recovery
	var scores []float64
	func() {
		defer func() {
			if r := recover(); r != nil {
				gologger.Warning().Msgf("[similarity] Recovered from panic in similarity calculation: %v", r)
				scores = []float64{} // Return empty scores on panic
			}
		}()
		scores = s.tfidfModel.Calculate(words)
	}()

	// Find the highest similarity score
	var maxScore float64
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}

	// Debug output showing similarity analysis
	if len(scores) > 0 {
		gologger.Debug().Msgf("[similarity] Content analysis: %d words, max_similarity=%.3f, threshold=%.3f",
			len(words), maxScore, s.similarityThreshold)
	}

	// Check if any score exceeds the similarity threshold
	if maxScore > s.similarityThreshold {
		s.similarFiltered++
		if s.currentURL != "" {
			gologger.Debug().Msgf("[similarity] Skipped %s (similarity score: %.3f > %.3f)",
				s.currentURL, maxScore, s.similarityThreshold)
		} else {
			gologger.Debug().Msgf("[similarity] Filtered similar content (score=%.3f > %.3f)",
				maxScore, s.similarityThreshold)
		}
		return false // Content is too similar to existing content
	}

	// Add this document to the TF-IDF model for future comparisons with panic recovery
	// Use a simple counter as document ID
	docID := strings.Join(words[:min(5, len(words))], "-") // Use first 5 words as ID

	func() {
		defer func() {
			if r := recover(); r != nil {
				gologger.Warning().Msgf("[similarity] Recovered from panic in document addition: %v", r)
			}
		}()
		s.tfidfModel.AddDocument(docID, words)
	}()

	s.uniqueDocuments++
	gologger.Debug().Msgf("[similarity] Added unique document: %s", docID)
	return true
}

// IsCycle attempts to determine if the url is a cycle loop
func (s *SimilarityFilter) IsCycle(url string) bool {
	// Use the same logic as Simple filter
	if len(url) > MaxChromeURLLength {
		return true
	}

	// Note: We could add TF-IDF based cycle detection here in the future
	return false
}

// Close closes the filter and releases associated resources
func (s *SimilarityFilter) Close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Don't log here anymore - stats will be included in the main completion message

	_ = s.urlMap.Close()
	_ = s.contentMap.Close()
}

// GetStats returns current similarity filter statistics
func (s *SimilarityFilter) GetStats() (totalProcessed, uniqueDocuments, similarFiltered int64) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.totalProcessed, s.uniqueDocuments, s.similarFiltered
}

// GetMemoryUsage returns an estimate of memory usage in bytes
func (s *SimilarityFilter) GetMemoryUsage() int64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Rough estimation: assume average 50 words per document, 10 bytes per word
	estimatedBytes := s.uniqueDocuments * 50 * 10

	// Add URL storage overhead (average 100 bytes per URL)
	urlBytes := s.totalProcessed * 100

	return estimatedBytes + urlBytes
}

// SetCurrentURL sets the URL context for verbose logging
func (s *SimilarityFilter) SetCurrentURL(url string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.currentURL = url
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
