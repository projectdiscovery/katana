package proxy

import (
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
)

// ProxyFilterPipeline manages filtering logic for proxy requests
type ProxyFilterPipeline struct {
	extensionValidator *extensions.Validator
	scopeManager       *scope.Manager
	matchRegex         []*regexp.Regexp
	filterRegex        []*regexp.Regexp
	matchCondition     string
	filterCondition    string
	enabled            bool
	proxy              string
	debug              bool
	extensionsMatch    []string
	extensionFilter    []string
	scopePatterns      []string
	outOfScopePatterns []string
	outputMatchRegex   []string
	outputFilterRegex  []string
	cache              *FilterCache
	stats              *FilterStats
}

// ProxyFilterConfig contains configuration for proxy filtering
type ProxyFilterConfig struct {
	Proxy                 string
	ProxyFiltering        bool
	Debug                 bool
	ExtensionsMatch       []string
	ExtensionFilter       []string
	Scope                 []string
	OutOfScope            []string
	OutputMatchRegex      []string
	OutputFilterRegex     []string
	OutputMatchCondition  string
	OutputFilterCondition string
	CacheSize             int
	CacheTTL              time.Duration
}

// NewProxyFilterPipeline creates a new proxy filter pipeline instance
func NewProxyFilterPipeline(config *ProxyFilterConfig, extensionValidator *extensions.Validator, scopeManager *scope.Manager) *ProxyFilterPipeline {
	pipeline := &ProxyFilterPipeline{
		extensionValidator:  extensionValidator,
		scopeManager:        scopeManager,
		matchCondition:      config.OutputMatchCondition,
		filterCondition:     config.OutputFilterCondition,
		proxy:               config.Proxy,
		debug:               config.Debug,
		extensionsMatch:     config.ExtensionsMatch,
		extensionFilter:     config.ExtensionFilter,
		scopePatterns:       config.Scope,
		outOfScopePatterns:  config.OutOfScope,
		outputMatchRegex:    config.OutputMatchRegex,
		outputFilterRegex:   config.OutputFilterRegex,
		enabled:             false,
	}

	// Compile regex patterns from config
	for _, pattern := range config.OutputMatchRegex {
		if compiled, err := regexp.Compile(pattern); err != nil {
			gologger.Warning().Msgf("Invalid match regex pattern '%s': %v", pattern, err)
		} else {
			pipeline.matchRegex = append(pipeline.matchRegex, compiled)
		}
	}

	for _, pattern := range config.OutputFilterRegex {
		if compiled, err := regexp.Compile(pattern); err != nil {
			gologger.Warning().Msgf("Invalid filter regex pattern '%s': %v", pattern, err)
		} else {
			pipeline.filterRegex = append(pipeline.filterRegex, compiled)
		}
	}

	// Enable filtering if proxy is configured and proxy filtering is enabled and any filters are specified
	pipeline.enabled = config.Proxy != "" && config.ProxyFiltering && pipeline.hasFilters()

	// Initialize caching if enabled
	if config.CacheSize > 0 && config.CacheTTL > 0 {
		pipeline.cache = NewFilterCache(config.CacheSize, config.CacheTTL)
		pipeline.cache.StartCleanupRoutine(config.CacheTTL / 2) // Cleanup every half TTL
		gologger.Debug().Msgf("Proxy filter cache enabled: size=%d, ttl=%v", config.CacheSize, config.CacheTTL)
	}

	// Initialize statistics tracking
	pipeline.stats = &FilterStats{}

	if pipeline.enabled {
		gologger.Debug().Msgf("Proxy filtering enabled with %d match regex, %d filter regex patterns", 
			len(pipeline.matchRegex), len(pipeline.filterRegex))
	}

	return pipeline
}

// hasFilters checks if any filtering options are configured
func (p *ProxyFilterPipeline) hasFilters() bool {
	return len(p.extensionsMatch) > 0 ||
		len(p.extensionFilter) > 0 ||
		len(p.scopePatterns) > 0 ||
		len(p.outOfScopePatterns) > 0 ||
		len(p.outputMatchRegex) > 0 ||
		len(p.outputFilterRegex) > 0 ||
		p.matchCondition != "" ||
		p.filterCondition != ""
}

// ShouldUseProxy determines if a URL should be sent through the proxy based on all configured filters
func (p *ProxyFilterPipeline) ShouldUseProxy(requestURL string, rootHostname string) bool {
	if !p.enabled {
		// If filtering is disabled, use proxy for all requests (existing behavior)
		return p.proxy != ""
	}

	// Validate input parameters
	if requestURL == "" {
		if p.debug {
			gologger.Warning().Msgf("Proxy filter: Empty URL provided, using direct connection")
		}
		return false
	}

	// Check cache first
	if p.cache != nil {
		if result, found := p.cache.Get(requestURL, rootHostname); found {
			if p.debug {
				gologger.Debug().Msgf("Proxy filter: Cache hit for URL '%s', result=%v", requestURL, result)
			}
			// Update statistics even for cache hits
			if p.stats != nil {
				if result {
					p.stats.IncrementProxyRequests()
				} else {
					p.stats.IncrementDirectRequests()
				}
			}
			return result
		}
	}

	// Apply filters in order of computational cost (fastest first)
	result := p.evaluateFilters(requestURL, rootHostname)

	// Cache the result
	if p.cache != nil {
		p.cache.Set(requestURL, rootHostname, result)
	}

	// Update statistics
	if p.stats != nil {
		if result {
			p.stats.IncrementProxyRequests()
		} else {
			p.stats.IncrementDirectRequests()
		}
	}

	return result
}

// evaluateFilters performs the actual filter evaluation
func (p *ProxyFilterPipeline) evaluateFilters(requestURL string, rootHostname string) bool {
	// 1. Extension filtering (fastest - string comparison)
	if !p.ValidateExtensions(requestURL) {
		if p.debug {
			gologger.Debug().Msgf("Proxy filter: URL '%s' filtered out by extension filter", requestURL)
		}
		if p.stats != nil {
			p.stats.IncrementExtensionFiltered()
		}
		return false
	}

	// 2. Scope filtering (fast - regex matching)
	if !p.ValidateScope(requestURL, rootHostname) {
		if p.debug {
			gologger.Debug().Msgf("Proxy filter: URL '%s' filtered out by scope filter", requestURL)
		}
		if p.stats != nil {
			p.stats.IncrementScopeFiltered()
		}
		return false
	}

	// 3. Regex filtering (moderate - pattern matching)
	if !p.ValidateRegex(requestURL) {
		if p.debug {
			gologger.Debug().Msgf("Proxy filter: URL '%s' filtered out by regex filter", requestURL)
		}
		if p.stats != nil {
			p.stats.IncrementRegexFiltered()
		}
		return false
	}

	// 4. Condition filtering (slowest - DSL evaluation)
	// Note: For condition filtering, we need a Result object, so we'll create a minimal one
	if !p.ValidateConditions(requestURL) {
		if p.debug {
			gologger.Debug().Msgf("Proxy filter: URL '%s' filtered out by condition filter", requestURL)
		}
		if p.stats != nil {
			p.stats.IncrementConditionFiltered()
		}
		return false
	}

	if p.debug {
		gologger.Debug().Msgf("Proxy filter: URL '%s' matches all filters, using proxy", requestURL)
	}
	return true
}

// ValidateExtensions validates URL against extension filters
func (p *ProxyFilterPipeline) ValidateExtensions(requestURL string) bool {
	if p.extensionValidator == nil {
		return true
	}
	return p.extensionValidator.ValidatePath(requestURL)
}

// ValidateScope validates URL against scope filters
func (p *ProxyFilterPipeline) ValidateScope(requestURL string, rootHostname string) bool {
	if p.scopeManager == nil {
		return true
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		gologger.Warning().Msgf("Proxy filter: Failed to parse URL '%s' for scope validation: %s", requestURL, err)
		// On parse error, default to not using proxy for safety
		return false
	}

	valid, err := p.scopeManager.Validate(parsedURL, rootHostname)
	if err != nil {
		gologger.Warning().Msgf("Proxy filter: Scope validation error for URL '%s': %s", requestURL, err)
		// On validation error, default to not using proxy for safety
		return false
	}
	
	if p.debug && !valid {
		gologger.Debug().Msgf("Proxy filter: URL '%s' is out of scope (root: %s)", requestURL, rootHostname)
	}
	
	return valid
}

// ValidateRegex validates URL against regex filters
func (p *ProxyFilterPipeline) ValidateRegex(requestURL string) bool {
	// Check filter regex first (exclusion patterns)
	for _, regex := range p.filterRegex {
		if regex.MatchString(requestURL) {
			return false
		}
	}

	// If no match regex patterns are specified, allow all URLs that passed filter regex
	if len(p.matchRegex) == 0 {
		return true
	}

	// Check match regex patterns (inclusion patterns)
	for _, regex := range p.matchRegex {
		if regex.MatchString(requestURL) {
			return true
		}
	}

	// If match patterns are specified but none matched, exclude the URL
	return false
}

// ValidateConditions validates URL against DSL condition filters
func (p *ProxyFilterPipeline) ValidateConditions(requestURL string) bool {
	// If no conditions are specified, allow all URLs
	if p.matchCondition == "" && p.filterCondition == "" {
		return true
	}

	// Create a minimal Result object for DSL evaluation
	// Note: This is a simplified approach - in a real scenario, we might need more context
	result := &output.Result{
		Request: &navigation.Request{
			URL: requestURL,
		},
	}

	// Check filter condition first (exclusion)
	if p.filterCondition != "" {
		if p.evalDslCondition(result, p.filterCondition) {
			return false
		}
	}

	// Check match condition (inclusion)
	if p.matchCondition != "" {
		return p.evalDslCondition(result, p.matchCondition)
	}

	return true
}

// evalDslCondition evaluates a DSL condition against a result
func (p *ProxyFilterPipeline) evalDslCondition(result *output.Result, condition string) bool {
	// This is a simplified implementation - the actual DSL evaluation logic
	// would need to be imported from the output package or refactored to be reusable
	
	// For now, we'll do basic string matching as a fallback
	// In a complete implementation, this would use the same DSL evaluation
	// logic as the output package
	
	// Basic URL-based condition matching
	if strings.Contains(condition, "url") {
		// Simple contains check for demonstration
		// Real implementation would parse and evaluate the DSL properly
		if strings.Contains(condition, "contains") {
			parts := strings.Split(condition, "'")
			if len(parts) >= 2 {
				searchTerm := parts[1]
				return strings.Contains(result.Request.URL, searchTerm)
			}
		}
	}

	// Default to true if we can't evaluate the condition
	gologger.Warning().Msgf("DSL condition evaluation not fully implemented for proxy filtering: %s", condition)
	return true
}

// IsEnabled returns whether proxy filtering is enabled
func (p *ProxyFilterPipeline) IsEnabled() bool {
	return p.enabled
}

// GetCacheStats returns cache statistics
func (p *ProxyFilterPipeline) GetCacheStats() *CacheStats {
	if p.cache == nil {
		return nil
	}
	stats := p.cache.GetStats()
	return &stats
}

// GetFilterStats returns filter statistics
func (p *ProxyFilterPipeline) GetFilterStats() *FilterStats {
	if p.stats == nil {
		return nil
	}
	stats := p.stats.GetFilterStats()
	return &stats
}

// LogPerformanceStats logs performance statistics for the filter pipeline
func (p *ProxyFilterPipeline) LogPerformanceStats() {
	if p.cache != nil {
		p.cache.LogCacheStats()
	}
	
	if p.stats != nil {
		stats := p.stats.GetFilterStats()
		total := stats.ProxyRequests + stats.DirectRequests
		if total > 0 {
			proxyPercent := float64(stats.ProxyRequests) / float64(total) * 100
			gologger.Info().Msgf("Proxy filter stats: %d total requests, %d (%.1f%%) via proxy, %d direct", 
				total, stats.ProxyRequests, proxyPercent, stats.DirectRequests)
			
			totalFiltered := stats.ExtensionFiltered + stats.ScopeFiltered + stats.RegexFiltered + stats.ConditionFiltered
			if totalFiltered > 0 {
				gologger.Info().Msgf("Filter breakdown: %d extension, %d scope, %d regex, %d condition", 
					stats.ExtensionFiltered, stats.ScopeFiltered, stats.RegexFiltered, stats.ConditionFiltered)
			}
		}
	}
}