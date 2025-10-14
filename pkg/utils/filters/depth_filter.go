package filters

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DepthFilter represents a single depth filtering condition
type DepthFilter struct {
	Operator string // "==", ">=", "<=", ">", "<", "range"
	Value    int    // The count value to compare against
	MaxValue int    // For range operations (Value-MaxValue)
}

// URLComponents represents cached URL components for performance
type URLComponents struct {
	PathDepth      int
	QueryParams    int
	SubdomainDepth int
	CachedAt       time.Time
}

// DepthFilterCache manages caching for URL components and filter results
type DepthFilterCache struct {
	urlComponents map[string]*URLComponents
	filterResults map[string]bool
	mutex         sync.RWMutex
	maxSize       int
	ttl           time.Duration
}

// DepthFilterValidator manages depth filtering for URLs
type DepthFilterValidator struct {
	pathFilters      []DepthFilter
	queryFilters     []DepthFilter
	subdomainFilters []DepthFilter
	useOrLogic       bool // If true, use OR logic between filter types; if false, use AND logic
	cache            *DepthFilterCache
}

// NewDepthFilterValidator creates a new depth filter validator
func NewDepthFilterValidator(pathFilters, queryFilters, subdomainFilters []string, useOrLogic bool) (*DepthFilterValidator, error) {
	validator := &DepthFilterValidator{
		useOrLogic: useOrLogic,
		cache:      NewDepthFilterCache(1000, 5*time.Minute), // Cache up to 1000 entries for 5 minutes
	}

	// Parse path depth filters
	for i, filter := range pathFilters {
		if filter == "" {
			continue
		}
		parsed, err := parseDepthFilter(filter)
		if err != nil {
			return nil, fmt.Errorf("❌ Path depth filter #%d error:\n%w\n\n🔧 Fix: Update your -cpd flag:\n   katana -cpd \"[corrected_filter]\"", i+1, err)
		}
		validator.pathFilters = append(validator.pathFilters, parsed)
	}

	// Parse query parameter filters
	for i, filter := range queryFilters {
		if filter == "" {
			continue
		}
		parsed, err := parseDepthFilter(filter)
		if err != nil {
			return nil, fmt.Errorf("❌ Query parameter filter #%d error:\n%w\n\n🔧 Fix: Update your -cqp flag:\n   katana -cqp \"[corrected_filter]\"", i+1, err)
		}
		validator.queryFilters = append(validator.queryFilters, parsed)
	}

	// Parse subdomain depth filters
	for i, filter := range subdomainFilters {
		if filter == "" {
			continue
		}
		parsed, err := parseDepthFilter(filter)
		if err != nil {
			return nil, fmt.Errorf("❌ Subdomain depth filter #%d error:\n%w\n\n🔧 Fix: Update your -csd flag:\n   katana -csd \"[corrected_filter]\"", i+1, err)
		}
		validator.subdomainFilters = append(validator.subdomainFilters, parsed)
	}

	return validator, nil
}

// ValidateURL validates a URL against all configured depth filters with caching
func (d *DepthFilterValidator) ValidateURL(parsedURL *url.URL) bool {
	if parsedURL == nil {
		return false
	}

	// Early exit: If no filters are configured, allow all URLs
	if len(d.pathFilters) == 0 && len(d.queryFilters) == 0 && len(d.subdomainFilters) == 0 {
		return true
	}

	urlStr := parsedURL.String()
	
	// Get or compute URL components (cached)
	components := d.cache.GetURLComponents(urlStr, parsedURL)
	
	// Check cache for filter result
	cacheKey := d.generateCacheKey(components)
	if result, exists := d.cache.GetFilterResult(cacheKey); exists {
		return result
	}

	// Compute filter result
	var result bool
	if d.useOrLogic {
		result = d.evaluateOrLogic(components)
	} else {
		result = d.evaluateAndLogic(components)
	}

	// Cache the result
	d.cache.SetFilterResult(cacheKey, result)
	
	return result
}

// evaluateOrLogic implements OR logic between filter types using cached components
func (d *DepthFilterValidator) evaluateOrLogic(components *URLComponents) bool {
	// At least one configured filter type must pass
	hasConfiguredFilters := len(d.pathFilters) > 0 || len(d.queryFilters) > 0 || len(d.subdomainFilters) > 0
	if !hasConfiguredFilters {
		return true
	}

	// Check each filter type - return true if any configured type passes
	if len(d.pathFilters) > 0 && d.validatePathDepthWithComponents(components.PathDepth) {
		return true
	}
	
	if len(d.queryFilters) > 0 && d.validateQueryParamsWithComponents(components.QueryParams) {
		return true
	}
	
	if len(d.subdomainFilters) > 0 && d.validateSubdomainDepthWithComponents(components.SubdomainDepth) {
		return true
	}

	return false
}

// evaluateAndLogic implements AND logic between filter types using cached components
func (d *DepthFilterValidator) evaluateAndLogic(components *URLComponents) bool {
	// All configured filter types must pass
	if len(d.pathFilters) > 0 && !d.validatePathDepthWithComponents(components.PathDepth) {
		return false
	}

	if len(d.queryFilters) > 0 && !d.validateQueryParamsWithComponents(components.QueryParams) {
		return false
	}

	if len(d.subdomainFilters) > 0 && !d.validateSubdomainDepthWithComponents(components.SubdomainDepth) {
		return false
	}

	return true
}

// parseDepthFilter parses a filter expression like ">=3", "==2", "<=4", "3-5"
func parseDepthFilter(filter string) (DepthFilter, error) {
	originalFilter := filter
	filter = strings.TrimSpace(filter)
	
	if filter == "" {
		return DepthFilter{}, fmt.Errorf("empty filter expression\n" +
			"💡 Tip: Use comparison operators like '>=3', '==2', '<=4' or ranges like '2-5'\n" +
			"📖 Examples:\n" +
			"   -cpd \">=3\"    # Path depth 3 or more\n" +
			"   -cqp \"==2\"    # Exactly 2 query parameters\n" +
			"   -csd \"1-3\"    # Subdomain levels between 1 and 3")
	}

	// Check for range syntax first (e.g., "3-5", "1-10")
	rangeRe := regexp.MustCompile(`^(\d+)-(\d+)$`)
	if rangeMatches := rangeRe.FindStringSubmatch(filter); len(rangeMatches) == 3 {
		minValue, err1 := strconv.Atoi(rangeMatches[1])
		maxValue, err2 := strconv.Atoi(rangeMatches[2])
		
		if err1 != nil || err2 != nil {
			return DepthFilter{}, fmt.Errorf("invalid range values in '%s': numbers must be valid integers\n" +
				"💡 Tip: Use only non-negative integers in ranges\n" +
				"📖 Examples: '1-5', '0-3', '2-10'", filter)
		}
		
		if minValue < 0 || maxValue < 0 {
			return DepthFilter{}, fmt.Errorf("invalid range '%s': negative values not allowed (min=%d, max=%d)\n" +
				"💡 Tip: Use non-negative integers only\n" +
				"📖 Examples: '0-2' (0 to 2), '1-5' (1 to 5)", filter, minValue, maxValue)
		}
		
		if minValue > maxValue {
			return DepthFilter{}, fmt.Errorf("invalid range '%s': minimum value (%d) cannot be greater than maximum value (%d)\n" +
				"💡 Tip: Ensure the first number is smaller than or equal to the second\n" +
				"❌ Wrong: '%s'\n" +
				"✅ Correct: '%d-%d'", filter, minValue, maxValue, filter, maxValue, minValue)
		}
		
		return DepthFilter{
			Operator: "range",
			Value:    minValue,
			MaxValue: maxValue,
		}, nil
	}

	// Regular expression to match operator and value
	re := regexp.MustCompile(`^(>=|<=|==|>|<)(\d+)$`)
	matches := re.FindStringSubmatch(filter)
	
	if len(matches) != 3 {
		// Provide specific guidance based on common mistakes
		errorMsg := fmt.Sprintf("malformed filter expression '%s'\n", originalFilter)
		
		// Check for common mistakes and provide specific guidance
		if strings.Contains(filter, " ") {
			errorMsg += "💡 Tip: Remove spaces from the filter expression\n" +
				fmt.Sprintf("❌ Wrong: '%s'\n", originalFilter) +
				fmt.Sprintf("✅ Correct: '%s'\n", strings.ReplaceAll(filter, " ", ""))
		} else if regexp.MustCompile(`^\d+$`).MatchString(filter) {
			errorMsg += "💡 Tip: Add a comparison operator before the number\n" +
				fmt.Sprintf("❌ Wrong: '%s'\n", filter) +
				fmt.Sprintf("✅ Correct: '>=%s', '==%s', or '<=%s'\n", filter, filter, filter)
		} else if regexp.MustCompile(`^[><=!]+$`).MatchString(filter) {
			errorMsg += "💡 Tip: Add a number after the operator\n" +
				fmt.Sprintf("❌ Wrong: '%s'\n", filter) +
				fmt.Sprintf("✅ Correct: '%s3', '%s2', or '%s5'\n", filter, filter, filter)
		} else if strings.Contains(filter, "!=") || strings.Contains(filter, "<>") {
			errorMsg += "💡 Tip: Use '==' for equality, not '!=' or '<>'\n" +
				"❌ Wrong: '!=3', '<>2'\n" +
				"✅ Correct: '==3', '==2'\n" +
				"ℹ️  Note: Use ranges for 'not equal' logic, e.g., '0-2' and '4-10' instead of '!=3'"
		} else {
			errorMsg += "💡 Tip: Use valid comparison operators\n"
		}
		
		errorMsg += "\n📖 Valid formats:\n" +
			"   Comparisons: '>=3', '<=5', '==2', '>1', '<4'\n" +
			"   Ranges: '2-5', '0-3', '1-10'\n" +
			"\n🎯 Common use cases:\n" +
			"   -cpd \">=2\"    # URLs with path depth 2 or more\n" +
			"   -cqp \"1-3\"    # URLs with 1 to 3 query parameters\n" +
			"   -csd \"==0\"    # URLs with no subdomains"
		
		return DepthFilter{}, fmt.Errorf("%s", errorMsg)
	}

	operator := matches[1]
	valueStr := matches[2]

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return DepthFilter{}, fmt.Errorf("invalid value '%s' in filter '%s': must be a valid integer\n" +
			"💡 Tip: Use only numeric values\n" +
			"❌ Wrong: '%s'\n" +
			"✅ Correct: '%s123', '%s0', '%s5'", valueStr, originalFilter, operator, operator, operator, operator)
	}

	if value < 0 {
		return DepthFilter{}, fmt.Errorf("invalid value '%d' in filter '%s': negative values not allowed\n" +
			"💡 Tip: Use non-negative integers (0, 1, 2, 3, ...)\n" +
			"❌ Wrong: '%s'\n" +
			"✅ Correct: '%s%d'", value, originalFilter, originalFilter, operator, -value)
	}

	return DepthFilter{
		Operator: operator,
		Value:    value,
		MaxValue: 0, // Not used for non-range operators
	}, nil
}

// evaluateCondition evaluates a condition (actual operator expected)
func evaluateCondition(actual int, filter DepthFilter) bool {
	switch filter.Operator {
	case "==":
		return actual == filter.Value
	case ">=":
		return actual >= filter.Value
	case "<=":
		return actual <= filter.Value
	case ">":
		return actual > filter.Value
	case "<":
		return actual < filter.Value
	case "range":
		return actual >= filter.Value && actual <= filter.MaxValue
	default:
		return false
	}
}

// countPathSegments counts the number of path segments in a URL path
func countPathSegments(path string) int {
	if path == "" || path == "/" {
		return 0
	}

	// Remove leading and trailing slashes
	path = strings.Trim(path, "/")
	if path == "" {
		return 0
	}

	// Split by slash and count non-empty segments
	segments := strings.Split(path, "/")
	count := 0
	for _, segment := range segments {
		if segment != "" {
			count++
		}
	}

	return count
}

// countQueryParams counts the number of query parameters in a query string
func countQueryParams(query string) int {
	if query == "" {
		return 0
	}

	// Split by & and count valid parameters
	params := strings.Split(query, "&")
	count := 0
	for _, param := range params {
		param = strings.TrimSpace(param)
		// Count parameters that have a key (with or without value)
		if param != "" && !strings.HasPrefix(param, "=") {
			count++
		}
	}

	return count
}

// countSubdomainLevels counts the number of subdomain levels in a hostname
func countSubdomainLevels(hostname string) int {
	if hostname == "" {
		return 0
	}

	// Remove any port number
	if colonIndex := strings.LastIndex(hostname, ":"); colonIndex != -1 {
		hostname = hostname[:colonIndex]
	}

	// Split by dots
	parts := strings.Split(hostname, ".")
	if len(parts) <= 2 {
		// No subdomains (e.g., "example.com" or "localhost")
		return 0
	}

	// Count subdomain levels (total parts - 2 for domain.tld)
	return len(parts) - 2
}

// NewDepthFilterCache creates a new cache for URL components and filter results
func NewDepthFilterCache(maxSize int, ttl time.Duration) *DepthFilterCache {
	return &DepthFilterCache{
		urlComponents: make(map[string]*URLComponents),
		filterResults: make(map[string]bool),
		maxSize:       maxSize,
		ttl:           ttl,
	}
}

// GetURLComponents retrieves cached URL components or computes and caches them
func (c *DepthFilterCache) GetURLComponents(urlStr string, parsedURL *url.URL) *URLComponents {
	c.mutex.RLock()
	if components, exists := c.urlComponents[urlStr]; exists {
		// Check if cache entry is still valid
		if time.Since(components.CachedAt) < c.ttl {
			c.mutex.RUnlock()
			return components
		}
	}
	c.mutex.RUnlock()

	// Compute components
	components := &URLComponents{
		PathDepth:      countPathSegments(parsedURL.Path),
		QueryParams:    countQueryParams(parsedURL.RawQuery),
		SubdomainDepth: countSubdomainLevels(parsedURL.Hostname()),
		CachedAt:       time.Now(),
	}

	// Cache the result
	c.mutex.Lock()
	// Implement simple LRU by clearing cache when it gets too large
	if len(c.urlComponents) >= c.maxSize {
		// Clear oldest entries (simple approach - clear all)
		c.urlComponents = make(map[string]*URLComponents)
		c.filterResults = make(map[string]bool)
	}
	c.urlComponents[urlStr] = components
	c.mutex.Unlock()

	return components
}

// GetFilterResult retrieves cached filter result or returns false if not found
func (c *DepthFilterCache) GetFilterResult(key string) (bool, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	result, exists := c.filterResults[key]
	return result, exists
}

// SetFilterResult caches a filter result
func (c *DepthFilterCache) SetFilterResult(key string, result bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.filterResults[key] = result
}

// generateCacheKey creates a cache key for filter results
func (d *DepthFilterValidator) generateCacheKey(components *URLComponents) string {
	return fmt.Sprintf("p%d:q%d:s%d:or%t", 
		components.PathDepth, 
		components.QueryParams, 
		components.SubdomainDepth, 
		d.useOrLogic)
}

// validatePathDepthWithComponents checks path depth using cached components
func (d *DepthFilterValidator) validatePathDepthWithComponents(depth int) bool {
	for _, filter := range d.pathFilters {
		if !evaluateCondition(depth, filter) {
			return false
		}
	}
	return true
}

// validateQueryParamsWithComponents checks query params using cached components
func (d *DepthFilterValidator) validateQueryParamsWithComponents(count int) bool {
	for _, filter := range d.queryFilters {
		if !evaluateCondition(count, filter) {
			return false
		}
	}
	return true
}

// validateSubdomainDepthWithComponents checks subdomain depth using cached components
func (d *DepthFilterValidator) validateSubdomainDepthWithComponents(depth int) bool {
	for _, filter := range d.subdomainFilters {
		if !evaluateCondition(depth, filter) {
			return false
		}
	}
	return true
}

// ValidateAndSuggest validates a filter and provides suggestions if invalid
func ValidateAndSuggest(filterType, filter string) error {
	_, err := parseDepthFilter(filter)
	if err != nil {
		return fmt.Errorf("❌ %s filter validation failed:\n%w", 
			strings.Title(filterType), err)
	}
	return nil
}