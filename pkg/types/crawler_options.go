package types

import (
	"context"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/ratelimit"
	errorutil "github.com/projectdiscovery/utils/errors"
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
		return nil, errorutil.NewWithErr(err).Msgf("could not create scope manager")
	}
	itemFilter, err := filters.NewSimple()
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create filter")
	}

	outputOptions := output.Options{
		Colors:           !options.NoColors,
		JSON:             options.JSON,
		Verbose:          options.Verbose,
		StoreResponse:    options.StoreResponse,
		OutputFile:       options.OutputFile,
		Fields:           options.Fields,
		StoreFields:      options.StoreFields,
		StoreResponseDir: options.StoreResponseDir,
		FieldConfig:      options.FieldConfig,
		ErrorLogFile:     options.ErrorLogFile,
	}
	outputWriter, err := output.New(outputOptions)
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create output writer")
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
