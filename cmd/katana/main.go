package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/internal/runner"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
)

var (
	cfgFile string
	options = &types.Options{}
)

func main() {
	flagSet, err := readFlags()
	if err != nil {
		gologger.Fatal().Msgf("Could not read flags: %s\n", err)
	}

	if options.HealthCheck {
		gologger.Print().Msgf("%s\n", runner.DoHealthCheck(options, flagSet))
		os.Exit(0)
	}

	katanaRunner, err := runner.New(options)
	if err != nil || katanaRunner == nil {
		if options.Version {
			return
		} else {
			gologger.Fatal().Msgf("could not create runner: %s\n", err)
		}
	}
	defer katanaRunner.Close()

	// close handler
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			gologger.DefaultLogger.Info().Msg("- Ctrl+C pressed in Terminal")
			katanaRunner.Close()
			os.Exit(0)
		}()
	}()

	if err := katanaRunner.ExecuteCrawling(); err != nil {
		gologger.Fatal().Msgf("could not execute crawling: %s", err)
	}

}

func readFlags() (*goflags.FlagSet, error) {
	flagSet := goflags.NewFlagSet()
	flagSet.SetDescription(`Katana is a fast crawler focused on execution in automation
pipelines offering both headless and non-headless crawling.`)

	flagSet.CreateGroup("input", "Input",
		flagSet.StringSliceVarP(&options.URLs, "list", "u", nil, "target url / list to crawl", goflags.FileCommaSeparatedStringSliceOptions),
	)

	flagSet.CreateGroup("config", "Configuration",
		flagSet.IntVarP(&options.MaxDepth, "depth", "d", 2, "maximum depth to crawl"),
		flagSet.BoolVarP(&options.ScrapeJSResponses, "js-crawl", "jc", false, "enable endpoint parsing / crawling in javascript file"),
		flagSet.IntVarP(&options.CrawlDuration, "crawl-duration", "ct", 0, "maximum duration to crawl the target for"),
		flagSet.StringVarP(&options.KnownFiles, "known-files", "kf", "", "enable crawling of known files (all,robotstxt,sitemapxml)"),
		flagSet.IntVarP(&options.BodyReadSize, "max-response-size", "mrs", 2*1024*1024, "maximum response size to read"),
		flagSet.IntVar(&options.Timeout, "timeout", 10, "time to wait for request in seconds"),
		flagSet.BoolVarP(&options.AutomaticFormFill, "automatic-form-fill", "aff", false, "enable automatic form filling (experimental)"),
		flagSet.IntVar(&options.Retries, "retry", 1, "number of times to retry the request"),
		flagSet.StringVar(&options.Proxy, "proxy", "", "http/socks5 proxy to use"),
		flagSet.StringSliceVarP(&options.CustomHeaders, "headers", "H", nil, "custom header/cookie to include in request", goflags.StringSliceOptions),
		flagSet.StringVar(&cfgFile, "config", "", "path to the katana configuration file"),
		flagSet.StringVarP(&options.FormConfig, "form-config", "fc", "", "path to custom form configuration file"),
		flagSet.StringVarP(&options.FieldConfig, "field-config", "flc", "", "path to custom field configuration file"),
	)

	flagSet.CreateGroup("debug", "Debug",
		flagSet.BoolVarP(&options.HealthCheck, "hc", "health-check", false, "run diagnostic check up"),
		flagSet.StringVarP(&options.ErrorLogFile, "error-log", "elog", "", "file to write sent requests error log"),
	)

	flagSet.CreateGroup("headless", "Headless",
		flagSet.BoolVarP(&options.Headless, "headless", "hl", false, "enable headless hybrid crawling (experimental)"),
		flagSet.BoolVarP(&options.UseInstalledChrome, "system-chrome", "sc", false, "use local installed chrome browser instead of katana installed"),
		flagSet.BoolVarP(&options.ShowBrowser, "show-browser", "sb", false, "show the browser on the screen with headless mode"),
		flagSet.StringSliceVarP(&options.HeadlessOptionalArguments, "headless-options", "ho", nil, "start headless chrome with additional options", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.BoolVarP(&options.HeadlessNoSandbox, "no-sandbox", "nos", false, "start headless chrome in --no-sandbox mode"),
		flagSet.StringVarP(&options.SystemChromePath, "system-chrome-path", "scp", "", "use specified chrome browser for headless crawling"),
		flagSet.BoolVarP(&options.HeadlessNoIncognito, "no-incognito", "noi", false, "start headless chrome without incognito mode"),
	)

	flagSet.CreateGroup("project", "Project",
		flagSet.StringVarP(&options.NewProject, "new-project", "np", "", "creates new project and opens browser for manual authentication"),
		flagSet.StringVarP(&options.CrawlProject, "crawl-project", "cp", "", "use project data while crawling (uses saved auth/session data)"),
		flagSet.BoolVarP(&options.ListProject, "list-project", "lp", false, "list all stored projects"),
	)

	flagSet.CreateGroup("scope", "Scope",
		flagSet.StringSliceVarP(&options.Scope, "crawl-scope", "cs", nil, "in scope url regex to be followed by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&options.OutOfScope, "crawl-out-scope", "cos", nil, "out of scope url regex to be excluded by crawler", goflags.FileCommaSeparatedStringSliceOptions),
		flagSet.StringVarP(&options.FieldScope, "field-scope", "fs", "rdn", "pre-defined scope field (dn,rdn,fqdn)"),
		flagSet.BoolVarP(&options.NoScope, "no-scope", "ns", false, "disables host based default scope"),
		flagSet.BoolVarP(&options.DisplayOutScope, "display-out-scope", "do", false, "display external endpoint from scoped crawling"),
	)

	availableFields := strings.Join(output.FieldNames, ",")
	flagSet.CreateGroup("filter", "Filter",
		flagSet.StringVarP(&options.Fields, "field", "f", "", fmt.Sprintf("field to display in output (%s)", availableFields)),
		flagSet.StringVarP(&options.StoreFields, "store-field", "sf", "", fmt.Sprintf("field to store in per-host output (%s)", availableFields)),
		flagSet.StringSliceVarP(&options.ExtensionsMatch, "extension-match", "em", nil, "match output for given extension (eg, -em php,html,js)", goflags.CommaSeparatedStringSliceOptions),
		flagSet.StringSliceVarP(&options.ExtensionFilter, "extension-filter", "ef", nil, "filter output for given extension (eg, -ef png,css)", goflags.CommaSeparatedStringSliceOptions),
	)

	flagSet.CreateGroup("ratelimit", "Rate-Limit",
		flagSet.IntVarP(&options.Concurrency, "concurrency", "c", 10, "number of concurrent fetchers to use"),
		flagSet.IntVarP(&options.Parallelism, "parallelism", "p", 10, "number of concurrent inputs to process"),
		flagSet.IntVarP(&options.Delay, "delay", "rd", 0, "request delay between each request in seconds"),
		flagSet.IntVarP(&options.RateLimit, "rate-limit", "rl", 150, "maximum requests to send per second"),
		flagSet.IntVarP(&options.RateLimitMinute, "rate-limit-minute", "rlm", 0, "maximum number of requests to send per minute"),
	)

	flagSet.CreateGroup("output", "Output",
		flagSet.StringVarP(&options.OutputFile, "output", "o", "", "file to write output to"),
		flagSet.BoolVarP(&options.StoreResponse, "store-response", "sr", false, "store http requests/responses"),
		flagSet.StringVarP(&options.StoreResponseDir, "store-response-dir", "srd", "", "store http requests/responses to custom directory"),
		flagSet.BoolVarP(&options.JSON, "json", "j", false, "write output in JSONL(ines) format"),
		flagSet.BoolVarP(&options.NoColors, "no-color", "nc", false, "disable output content coloring (ANSI escape codes)"),
		flagSet.BoolVar(&options.Silent, "silent", false, "display output only"),
		flagSet.BoolVarP(&options.Verbose, "verbose", "v", false, "display verbose output"),
		flagSet.BoolVar(&options.Version, "version", false, "display project version"),
	)

	if err := flagSet.Parse(); err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not parse flags")
	}

	if cfgFile != "" {
		if err := flagSet.MergeConfigFile(cfgFile); err != nil {
			return nil, errorutil.NewWithErr(err).Msgf("could not read config file")
		}
	}
	return flagSet, nil
}

func init() {
	// show detailed stacktrace in debug mode
	if os.Getenv("DEBUG") != "" {
		errorutil.ShowStackTrace = true
	}
}
