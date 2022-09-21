package main

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/internal/runner"
	"github.com/projectdiscovery/katana/pkg/types"
)

var (
	cfgFile string
	options = &types.Options{}
)

func main() {
	if err := readFlags(); err != nil {
		gologger.Fatal().Msgf("Could not read flags: %s\n", err)
	}

	if err := process(); err != nil {
		gologger.Fatal().Msgf("Could not process: %s\n", err)
	}
}

func process() error {
	runner, err := runner.New(options)
	if err != nil {
		return errors.Wrap(err, "could not create runner")
	}
	if runner == nil {
		return nil
	}
	defer runner.Close()

	if err := runner.ExecuteCrawling(); err != nil {
		return errors.Wrap(err, "could not execute crawling")
	}
	return nil
}

func readFlags() error {
	flagSet := goflags.NewFlagSet()
	flagSet.SetDescription(`Katana is a fast crawler focused on execution in automation
pipelines offering both headless and non-headless crawling.`)

	createGroup(flagSet, "input", "Input",
		flagSet.StringSliceVarP(&options.URLs, "list", "u", []string{}, "target url / list to crawl", goflags.FileCommaSeparatedStringSliceOptions),
	)

	createGroup(flagSet, "configs", "Configurations",
		flagSet.StringVar(&cfgFile, "config", "", "path to the nuclei configuration file"),
		flagSet.IntVarP(&options.MaxDepth, "depth", "d", 2, "maximum depth to crawl"),
		flagSet.IntVarP(&options.CrawlDuration, "crawl-duration", "ct", 0, "maximum duration to crawl the target for"),
		flagSet.IntVarP(&options.BodyReadSize, "max-response-size", "mrs", 2*1024*1024, "maximum response size to read"),
		flagSet.IntVar(&options.Timeout, "timeout", 10, "time to wait for request in seconds"),
		flagSet.IntVar(&options.Retries, "retries", 1, "number of times to retry the request"),
		flagSet.StringVar(&options.Proxy, "proxy", "", "http/socks5 proxy to use"),
		flagSet.RuntimeMapVarP(&options.CustomHeaders, "headers", "H", []string{}, "custom header/cookie to include in request"),
	)

	createGroup(flagSet, "filters", "Filters",
		flagSet.StringSliceVarP(&options.Scope, "crawl-scope", "cs", []string{}, "in scope url regex to be followed by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&options.OutOfScope, "crawl-out-scope", "cos", []string{}, "out of scope url regex to be excluded by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&options.ScopeDomains, "crawl-scope-domains", "csd", []string{}, "in scope hosts to be followed by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&options.OutOfScopeDomains, "crawl-out-scope-domains", "cosd", []string{}, "out of scope hosts to be excluded by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.BoolVarP(&options.IncludeSubdomains, "include-sub", "is", false, "include subdomains in crawl scope"),
		flagSet.BoolVarP(&options.ScrapeJSResponses, "js-crawl", "jc", false, "enable endpoint parsing / crawling in javascript file"),
		flagSet.StringSliceVarP(&options.Extensions, "extension", "e", []string{}, "extensions to be explicitly allowed for crawling (* means all - default)", goflags.CommaSeparatedStringSliceOptions),
		flagSet.StringSliceVar(&options.ExtensionsAllowList, "extensions-allow-list", []string{}, "extensions to allow from default deny list", goflags.CommaSeparatedStringSliceOptions),
		flagSet.StringSliceVar(&options.ExtensionDenyList, "extensions-deny-list", []string{}, "custom extensions for the crawl extensions deny list", goflags.CommaSeparatedStringSliceOptions),
	)

	createGroup(flagSet, "ratelimit", "Rate-Limit",
		flagSet.IntVarP(&options.Concurrency, "concurrency", "c", 10, "number of concurrent fetchers to use"),
		flagSet.IntVarP(&options.Parallelism, "parallelism", "p", 10, "number of concurrent inputs to process"),
		flagSet.IntVarP(&options.Delay, "delay", "rd", 0, "request delay between each request in seconds"),
		flagSet.IntVarP(&options.RateLimit, "rate-limit", "rl", 150, "maximum requests to send per second"),
		flagSet.IntVarP(&options.RateLimitMinute, "rate-limit-minute", "rlm", 0, "maximum number of requests to send per minute"),
	)

	createGroup(flagSet, "output", "Output",
		flagSet.StringVarP(&options.OutputFile, "output", "o", "", "file to write output to"),
		flagSet.StringVarP(&options.Fields, "fields", "f", "", "field to display in output (fqdn,rdn,url,rurl,path,file,key,value,kv) (default url)"),
		flagSet.BoolVarP(&options.JSON, "json", "j", false, "write output in JSONL(ines) format"),
		flagSet.BoolVarP(&options.NoColors, "no-color", "nc", false, "disable output content coloring (ANSI escape codes)"),
		flagSet.BoolVar(&options.Silent, "silent", false, "display output only"),
		flagSet.BoolVarP(&options.Verbose, "verbose", "v", false, "display verbose output"),
		flagSet.BoolVar(&options.Version, "version", false, "display project version"),
	)

	if err := flagSet.Parse(); err != nil {
		return errors.Wrap(err, "could not parse flags")
	}

	if cfgFile != "" {
		if err := flagSet.MergeConfigFile(cfgFile); err != nil {
			return errors.Wrap(err, "could not read config file")
		}
	}
	return nil
}

func createGroup(flagSet *goflags.FlagSet, groupName, description string, flags ...*goflags.FlagData) {
	flagSet.SetGroup(groupName, description)
	for _, currentFlag := range flags {
		currentFlag.Group(groupName)
	}
}
