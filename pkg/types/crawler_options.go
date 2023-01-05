package types

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/ratelimit"
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
	// Dialer is instance of the dialer for global crawler
	Dialer *fastdialer.Dialer
}

// NewCrawlerOptions creates a new crawler options structure
// from user specified options.
func NewCrawlerOptions(options *Options) (*CrawlerOptions, error) {
	extensionsValidator := extensions.NewValidator(options.ExtensionsMatch, options.ExtensionFilter)

	fastdialerInstance, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	if err != nil {
		return nil, err
	}
	scopeManager, err := scope.NewManager(options.Scope, options.OutOfScope, options.FieldScope, options.NoScope)
	if err != nil {
		return nil, errors.Wrap(err, "could not create scope manager")
	}
	itemFilter, err := filters.NewSimple()
	if err != nil {
		return nil, errors.Wrap(err, "could not create filter")
	}

	outputWriter, err := output.New(!options.NoColors, options.JSON, options.Verbose, options.StoreResponse, options.OutputFile, options.Fields, options.StoreFields, options.StoreResponseDir, options.FieldConfig)
	if err != nil {
		return nil, errors.Wrap(err, "could not create output writer")
	}

	var ratelimiter ratelimit.Limiter
	if options.RateLimit > 0 {
		ratelimiter = *ratelimit.New(context.Background(), uint(options.RateLimit), time.Second)
	} else if options.RateLimitMinute > 0 {
		ratelimiter = *ratelimit.New(context.Background(), uint(options.RateLimitMinute), time.Minute)
	}

	crawlerOptions := &CrawlerOptions{
		ExtensionsValidator: extensionsValidator,
		ScopeManager:        scopeManager,
		UniqueFilter:        itemFilter,
		RateLimit:           ratelimiter,
		Options:             options,
		Dialer:              fastdialerInstance,
		OutputWriter:        outputWriter,
	}
	return crawlerOptions, nil
}

// Close closes the crawler options resources
func (c *CrawlerOptions) Close() error {
	c.UniqueFilter.Close()
	return c.OutputWriter.Close()
}
