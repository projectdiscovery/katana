package proxy

import (
	"sync/atomic"
	"time"
)

// FilterResult represents the result of a proxy filter decision
type FilterResult struct {
	ShouldUseProxy bool
	FilterReason   string
	AppliedFilters []string
	URL            string
	Timestamp      time.Time
}

// FilterStats tracks statistics for proxy filtering operations
type FilterStats struct {
	ExtensionFiltered int64
	ScopeFiltered     int64
	RegexFiltered     int64
	ConditionFiltered int64
	ProxyRequests     int64
	DirectRequests    int64
}

// IncrementExtensionFiltered atomically increments the extension filtered counter
func (fs *FilterStats) IncrementExtensionFiltered() {
	atomic.AddInt64(&fs.ExtensionFiltered, 1)
}

// IncrementScopeFiltered atomically increments the scope filtered counter
func (fs *FilterStats) IncrementScopeFiltered() {
	atomic.AddInt64(&fs.ScopeFiltered, 1)
}

// IncrementRegexFiltered atomically increments the regex filtered counter
func (fs *FilterStats) IncrementRegexFiltered() {
	atomic.AddInt64(&fs.RegexFiltered, 1)
}

// IncrementConditionFiltered atomically increments the condition filtered counter
func (fs *FilterStats) IncrementConditionFiltered() {
	atomic.AddInt64(&fs.ConditionFiltered, 1)
}

// IncrementProxyRequests atomically increments the proxy request counter
func (fs *FilterStats) IncrementProxyRequests() {
	atomic.AddInt64(&fs.ProxyRequests, 1)
}

// IncrementDirectRequests atomically increments the direct request counter
func (fs *FilterStats) IncrementDirectRequests() {
	atomic.AddInt64(&fs.DirectRequests, 1)
}

// GetFilterStats returns a copy of the current filter statistics
func (fs *FilterStats) GetFilterStats() FilterStats {
	return FilterStats{
		ExtensionFiltered: atomic.LoadInt64(&fs.ExtensionFiltered),
		ScopeFiltered:     atomic.LoadInt64(&fs.ScopeFiltered),
		RegexFiltered:     atomic.LoadInt64(&fs.RegexFiltered),
		ConditionFiltered: atomic.LoadInt64(&fs.ConditionFiltered),
		ProxyRequests:     atomic.LoadInt64(&fs.ProxyRequests),
		DirectRequests:    atomic.LoadInt64(&fs.DirectRequests),
	}
}

// ProxyStats tracks proxy vs direct request statistics
type ProxyStats struct {
	TotalRequests  int64
	ProxyRequests  int64
	DirectRequests int64
	FilteredOut    int64
}

// IncrementProxyRequests atomically increments the proxy request counter
func (ps *ProxyStats) IncrementProxyRequests() {
	atomic.AddInt64(&ps.ProxyRequests, 1)
	atomic.AddInt64(&ps.TotalRequests, 1)
}

// IncrementDirectRequests atomically increments the direct request counter
func (ps *ProxyStats) IncrementDirectRequests() {
	atomic.AddInt64(&ps.DirectRequests, 1)
	atomic.AddInt64(&ps.TotalRequests, 1)
}

// IncrementFilteredOut atomically increments the filtered out counter
func (ps *ProxyStats) IncrementFilteredOut() {
	atomic.AddInt64(&ps.FilteredOut, 1)
}

// GetStats returns a copy of the current statistics
func (ps *ProxyStats) GetStats() ProxyStats {
	return ProxyStats{
		TotalRequests:  atomic.LoadInt64(&ps.TotalRequests),
		ProxyRequests:  atomic.LoadInt64(&ps.ProxyRequests),
		DirectRequests: atomic.LoadInt64(&ps.DirectRequests),
		FilteredOut:    atomic.LoadInt64(&ps.FilteredOut),
	}
}

// ProxyFilterSettings holds configuration for proxy filtering behavior
type ProxyFilterSettings struct {
	Enabled             bool
	ExtensionFiltering  bool
	ScopeFiltering      bool
	RegexFiltering      bool
	ConditionFiltering  bool
	StatsEnabled        bool
}

// NewProxyFilterSettings creates a new proxy filter configuration from options
func NewProxyFilterSettings(hasExtensionFilters, hasScopeFilters, hasRegexFilters, hasConditionFilters bool) *ProxyFilterSettings {
	return &ProxyFilterSettings{
		Enabled:             hasExtensionFilters || hasScopeFilters || hasRegexFilters || hasConditionFilters,
		ExtensionFiltering:  hasExtensionFilters,
		ScopeFiltering:      hasScopeFilters,
		RegexFiltering:      hasRegexFilters,
		ConditionFiltering:  hasConditionFilters,
		StatsEnabled:        true, // Enable stats by default
	}
}