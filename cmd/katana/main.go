package main

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/internal/runner"
	"github.com/projectdiscovery/katana/pkg/output"
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

	flagSet.CreateGroup("input", "Input",
		flagSet.StringSliceVarP(&options.URLs, "list", "u", nil, "target url / list to crawl", goflags.FileCommaSeparatedStringSliceOptions),
	)

	flagSet.CreateGroup("configs", "Configurations",
		flagSet.StringVar(&cfgFile, "config", "", "path to the nuclei configuration file"),
		flagSet.IntVarP(&options.MaxDepth, "depth", "d", 2, "maximum depth to crawl"),
		flagSet.IntVarP(&options.CrawlDuration, "crawl-duration", "ct", 0, "maximum duration to crawl the target for"),
		flagSet.IntVarP(&options.BodyReadSize, "max-response-size", "mrs", 2*1024*1024, "maximum response size to read"),
		flagSet.IntVar(&options.Timeout, "timeout", 10, "time to wait for request in seconds"),
		flagSet.IntVar(&options.Retries, "retries", 1, "number of times to retry the request"),
		flagSet.StringVar(&options.Proxy, "proxy", "", "http/socks5 proxy to use"),
		flagSet.BoolVarP(&options.Headless, "headless", "he", false, "enable experimental headless hybrid crawling (process in one pass raw http requests/responses and dom-javascript web pages in browser context)"),
		flagSet.StringVar(&options.FormConfig, "form-config", "", "path to custom form configuration file"),
		flagSet.StringSliceVarP(&options.CustomHeaders, "headers", "H", nil, "custom header/cookie to include in request", goflags.StringSliceOptions),
		flagSet.BoolVarP(&options.UseInstalledChrome, "system-chrome", "sc", false, "Use local installed chrome browser instead of nuclei installed"),
		flagSet.BoolVarP(&options.ShowBrowser, "show-browser", "sb", false, "show the browser on the screen with headless mode"),
	)

	flagSet.CreateGroup("filters", "Filters",
		flagSet.StringVarP(&options.FieldScope, "field-scope", "fs", "rdn", "pre-defined scope field (dn,rdn,fqdn)"),
		flagSet.BoolVarP(&options.NoScope, "no-scope", "ns", false, "disables host based default scope"),
		flagSet.StringSliceVarP(&options.Scope, "crawl-scope", "cs", nil, "in scope url regex to be followed by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&options.OutOfScope, "crawl-out-scope", "cos", nil, "out of scope url regex to be excluded by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.BoolVarP(&options.ScrapeJSResponses, "js-crawl", "jc", false, "enable endpoint parsing / crawling in javascript file"),
		flagSet.StringSliceVarP(&options.Extensions, "extension", "e", nil, "extensions to be explicitly allowed for crawling (* means all - default)", goflags.CommaSeparatedStringSliceOptions),
		flagSet.StringSliceVar(&options.ExtensionsAllowList, "extensions-allow-list", nil, "extensions to allow from default deny list", goflags.CommaSeparatedStringSliceOptions),
		flagSet.StringSliceVar(&options.ExtensionDenyList, "extensions-deny-list", nil, "custom extensions for the crawl extensions deny list", goflags.CommaSeparatedStringSliceOptions),
	)

	flagSet.CreateGroup("ratelimit", "Rate-Limit",
		flagSet.IntVarP(&options.Concurrency, "concurrency", "c", 10, "number of concurrent fetchers to use"),
		flagSet.IntVarP(&options.Parallelism, "parallelism", "p", 10, "number of concurrent inputs to process"),
		flagSet.IntVarP(&options.Delay, "delay", "rd", 0, "request delay between each request in seconds"),
		flagSet.IntVarP(&options.RateLimit, "rate-limit", "rl", 150, "maximum requests to send per second"),
		flagSet.IntVarP(&options.RateLimitMinute, "rate-limit-minute", "rlm", 0, "maximum number of requests to send per minute"),
	)

	availableFields := strings.Join(output.FieldNames, ",")
	flagSet.CreateGroup("output", "Output",
		flagSet.StringVarP(&options.OutputFile, "output", "o", "", "file to write output to"),
		flagSet.StringVarP(&options.Fields, "fields", "f", "", fmt.Sprintf("field to display in output (%s)", availableFields)),
		flagSet.StringVarP(&options.StoreFields, "store-fields", "sf", "", fmt.Sprintf("field to store in per-host output (%s)", availableFields)),
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
