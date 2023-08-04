package hybrid

import (
	"fmt"
	"os"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
	stringsutil "github.com/projectdiscovery/utils/strings"
	urlutil "github.com/projectdiscovery/utils/url"
	ps "github.com/shirou/gopsutil/v3/process"
	"go.uber.org/multierr"
)

// Crawler is a standard crawler instance
type Crawler struct {
	*common.Shared

	browser      *rod.Browser
	previousPIDs map[int32]struct{} // track already running PIDs
	tempDir      string
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	var dataStore string
	var err error
	if options.Options.ChromeDataDir != "" {
		dataStore = options.Options.ChromeDataDir
	} else {
		dataStore, err = os.MkdirTemp("", "katana-*")
		if err != nil {
			return nil, errorutil.NewWithTag("hybrid", "could not create temporary directory").Wrap(err)
		}
	}

	previousPIDs := findChromeProcesses()

	var launcherURL string
	var chromeLauncher *launcher.Launcher

	if options.Options.ChromeWSUrl != "" {
		launcherURL = options.Options.ChromeWSUrl
	} else {
		// create new chrome launcher instance
		chromeLauncher, err = buildChromeLauncher(options, dataStore)
		if err != nil {
			return nil, err
		}

		// launch chrome headless process
		launcherURL, err = chromeLauncher.Launch()
		if err != nil {
			return nil, err
		}
	}

	browser := rod.New().ControlURL(launcherURL)
	if browserErr := browser.Connect(); browserErr != nil {
		return nil, errorutil.NewWithErr(browserErr).Msgf("failed to connect to chrome instance at %s", launcherURL)
	}

	// create a new browser instance (default to incognito mode)
	if !options.Options.HeadlessNoIncognito {
		incognito, err := browser.Incognito()
		if err != nil {
			if chromeLauncher != nil {
				chromeLauncher.Kill()
			}
			return nil, errorutil.NewWithErr(err).Msgf("failed to create incognito browser")
		}
		browser = incognito
	}

	shared, err := common.NewShared(options)
	if err != nil {
		return nil, errorutil.NewWithErr(err).WithTag("hybrid")
	}

	crawler := &Crawler{
		Shared:       shared,
		browser:      browser,
		previousPIDs: previousPIDs,
		tempDir:      dataStore,
	}

	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	if c.Options.Options.ChromeWSUrl == "" {
		if err := c.browser.Close(); err != nil {
			return err
		}
	}
	if c.Options.Options.ChromeDataDir == "" {
		if err := os.RemoveAll(c.tempDir); err != nil {
			return err
		}
	}
	return c.killChromeProcesses()
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	crawlSession, err := c.NewCrawlSessionWithURL(rootURL)
	crawlSession.Browser = c.browser
	if err != nil {
		return errorutil.NewWithErr(err).WithTag("hybrid")
	}
	defer crawlSession.CancelFunc()

	gologger.Info().Msgf("Started headless crawling for => %v", rootURL)
	if err := c.Do(crawlSession, c.navigateRequest); err != nil {
		return errorutil.NewWithErr(err).WithTag("standard")
	}
	return nil
}

// buildChromeLauncher builds a new chrome launcher instance
func buildChromeLauncher(options *types.CrawlerOptions, dataStore string) (*launcher.Launcher, error) {
	chromeLauncher := launcher.New().
		Leakless(false).
		Set("disable-gpu", "true").
		Set("ignore-certificate-errors", "true").
		Set("ignore-certificate-errors", "1").
		Set("disable-crash-reporter", "true").
		Set("disable-notifications", "true").
		Set("hide-scrollbars", "true").
		Set("window-size", fmt.Sprintf("%d,%d", 1080, 1920)).
		Set("mute-audio", "true").
		Delete("use-mock-keychain").
		UserDataDir(dataStore)

	if options.Options.UseInstalledChrome {
		if chromePath, hasChrome := launcher.LookPath(); hasChrome {
			chromeLauncher.Bin(chromePath)
		} else {
			return nil, errorutil.NewWithTag("hybrid", "the chrome browser is not installed").WithLevel(errorutil.Fatal)
		}
	}
	if options.Options.SystemChromePath != "" {
		chromeLauncher.Bin(options.Options.SystemChromePath)
	}

	if options.Options.ShowBrowser {
		chromeLauncher = chromeLauncher.Headless(false)
	} else {
		chromeLauncher = chromeLauncher.Headless(true)
	}

	if options.Options.HeadlessNoSandbox {
		chromeLauncher.Set("no-sandbox", "true")
	}

	if options.Options.Proxy != "" && options.Options.Headless {
		proxyURL, err := urlutil.Parse(options.Options.Proxy)
		if err != nil {
			return nil, err
		}
		chromeLauncher.Set("proxy-server", proxyURL.String())
	}

	for k, v := range options.Options.ParseHeadlessOptionalArguments() {
		chromeLauncher.Set(flags.Flag(k), v)
	}

	return chromeLauncher, nil
}

// killChromeProcesses any and all new chrome processes started after
// headless process launch.
func (c *Crawler) killChromeProcesses() error {
	var errs []error
	processes, _ := ps.Processes()

	for _, process := range processes {
		// skip non-chrome processes
		if !isChromeProcess(process) {
			continue
		}

		// skip chrome processes that were already running
		if _, ok := c.previousPIDs[process.Pid]; ok {
			continue
		}

		if err := process.Kill(); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.Combine(errs...)
}

// findChromeProcesses finds chrome process running on host
func findChromeProcesses() map[int32]struct{} {
	processes, _ := ps.Processes()
	list := make(map[int32]struct{})
	for _, process := range processes {
		if isChromeProcess(process) {
			list[process.Pid] = struct{}{}
			if ppid, err := process.Ppid(); err == nil {
				list[ppid] = struct{}{}
			}
		}
	}
	return list
}

// isChromeProcess checks if a process is chrome/chromium
func isChromeProcess(process *ps.Process) bool {
	name, _ := process.Name()
	executable, _ := process.Exe()
	return stringsutil.ContainsAny(name, "chrome", "chromium") || stringsutil.ContainsAny(executable, "chrome", "chromium")
}
