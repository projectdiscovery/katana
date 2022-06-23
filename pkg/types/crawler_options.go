package types

import (
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"go.uber.org/ratelimit"
)

// CrawlerOptions contains helper utilities for the crawler
type CrawlerOptions struct {
	// OutputWriter is the interface for writing output
	OutputWriter output.Writer
	// RateLimit is a mechanism for controlling request rate limit
	RateLimit ratelimit.Limiter
	// Options contains the user specified configuration options
	Options *Options
	// ExtensionsValidator is a validator for file extensions
	ExtensionsValidator *extensions.Validator
	// UniqueFilter is a filter for deduplication of unique items
	UniqueFilter filters.Filter
	// ScopeManager is a manager for validating crawling scope
	ScopeManager *scope.Manager
}

// NewCrawlerOptions creates a new crawler options structure
// from user specified options.
func NewCrawlerOptions(options *Options) (*CrawlerOptions, error) {
	extensionsValidator := extensions.NewValidator(options.Extensions, options.ExtensionsAllowList, options.ExtensionDenyList)

	scopeManager, err := scope.NewManager(options.Scope, options.OutOfScope, options.ScopeDomains, options.OutOfScopeDomains, options.IncludeSubdomains)
	if err != nil {
		return nil, errors.Wrap(err, "could not create scope manager")
	}
	itemFilter := filters.NewSimple()

	outputWriter, err := output.New(!options.NoColors, options.JSON, options.Verbose, options.OutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not create output writer")
	}

	var ratelimiter ratelimit.Limiter
	if options.RateLimit > 0 {
		ratelimiter = ratelimit.New(options.RateLimit)
	} else if options.RateLimitMinute > 0 {
		ratelimiter = ratelimit.New(options.RateLimitMinute, ratelimit.Per(60*time.Second))
	}
	crawlerOptions := &CrawlerOptions{
		ExtensionsValidator: extensionsValidator,
		ScopeManager:        scopeManager,
		UniqueFilter:        itemFilter,
		RateLimit:           ratelimiter,
		Options:             options,
		OutputWriter:        outputWriter,
	}
	return crawlerOptions, nil
}

// Close closes the crawler options resources
func (c *CrawlerOptions) Close() error {
	return c.OutputWriter.Close()
}
