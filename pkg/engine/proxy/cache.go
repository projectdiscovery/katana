package proxy

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"github.com/projectdiscovery/gologger"
)

// FilterCache provides caching for proxy filter decisions
type FilterCache struct {
	cache     map[string]*CacheEntry
	mutex     sync.RWMutex
	maxSize   int
	ttl       time.Duration
	hits      int64
	misses    int64
	enabled   bool
}

// CacheEntry represents a cached filter decision
type CacheEntry struct {
	Result    bool
	Timestamp time.Time
	URL       string
}

// NewFilterCache creates a new filter cache
func NewFilterCache(maxSize int, ttl time.Duration) *FilterCache {
	return &FilterCache{
		cache:   make(map[string]*CacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
		enabled: maxSize > 0 && ttl > 0,
	}
}

// Get retrieves a cached filter result
func (fc *FilterCache) Get(url, rootHostname string) (bool, bool) {
	if !fc.enabled {
		return false, false
	}

	fc.mutex.RLock()
	defer fc.mutex.RUnlock()

	key := fc.generateKey(url, rootHostname)
	entry, exists := fc.cache[key]
	
	if !exists {
		fc.misses++
		return false, false
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > fc.ttl {
		// Entry expired, will be cleaned up later
		fc.misses++
		return false, false
	}

	fc.hits++
	return entry.Result, true
}

// Set stores a filter result in the cache
func (fc *FilterCache) Set(url, rootHostname string, result bool) {
	if !fc.enabled {
		return
	}

	fc.mutex.Lock()
	defer fc.mutex.Unlock()

	key := fc.generateKey(url, rootHostname)
	
	// Check if we need to evict entries
	if len(fc.cache) >= fc.maxSize {
		fc.evictOldest()
	}

	fc.cache[key] = &CacheEntry{
		Result:    result,
		Timestamp: time.Now(),
		URL:       url,
	}
}

// generateKey creates a cache key from URL and root hostname
func (fc *FilterCache) generateKey(url, rootHostname string) string {
	data := fmt.Sprintf("%s|%s", url, rootHostname)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// evictOldest removes the oldest entries from the cache
func (fc *FilterCache) evictOldest() {
	if len(fc.cache) == 0 {
		return
	}

	// Find oldest entry
	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, entry := range fc.cache {
		if first || entry.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.Timestamp
			first = false
		}
	}

	if oldestKey != "" {
		delete(fc.cache, oldestKey)
	}
}

// Cleanup removes expired entries from the cache
func (fc *FilterCache) Cleanup() {
	if !fc.enabled {
		return
	}

	fc.mutex.Lock()
	defer fc.mutex.Unlock()

	now := time.Now()
	for key, entry := range fc.cache {
		if now.Sub(entry.Timestamp) > fc.ttl {
			delete(fc.cache, key)
		}
	}
}

// GetStats returns cache statistics
func (fc *FilterCache) GetStats() CacheStats {
	fc.mutex.RLock()
	defer fc.mutex.RUnlock()

	total := fc.hits + fc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(fc.hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:     fc.hits,
		Misses:   fc.misses,
		HitRate:  hitRate,
		Size:     len(fc.cache),
		MaxSize:  fc.maxSize,
		Enabled:  fc.enabled,
	}
}

// Clear empties the cache
func (fc *FilterCache) Clear() {
	fc.mutex.Lock()
	defer fc.mutex.Unlock()

	fc.cache = make(map[string]*CacheEntry)
	fc.hits = 0
	fc.misses = 0
}

// CacheStats represents cache performance statistics
type CacheStats struct {
	Hits     int64
	Misses   int64
	HitRate  float64
	Size     int
	MaxSize  int
	Enabled  bool
}

// LogCacheStats logs cache performance statistics
func (fc *FilterCache) LogCacheStats() {
	if !fc.enabled {
		return
	}

	stats := fc.GetStats()
	gologger.Info().Msgf("Proxy filter cache stats: %d hits, %d misses, %.1f%% hit rate, %d/%d entries", 
		stats.Hits, stats.Misses, stats.HitRate, stats.Size, stats.MaxSize)
}

// StartCleanupRoutine starts a background routine to clean up expired cache entries
func (fc *FilterCache) StartCleanupRoutine(interval time.Duration) {
	if !fc.enabled {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			fc.Cleanup()
		}
	}()
}