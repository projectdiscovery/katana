package types

import (
	"context"
	"strconv"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/mapcidr"
	"github.com/projectdiscovery/mapcidr/asn"
	"github.com/projectdiscovery/networkpolicy"
	"github.com/projectdiscovery/ratelimit"
	errorutil "github.com/projectdiscovery/utils/errors"
	iputil "github.com/projectdiscovery/utils/ip"
	urlutil "github.com/projectdiscovery/utils/url"
	wappalyzer "github.com/projectdiscovery/wappalyzergo"
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
	// Wappalyzer instance for technologies detection
	Wappalyzer *wappalyzer.Wappalyze
}

// NewCrawlerOptions creates a new crawler options structure
// from user specified options.
func NewCrawlerOptions(options *Options) (*CrawlerOptions, error) {
	extensionsValidator := extensions.NewValidator(options.ExtensionsMatch, options.ExtensionFilter)

	dialerOpts := fastdialer.DefaultOptions
	if len(options.Resolvers) > 0 {
		dialerOpts.BaseResolvers = options.Resolvers
	}
	for _, exclude := range options.Exclude {
		switch {
		case exclude == "cdn":
			//implement cdn check in netoworkpolicy pkg??
			continue
		case exclude == "private-ips":
			dialerOpts.Deny = append(dialerOpts.Deny, networkpolicy.DefaultIPv4Denylist...)
			dialerOpts.Deny = append(dialerOpts.Deny, networkpolicy.DefaultIPv4DenylistRanges...)
			dialerOpts.Deny = append(dialerOpts.Deny, networkpolicy.DefaultIPv6Denylist...)
			dialerOpts.Deny = append(dialerOpts.Deny, networkpolicy.DefaultIPv6DenylistRanges...)
		case iputil.IsCIDR(exclude):
			dialerOpts.Deny = append(dialerOpts.Deny, exclude)
		case asn.IsASN(exclude):
			// update this to use networkpolicy pkg once https://github.com/projectdiscovery/networkpolicy/pull/55 is merged
			ips := expandASNInputValue(exclude)
			dialerOpts.Deny = append(dialerOpts.Deny, ips...)
		case iputil.IsPort(exclude):
			port, _ := strconv.Atoi(exclude)
			dialerOpts.DenyPortList = append(dialerOpts.DenyPortList, port)
		default:
			dialerOpts.Deny = append(dialerOpts.Deny, exclude)
		}
	}

	fastdialerInstance, err := fastdialer.NewDialer(dialerOpts)
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
		Colors:                !options.NoColors,
		JSON:                  options.JSON,
		Verbose:               options.Verbose,
		StoreResponse:         options.StoreResponse,
		OutputFile:            options.OutputFile,
		Fields:                options.Fields,
		StoreFields:           options.StoreFields,
		StoreResponseDir:      options.StoreResponseDir,
		OmitRaw:               options.OmitRaw,
		OmitBody:              options.OmitBody,
		FieldConfig:           options.FieldConfig,
		ErrorLogFile:          options.ErrorLogFile,
		MatchRegex:            options.MatchRegex,
		FilterRegex:           options.FilterRegex,
		ExtensionValidator:    extensionsValidator,
		OutputMatchCondition:  options.OutputMatchCondition,
		OutputFilterCondition: options.OutputFilterCondition,
	}
	outputWriter, err := output.New(outputOptions)
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create output writer")
	}

	crawlerOptions := &CrawlerOptions{
		ExtensionsValidator: extensionsValidator,
		ScopeManager:        scopeManager,
		UniqueFilter:        itemFilter,
		Options:             options,
		Dialer:              fastdialerInstance,
		OutputWriter:        outputWriter,
	}

	if options.RateLimit > 0 {
		crawlerOptions.RateLimit = *ratelimit.New(context.Background(), uint(options.RateLimit), time.Second)
	} else if options.RateLimitMinute > 0 {
		crawlerOptions.RateLimit = *ratelimit.New(context.Background(), uint(options.RateLimitMinute), time.Minute)
	}

	wappalyze, err := wappalyzer.New()
	if err != nil {
		return nil, err
	}
	crawlerOptions.Wappalyzer = wappalyze

	return crawlerOptions, nil
}

func expandCIDRInputValue(value string) []string {
	var ips []string
	ipsCh, _ := mapcidr.IPAddressesAsStream(value)
	for ip := range ipsCh {
		ips = append(ips, ip)
	}
	return ips
}

func expandASNInputValue(value string) []string {
	var ips []string
	cidrs, _ := asn.GetCIDRsForASNNum(value)
	for _, cidr := range cidrs {
		ips = append(ips, expandCIDRInputValue(cidr.String())...)
	}
	return ips
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
