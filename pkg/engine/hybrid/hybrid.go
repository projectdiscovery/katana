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
	urlutil "github.com/projectdiscovery/utils/url"
)

// Crawler is a standard crawler instance
type Crawler struct {
	*common.Shared

	browser *rod.Browser
	// TODO: Remove the Chrome PID kill code in favor of using Leakless(true).
	// This change will be made if there are no complaints about zombie Chrome processes.
	// References:
	// https://github.com/projectdiscovery/katana/issues/632
	// https://github.com/projectdiscovery/httpx/issues/1425
	// previousPIDs map[int32]struct{} // track already running PIDs
	tempDir string
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

	// previousPIDs := processutil.FindProcesses(processutil.IsChromeProcess)

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
		Shared:  shared,
		browser: browser,
		// previousPIDs: previousPIDs,
		tempDir: dataStore,
	}

	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	if c.Options.Options.ChromeDataDir == "" {
		if err := os.RemoveAll(c.tempDir); err != nil {
			return err
		}
	}
	// processutil.CloseProcesses(processutil.IsChromeProcess, c.previousPIDs)
	return nil
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
		Leakless(true).
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
		if options.Options.SystemChromePath != "" {
			chromeLauncher.Bin(options.Options.SystemChromePath)
		} else {
			if chromePath, hasChrome := launcher.LookPath(); hasChrome {
				chromeLauncher.Bin(chromePath)
			} else {
				return nil, errorutil.NewWithTag("hybrid", "the chrome browser is not installed").WithLevel(errorutil.Fatal)
			}
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
