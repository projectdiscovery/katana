package proxy

import (
	"fmt"
	"net/url"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/utils/errkit"
)

// ProxyError represents a proxy-related error with additional context
type ProxyError struct {
	Operation string
	URL       string
	Proxy     string
	Cause     error
}

func (e *ProxyError) Error() string {
	if e.URL != "" && e.Proxy != "" {
		return fmt.Sprintf("proxy %s failed for URL %s via proxy %s: %v", e.Operation, e.URL, e.Proxy, e.Cause)
	} else if e.URL != "" {
		return fmt.Sprintf("proxy %s failed for URL %s: %v", e.Operation, e.URL, e.Cause)
	}
	return fmt.Sprintf("proxy %s failed: %v", e.Operation, e.Cause)
}

func (e *ProxyError) Unwrap() error {
	return e.Cause
}

// NewProxyError creates a new proxy error with context
func NewProxyError(operation, url, proxy string, cause error) *ProxyError {
	return &ProxyError{
		Operation: operation,
		URL:       url,
		Proxy:     proxy,
		Cause:     cause,
	}
}

// ValidateProxyURL validates a proxy URL and returns a detailed error if invalid
func ValidateProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return errkit.New("proxy URL cannot be empty")
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return errkit.Wrap(err, "invalid proxy URL format")
	}

	// Check scheme
	validSchemes := map[string]bool{
		"http":   true,
		"https":  true,
		"socks5": true,
	}
	
	if !validSchemes[parsed.Scheme] {
		return errkit.New("proxy URL must use http, https, or socks5 scheme, got: " + parsed.Scheme)
	}

	// Check host
	if parsed.Host == "" {
		return errkit.New("proxy URL must include a host")
	}

	// Validate port if specified
	if parsed.Port() != "" {
		// Port validation is handled by url.Parse, but we can add additional checks
		gologger.Debug().Msgf("Proxy URL validation: Using port %s", parsed.Port())
	}

	return nil
}

// LogProxyError logs a proxy error with appropriate severity
func LogProxyError(err error, fallbackAction string) {
	if proxyErr, ok := err.(*ProxyError); ok {
		gologger.Error().Msgf("Proxy Error - %s", proxyErr.Error())
		if fallbackAction != "" {
			gologger.Warning().Msgf("Fallback action: %s", fallbackAction)
		}
	} else {
		gologger.Error().Msgf("Proxy operation failed: %v", err)
		if fallbackAction != "" {
			gologger.Warning().Msgf("Fallback action: %s", fallbackAction)
		}
	}
}

// RecoverFromPanic recovers from panics in proxy operations and logs them
func RecoverFromPanic(operation, url string) {
	if r := recover(); r != nil {
		gologger.Error().Msgf("Panic recovered in proxy %s for URL %s: %v", operation, url, r)
		gologger.Warning().Msgf("Continuing with fallback behavior after panic recovery")
	}
}

// ProxyHealthCheck performs basic health checks on proxy configuration
func ProxyHealthCheck(config *ProxyConfig) []string {
	var issues []string

	if config == nil {
		issues = append(issues, "ProxyConfig is nil")
		return issues
	}

	if config.DirectClient == nil {
		issues = append(issues, "Direct HTTP client is nil - this will cause failures")
	}

	if config.ProxyURL != nil && config.ProxyClient == nil {
		issues = append(issues, "Proxy URL is configured but proxy client is nil")
	}

	if config.FilterPipeline == nil {
		issues = append(issues, "Filter pipeline is nil - proxy filtering will not work")
	}

	if config.Stats == nil {
		issues = append(issues, "Statistics tracking is nil")
	}

	return issues
}

// LogProxyHealthCheck logs the results of a proxy health check
func LogProxyHealthCheck(config *ProxyConfig) {
	issues := ProxyHealthCheck(config)
	
	if len(issues) == 0 {
		gologger.Debug().Msgf("Proxy configuration health check: All checks passed")
		return
	}

	gologger.Warning().Msgf("Proxy configuration health check found %d issues:", len(issues))
	for i, issue := range issues {
		gologger.Warning().Msgf("  %d. %s", i+1, issue)
	}
}