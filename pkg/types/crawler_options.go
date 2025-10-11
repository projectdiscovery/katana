package types

import (
	"context"
	"regexp"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/proxy"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/ratelimit"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/utils/errkit"
	urlutil "github.com/projectdiscovery/utils/url"
	wappalyzer "github.com/projectdiscovery/wappalyzergo"
)

// CrawlerOptions contains helper utilities for the crawler
type CrawlerOptions struct {
	// OutputWriter is the interface for writing output
	OutputWriter output.Writer
	// RateLimit is a mechanism for controlling request rate limit
	RateLimit *ratelimit.Limiter
	// Parser is a mechanism for extracting new URLS from responses
	Parser *parser.Parser
	// Options contains the user specified configuration options
	Options *Options
	// ExtensionsValidator is a validator for file extensions
	ExtensionsValidator *extensions.Validator
	// UniqueFilter is a filter for deduplication of unique items
	UniqueFilter filters.Filter
	// ScopeManager is a manager for validating crawling scope
	ScopeManager *scope.Manager
	// Dialer is instance of the dialer for global crawler
	Dialer *fastdialer.Dialer
	// Wappalyzer instance for technologies detection
	Wappalyzer *wappalyzer.Wappalyze
	// ProxyFilterPipeline manages proxy filtering logic
	ProxyFilterPipeline *proxy.ProxyFilterPipeline
	// ProxyConfig manages HTTP clients with proxy support
	ProxyConfig *proxy.ProxyConfig
}

// NewCrawlerOptions creates a new crawler options structure
// from user specified options.
func NewCrawlerOptions(options *Options) (*CrawlerOptions, error) {
	options.ConfigureOutput()
	extensionsValidator := extensions.NewValidator(options.ExtensionsMatch, options.ExtensionFilter, options.NoDefaultExtFilter)

	parserOptions := &parser.Options{
		AutomaticFormFill:      options.AutomaticFormFill,
		ScrapeJSLuiceResponses: options.ScrapeJSLuiceResponses,
		ScrapeJSResponses:      options.ScrapeJSResponses,
		DisableRedirects:       options.DisableRedirects,
	}

	responseParser := parser.NewResponseParser()
	responseParser.InitWithOptions(parserOptions)

	dialerOpts := fastdialer.DefaultOptions
	if len(options.Resolvers) > 0 {
		dialerOpts.BaseResolvers = options.Resolvers
	}

	fastdialerInstance, err := fastdialer.NewDialer(dialerOpts)
	if err != nil {
		return nil, err
	}
	scopeManager, err := scope.NewManager(options.Scope, options.OutOfScope, options.FieldScope, options.NoScope)
	if err != nil {
		return nil, errkit.Wrap(err, "could not create scope manager")
	}
	itemFilter, err := filters.NewSimple()
	if err != nil {
		return nil, errkit.Wrap(err, "could not create filter")
	}

	outputOptions := output.Options{
		Colors:                !options.NoColors,
		JSON:                  options.JSON,
		Verbose:               options.Verbose,
		StoreResponse:         options.StoreResponse,
		OutputFile:            options.OutputFile,
		Fields:                options.Fields,
		StoreFields:           options.StoreFields,
		StoreResponseDir:      options.StoreResponseDir,
		NoClobber:             options.NoClobber,
		StoreFieldDir:         options.StoreFieldDir,
		OmitRaw:               options.OmitRaw,
		OmitBody:              options.OmitBody,
		FieldConfig:           options.FieldConfig,
		ErrorLogFile:          options.ErrorLogFile,
		MatchRegex:            options.MatchRegex,
		FilterRegex:           options.FilterRegex,
		ExtensionValidator:    extensionsValidator,
		OutputTemplate:        options.OutputTemplate,
		OutputMatchCondition:  options.OutputMatchCondition,
		OutputFilterCondition: options.OutputFilterCondition,
		ExcludeOutputFields:   options.ExcludeOutputFields,
	}

	for _, mr := range options.OutputMatchRegex {
		cr, err := regexp.Compile(mr)
		if err != nil {
			return nil, errkit.Wrap(err, "Invalid value for match regex option")
		}
		outputOptions.MatchRegex = append(outputOptions.MatchRegex, cr)
	}
	for _, fr := range options.OutputFilterRegex {
		cr, err := regexp.Compile(fr)
		if err != nil {
			return nil, errkit.Wrap(err, "Invalid value for filter regex option")
		}
		outputOptions.FilterRegex = append(outputOptions.FilterRegex, cr)
	}

	outputWriter, err := output.New(outputOptions)
	if err != nil {
		return nil, errkit.Wrap(err, "could not create output writer")
	}

	// Initialize proxy filter pipeline
	proxyFilterConfig := &proxy.ProxyFilterConfig{
		Proxy:                 options.Proxy,
		ProxyFiltering:        options.ProxyFiltering,
		Debug:                 options.Debug,
		ExtensionsMatch:       options.ExtensionsMatch,
		ExtensionFilter:       options.ExtensionFilter,
		Scope:                 options.Scope,
		OutOfScope:            options.OutOfScope,
		OutputMatchRegex:      options.OutputMatchRegex,
		OutputFilterRegex:     options.OutputFilterRegex,
		OutputMatchCondition:  options.OutputMatchCondition,
		OutputFilterCondition: options.OutputFilterCondition,
		CacheSize:             1000,        // Cache up to 1000 filter results
		CacheTTL:              time.Minute, // Cache results for 1 minute
	}
	proxyFilterPipeline := proxy.NewProxyFilterPipeline(proxyFilterConfig, extensionsValidator, scopeManager)
	
	// Validate proxy URL if provided
	if options.Proxy != "" {
		if err := proxy.ValidateProxyURL(options.Proxy); err != nil {
			return nil, errkit.Wrap(err, "invalid proxy configuration")
		}
	}

	// Initialize proxy configuration with enhanced HTTP client management
	httpClientConfig := &proxy.HttpClientConfig{
		Proxy:            options.Proxy,
		Timeout:          options.Timeout,
		Retries:          options.Retries,
		TlsImpersonate:   options.TlsImpersonate,
		DisableRedirects: options.DisableRedirects,
	}
	proxyConfig, err := proxy.BuildHttpClientWithProxyFilter(fastdialerInstance, httpClientConfig, proxyFilterPipeline, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "could not create proxy configuration")
	}

	// Perform health check on proxy configuration
	proxy.LogProxyHealthCheck(proxyConfig)

	crawlerOptions := &CrawlerOptions{
		ExtensionsValidator: extensionsValidator,
		Parser:              responseParser,
		ScopeManager:        scopeManager,
		UniqueFilter:        itemFilter,
		Options:             options,
		Dialer:              fastdialerInstance,
		OutputWriter:        outputWriter,
		ProxyFilterPipeline: proxyFilterPipeline,
		ProxyConfig:         proxyConfig,
	}

	if options.RateLimit > 0 {
		crawlerOptions.RateLimit = ratelimit.New(context.Background(), uint(options.RateLimit), time.Second)
	} else if options.RateLimitMinute > 0 {
		crawlerOptions.RateLimit = ratelimit.New(context.Background(), uint(options.RateLimitMinute), time.Minute)
	}

	if options.TechDetect {
		wappalyze, err := wappalyzer.New()
		if err != nil {
			return nil, err
		}
		crawlerOptions.Wappalyzer = wappalyze
	}

	return crawlerOptions, nil
}

// Close closes the crawler options resources
func (c *CrawlerOptions) Close() error {
	c.UniqueFilter.Close()
	return c.OutputWriter.Close()
}

func (c *CrawlerOptions) ValidatePath(path string) bool {
	if c.ExtensionsValidator != nil {
		return c.ExtensionsValidator.ValidatePath(path)
	}
	return true
}

// ValidateScope validates scope for an AbsURL
func (c *CrawlerOptions) ValidateScope(absURL, rootHostname string) (bool, error) {
	parsed, err := urlutil.Parse(absURL)
	if err != nil {
		return false, err
	}
	if c.ScopeManager != nil {
		return c.ScopeManager.Validate(parsed.URL, rootHostname)
	}
	return true, nil
}

// GetHttpClient returns the appropriate HTTP client based on proxy filtering
func (c *CrawlerOptions) GetHttpClient(requestURL, rootHostname string) *retryablehttp.Client {
	if c.ProxyConfig != nil {
		return c.ProxyConfig.GetClient(requestURL, rootHostname)
	}
	// Return nil if no proxy config is available - the caller should handle this
	return nil
}
