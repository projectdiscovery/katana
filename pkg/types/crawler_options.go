package types

import (
	"context"
	"log/slog"
	"os/user"
	"regexp"
	"strings"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/secrets"
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
	// DitClassifier instance for knowledge base classification
	DitClassifier *dit.Classifier
	// SecretsScanner instance for secrets detection in HTTP responses
	SecretsScanner *secrets.Scanner

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

	if len(options.FilterPageType) > 0 {
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
		// Pass nil for exclude patterns to use defaults, or user-specified list
		var excludePatterns []string
		if len(options.SecretsExclude) > 0 {
			excludePatterns = options.SecretsExclude
		}
		secretsScanner, err := secrets.NewScanner(options.SecretsValidate, excludePatterns)
		if err != nil {
			return nil, errkit.Wrap(err, "could not init secrets scanner")
		}
		crawlerOptions.SecretsScanner = secretsScanner
	}

	if options.MaxOnclickLinks <= 0 {
		options.MaxOnclickLinks = 10
	}

	return crawlerOptions, nil
}

// Close closes the crawler options resources
func (c *CrawlerOptions) Close() error {
	c.UniqueFilter.Close()
	if c.SecretsScanner != nil {
		c.SecretsScanner.Close()
	}
	return c.OutputWriter.Close()
}

func (c *CrawlerOptions) ValidatePath(path string) bool {
	if c.ExtensionsValidator != nil {
		return c.ExtensionsValidator.ValidatePath(path)
	}
	return true
}

// ClassifyPage classifies a page using the dit classifier and returns the knowledge base map.
func (c *CrawlerOptions) ClassifyPage(body string) map[string]any {
	if c.DitClassifier == nil {
		return nil
	}
	result, err := c.DitClassifier.ExtractPageType(body)
	if err != nil {
		return nil
	}
	kb := map[string]any{
		"PageType": result.Type,
	}
	if len(result.Forms) > 0 {
		kb["Forms"] = result.Forms
	}
	return kb
}

// maxSecretsScanSize is the maximum response body size to scan for secrets (10MB).
const maxSecretsScanSize = 10 * 1024 * 1024

// skipContentTypes are content-type prefixes that are never scanned for secrets.
var skipContentTypes = []string{
	"image/",
	"audio/",
	"video/",
	"font/",
	"application/octet-stream",
	"application/pdf",
	"application/zip",
	"application/gzip",
	"application/wasm",
}

// ScanSecrets scans HTTP response headers and body for leaked secrets.
// Only responses are scanned — request data is excluded because it contains
// user-supplied credentials (form submissions, auth headers) which are not leaks.
// Skips binary content types and bodies larger than 512KB for performance.
func (c *CrawlerOptions) ScanSecrets(rr *output.Result) []secrets.Finding {
	if c.SecretsScanner == nil || rr == nil || rr.Response == nil {
		return nil
	}

	// Skip binary/non-text content types
	if ct, ok := rr.Response.Headers["content-type"]; ok {
		ctLower := strings.ToLower(ct)
		for _, skip := range skipContentTypes {
			if strings.HasPrefix(ctLower, skip) {
				return nil
			}
		}
	}

	url := ""
	if rr.Request != nil {
		url = rr.Request.URL
	}

	var findings []secrets.Finding
	if rr.Response.Headers != nil {
		findings = append(findings, c.SecretsScanner.ScanHeaders(map[string]string(rr.Response.Headers), "response_header", url)...)
	}
	if rr.Response.Body != "" && len(rr.Response.Body) <= maxSecretsScanSize {
		findings = append(findings, c.SecretsScanner.ScanString(rr.Response.Body, "response_body", url)...)
	}
	return findings
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
