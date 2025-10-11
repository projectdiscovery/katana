package proxy

import (
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
)

func TestFilterCache(t *testing.T) {
	cache := NewFilterCache(3, time.Minute)

	// Test cache miss
	result, found := cache.Get("http://example.com", "example.com")
	if found {
		t.Error("Expected cache miss, got hit")
	}

	// Test cache set and hit
	cache.Set("http://example.com", "example.com", true)
	result, found = cache.Get("http://example.com", "example.com")
	if !found {
		t.Error("Expected cache hit, got miss")
	}
	if !result {
		t.Error("Expected cached result to be true")
	}

	// Test cache eviction
	cache.Set("http://example.com/1", "example.com", true)
	cache.Set("http://example.com/2", "example.com", false)
	cache.Set("http://example.com/3", "example.com", true)
	
	// This should evict the oldest entry
	cache.Set("http://example.com/4", "example.com", false)
	
	stats := cache.GetStats()
	if stats.Size != 3 {
		t.Errorf("Expected cache size 3, got %d", stats.Size)
	}
}

func TestFilterCacheExpiration(t *testing.T) {
	cache := NewFilterCache(10, 50*time.Millisecond)

	// Set a value
	cache.Set("http://example.com", "example.com", true)
	
	// Should be found immediately
	_, found := cache.Get("http://example.com", "example.com")
	if !found {
		t.Error("Expected cache hit immediately after set")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)
	
	// Should be expired now
	_, found = cache.Get("http://example.com", "example.com")
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

func TestFilterCacheStats(t *testing.T) {
	cache := NewFilterCache(10, time.Minute)

	// Generate some hits and misses
	cache.Get("http://example.com/1", "example.com") // miss
	cache.Set("http://example.com/1", "example.com", true)
	cache.Get("http://example.com/1", "example.com") // hit
	cache.Get("http://example.com/2", "example.com") // miss

	stats := cache.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses, got %d", stats.Misses)
	}
	expectedHitRate := 33.3
	if stats.HitRate < expectedHitRate-0.1 || stats.HitRate > expectedHitRate+0.1 {
		t.Errorf("Expected hit rate around %.1f%%, got %.1f%%", expectedHitRate, stats.HitRate)
	}
}

func TestFilterCacheDisabled(t *testing.T) {
	cache := NewFilterCache(0, 0) // Disabled cache

	if cache.enabled {
		t.Error("Cache should be disabled")
	}

	// Operations should be no-ops
	cache.Set("http://example.com", "example.com", true)
	_, found := cache.Get("http://example.com", "example.com")
	if found {
		t.Error("Disabled cache should never return hits")
	}
}

func TestProxyFilterPipelineWithCache(t *testing.T) {
	config := &ProxyFilterConfig{
		Proxy:           "http://127.0.0.1:8080",
		ProxyFiltering:  true,
		ExtensionsMatch: []string{"php"},
		CacheSize:       10,
		CacheTTL:        time.Minute,
		Debug:           true,
	}

	extensionValidator := extensions.NewValidator(config.ExtensionsMatch, config.ExtensionFilter, false)
	scopeManager, _ := scope.NewManager(config.Scope, config.OutOfScope, "", false)
	
	pipeline := NewProxyFilterPipeline(config, extensionValidator, scopeManager)

	// First call should miss cache and evaluate filters
	result1 := pipeline.ShouldUseProxy("http://example.com/test.php", "example.com")
	if !result1 {
		t.Error("Expected PHP file to use proxy")
	}

	// Second call should hit cache
	result2 := pipeline.ShouldUseProxy("http://example.com/test.php", "example.com")
	if !result2 {
		t.Error("Expected cached result to be true")
	}

	// Check cache stats
	cacheStats := pipeline.GetCacheStats()
	if cacheStats == nil {
		t.Error("Expected cache stats to be available")
	} else {
		if cacheStats.Hits != 1 {
			t.Errorf("Expected 1 cache hit, got %d", cacheStats.Hits)
		}
	}

	// Check filter stats
	filterStats := pipeline.GetFilterStats()
	if filterStats == nil {
		t.Error("Expected filter stats to be available")
	} else {
		// Both calls should count as proxy requests (even the cached one)
		if filterStats.ProxyRequests != 2 {
			t.Errorf("Expected 2 proxy requests, got %d", filterStats.ProxyRequests)
		}
	}
}