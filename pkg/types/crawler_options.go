package types

import (
	"context"
	"log/slog"
	"net/http"
	"os/user"
	"regexp"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/knowledgebase"
	"github.com/projectdiscovery/katana/pkg/knowledgebase/extractors/endpoints"
	"github.com/projectdiscovery/katana/pkg/knowledgebase/extractors/secrets"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/ratelimit"
	"github.com/happyhackingspace/dit"
	"github.com/projectdiscovery/utils/errkit"
	urlutil "github.com/projectdiscovery/utils/url"
	wappalyzer "github.com/projectdiscovery/wappalyzergo"
)

// CrawlerOptions contains helper utilities for the crawler
type CrawlerOptions struct {
	// OutputWriter is the interface for writing output
	OutputWriter output.Writer
	// RateLimit is the global rate limiter (used when -rl is set)
	RateLimit *ratelimit.Limiter
	// HostRateLimit is the per-host rate limiter (used when -hrl is set, replaces global)
	HostRateLimit *ratelimit.AutoLimiter
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
	// DitClassifier instance for knowledge base classification
	DitClassifier *dit.Classifier
	// Extractors is the chain of knowledgebase.Extractor implementations whose
	// outputs are merged into the response KnowledgeBase map by BuildKnowledgeBase.
	Extractors []knowledgebase.Extractor

	// Optional structured logger for headless crawler
	Logger *slog.Logger
	// ChromeUser is the user to use for chrome
	ChromeUser *user.User
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
		FilterPageType:        options.FilterPageType,
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

	crawlerOptions := &CrawlerOptions{
		ExtensionsValidator: extensionsValidator,
		Parser:              responseParser,
		ScopeManager:        scopeManager,
		UniqueFilter:        itemFilter,
		Options:             options,
		Dialer:              fastdialerInstance,
		OutputWriter:        outputWriter,
	}

	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if options.HostRateLimit > 0 {
		crawlerOptions.HostRateLimit = ratelimit.NewAutoLimiter(ctx, ratelimit.WithMaxCount(uint(options.HostRateLimit)), ratelimit.WithDuration(time.Second))
	} else if options.HostRateLimitMinute > 0 {
		crawlerOptions.HostRateLimit = ratelimit.NewAutoLimiter(ctx, ratelimit.WithMaxCount(uint(options.HostRateLimitMinute)), ratelimit.WithDuration(time.Minute))
	} else if options.RateLimit > 0 {
		crawlerOptions.RateLimit = ratelimit.New(ctx, uint(options.RateLimit), time.Second)
	} else if options.RateLimitMinute > 0 {
		crawlerOptions.RateLimit = ratelimit.New(ctx, uint(options.RateLimitMinute), time.Minute)
	}

	if options.TechDetect {
		wappalyze, err := wappalyzer.New()
		if err != nil {
			return nil, err
		}
		crawlerOptions.Wappalyzer = wappalyze
	}

	if len(options.FilterPageType) > 0 || options.AuthCredentials != "" {
		options.KnowledgeBase = true
	}
	if options.KnowledgeBase {
		classifier, err := dit.New()
		if err != nil {
			return nil, errkit.Wrap(err, "could not init dit classifier")
		}
		crawlerOptions.DitClassifier = classifier
	}

	if options.Secrets {
		secretsExtractor, err := secrets.New(secrets.Config{Validate: options.ValidateSecrets})
		if err != nil {
			return nil, errkit.Wrap(err, "could not init secrets extractor")
		}
		crawlerOptions.Extractors = append(crawlerOptions.Extractors, secretsExtractor)
	}

	if options.Endpoints {
		crawlerOptions.Extractors = append(crawlerOptions.Extractors, endpoints.New())
	}

	if options.MaxOnclickLinks <= 0 {
		options.MaxOnclickLinks = 10
	}

	return crawlerOptions, nil
}

// Close closes the crawler options resources
func (c *CrawlerOptions) Close() error {
	if c.RateLimit != nil {
		c.RateLimit.Stop()
	}
	if c.HostRateLimit != nil {
		c.HostRateLimit.Stop()
	}
	if c.Dialer != nil {
		c.Dialer.Close()
	}
	for _, e := range c.Extractors {
		if closer, ok := e.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}
	c.UniqueFilter.Close()
	return c.OutputWriter.Close()
}

func (c *CrawlerOptions) ValidatePath(path string) bool {
	if c.ExtensionsValidator != nil {
		return c.ExtensionsValidator.ValidatePath(path)
	}
	return true
}

// BuildKnowledgeBase assembles the response KnowledgeBase map by merging
// output from the dit page-type classifier (when enabled) with each registered
// Extractor. Returns nil when no producer is configured or none produced output.
//
// body is the fully drained response body (resp.Body has already been
// consumed by the caller). req and resp are forwarded to extractors that
// classify by request shape (endpoints, headers_audit, etc.); body-only
// extractors ignore them. Extractors MUST treat req/resp as read-only.
func (c *CrawlerOptions) BuildKnowledgeBase(body string, req *http.Request, resp *http.Response) map[string]any {
	if c.DitClassifier == nil && len(c.Extractors) == 0 {
		return nil
	}
	kb := map[string]any{}
	if c.DitClassifier != nil {
		if result, err := c.DitClassifier.ExtractPageType(body); err == nil {
			kb["PageType"] = result.Type
			if len(result.Forms) > 0 {
				kb["Forms"] = result.Forms
			}
		}
	}
	for _, e := range c.Extractors {
		if out := e.Extract(body, req, resp); out != nil {
			kb[e.Name()] = out
		}
	}
	if len(kb) == 0 {
		return nil
	}
	return kb
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
