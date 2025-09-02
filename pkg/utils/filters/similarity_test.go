package filters

import (
	"testing"
)

func TestSimilarityFilter(t *testing.T) {
	// Use a much lower threshold since TF-IDF cosine similarity scores tend to be low
	filter, err := NewSimilarity(0.05)
	if err != nil {
		t.Fatalf("Failed to create similarity filter: %v", err)
	}
	defer filter.Close()

	// Test case 1: Different content should be unique
	content1 := []byte("This is a unique piece of content about web crawling technology and modern web development")
	content2 := []byte("Machine learning and artificial intelligence are transforming business operations worldwide")

	if !filter.UniqueContent(content1) {
		t.Error("First unique content should be accepted")
	}

	if !filter.UniqueContent(content2) {
		t.Error("Second unique content should be accepted")
	}

	// Test case 2: Identical content should be filtered
	content3 := []byte("This is a unique piece of content about web crawling technology and modern web development")

	// This should be filtered as it's identical to content1
	if filter.UniqueContent(content3) {
		t.Error("Identical content should be filtered")
	}

	// Test case 3: Empty content
	emptyContent := []byte("")
	if !filter.UniqueContent(emptyContent) {
		t.Error("Empty content should be considered unique")
	}

	// Test case 4: Check statistics
	totalProcessed, uniqueDocuments, similarFiltered := filter.GetStats()

	if totalProcessed != 4 {
		t.Errorf("Expected 4 total processed, got %d", totalProcessed)
	}

	if uniqueDocuments != 3 {
		t.Errorf("Expected 3 unique documents, got %d", uniqueDocuments)
	}

	if similarFiltered != 1 {
		t.Errorf("Expected 1 similar filtered, got %d", similarFiltered)
	}

	t.Logf("Similarity filter stats: %d processed, %d unique, %d filtered",
		totalProcessed, uniqueDocuments, similarFiltered)
}

func TestSimilarityFilterThreshold(t *testing.T) {
	// Test with a lower threshold (more strict)
	strictFilter, err := NewSimilarity(0.3)
	if err != nil {
		t.Fatalf("Failed to create strict similarity filter: %v", err)
	}
	defer strictFilter.Close()

	content1 := []byte("Web crawling is an important technique")
	content2 := []byte("Web scraping is an important method")

	if !strictFilter.UniqueContent(content1) {
		t.Error("First content should be accepted")
	}

	// With lower threshold, this might be filtered
	isUnique := strictFilter.UniqueContent(content2)

	t.Logf("With threshold 0.3, second content unique: %v", isUnique)

	// Test with higher threshold (more permissive)
	permissiveFilter, err := NewSimilarity(0.9)
	if err != nil {
		t.Fatalf("Failed to create permissive similarity filter: %v", err)
	}
	defer permissiveFilter.Close()

	if !permissiveFilter.UniqueContent(content1) {
		t.Error("First content should be accepted")
	}

	// With higher threshold, this should pass
	if !permissiveFilter.UniqueContent(content2) {
		t.Error("With threshold 0.9, second content should be accepted")
	}
}

func TestSimilarityFilterURLs(t *testing.T) {
	filter, err := NewSimilarity(0.7)
	if err != nil {
		t.Fatalf("Failed to create similarity filter: %v", err)
	}
	defer filter.Close()

	// Test URL uniqueness (should work like simple filter)
	url1 := "https://example.com/page1"
	url2 := "https://example.com/page2"
	url3 := "https://example.com/page1" // duplicate

	if !filter.UniqueURL(url1) {
		t.Error("First URL should be unique")
	}

	if !filter.UniqueURL(url2) {
		t.Error("Second URL should be unique")
	}

	if filter.UniqueURL(url3) {
		t.Error("Third URL should be duplicate")
	}
}
