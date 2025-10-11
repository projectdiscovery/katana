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
	"github.com/projectdiscovery/utils/errkit"
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
			return nil, errkit.Wrap(err, "hybrid: could not create temporary directory")
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
		return nil, errkit.Wrap(browserErr, fmt.Sprintf("hybrid: failed to connect to chrome instance at %s", launcherURL))
	}

	// create a new browser instance (default to incognito mode)
	if !options.Options.HeadlessNoIncognito {
		incognito, err := browser.Incognito()
		if err != nil {
			if chromeLauncher != nil {
				chromeLauncher.Kill()
			}
			return nil, errkit.Wrap(err, "hybrid: failed to create incognito browser")
		}
		browser = incognito
	}

	shared, err := common.NewShared(options)
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid")
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
    // Close browser first to release locks
    if c.browser != nil {
        _ = c.browser.Close()
    }
    // Kill launcher (best-effort) to ensure chrome exits
    if c.launcher != nil {
        c.launcher.Kill()
    }

    if c.Options.Options.ChromeDataDir == "" {
        // Retry removal on Windows because Chrome Crashpad may keep files briefly locked
        // Try for up to ~5 seconds
        const maxAttempts = 25
        var lastErr error
        for attempt := 1; attempt <= maxAttempts; attempt++ {
            if err := os.RemoveAll(c.tempDir); err != nil {
                lastErr = err
                if runtime.GOOS == "windows" {
                    msg := err.Error()
                    if strings.Contains(msg, "CrashpadMetrics") || strings.Contains(msg, "Access is denied") {
                        time.Sleep(200 * time.Millisecond)
                        continue
                    }
                }
                break
            }
            lastErr = nil
            break
        }
        if lastErr != nil {
            if runtime.GOOS == "windows" {
                return nil
            }
            return lastErr
        }
    }
	// processutil.CloseProcesses(processutil.IsChromeProcess, c.previousPIDs)
	return nil
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	crawlSession, err := c.NewCrawlSessionWithURL(rootURL)
	if err != nil {
		return errkit.Wrap(err, "hybrid")
	}
	crawlSession.Browser = c.browser

	defer crawlSession.CancelFunc()

	gologger.Info().Msgf("Started headless crawling for => %v", rootURL)
	if err := c.Do(crawlSession, c.navigateRequest); err != nil {
		return errkit.Wrap(err, "hybrid")
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
				return nil, errkit.New("hybrid: the chrome browser is not installed")
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

	// Handle proxy configuration for hybrid mode
	if options.Options.Proxy != "" && options.Options.Headless {
		proxyURL, err := urlutil.Parse(options.Options.Proxy)
		if err != nil {
			return nil, err
		}
		
		// If proxy filtering is enabled, we need to handle proxy settings differently
		if options.Options.ProxyFiltering && options.ProxyFilterPipeline != nil && options.ProxyFilterPipeline.IsEnabled() {
			// In proxy filtering mode, we don't set browser-level proxy
			// Instead, we'll handle proxy decisions per-request in the request interceptor
			gologger.Debug().Msgf("Hybrid mode: Proxy filtering enabled, will handle proxy per-request")
		} else {
			// Traditional mode: set browser-level proxy for all requests
			chromeLauncher.Set("proxy-server", proxyURL.String())
			gologger.Debug().Msgf("Hybrid mode: Browser-level proxy set to %s", proxyURL.String())
		}
	}

	for k, v := range options.Options.ParseHeadlessOptionalArguments() {
		chromeLauncher.Set(flags.Flag(k), v)
	}

	return chromeLauncher, nil
}
