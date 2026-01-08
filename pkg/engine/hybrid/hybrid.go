package hybrid

import (
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
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

// Do executes the crawling loop with browser-safe concurrency.
// Unlike the base implementation, this uses sequential processing (concurrency=1)
// because Chrome DevTools Protocol operations cannot safely run concurrently
// on the same browser instance. Multiple concurrent page operations cause
// race conditions, navigation conflicts, and network interception issues.
func (c *Crawler) Do(crawlSession *common.CrawlSession, doRequest common.DoRequestFunc) error {
	for item := range crawlSession.Queue.Pop() {
		if ctxErr := crawlSession.Ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		req, ok := item.(*navigation.Request)
		if !ok {
			continue
		}

		if !utils.IsURL(req.URL) {
			if c.Options.Options.OnSkipURL != nil {
				c.Options.Options.OnSkipURL(req.URL)
			}
			gologger.Debug().Msgf("`%v` not a url. skipping", req.URL)
			continue
		}

		if !c.Options.ValidatePath(req.URL) {
			gologger.Debug().Msgf("`%v` filtered path. skipping", req.URL)
			continue
		}

		inScope, scopeErr := c.Options.ValidateScope(req.URL, crawlSession.Hostname)
		if scopeErr != nil {
			gologger.Debug().Msgf("Error validating scope for `%v`: %v. skipping", req.URL, scopeErr)
			continue
		}
		if !req.SkipValidation && !inScope {
			gologger.Debug().Msgf("`%v` not in scope. skipping", req.URL)
			continue
		}

		c.Options.RateLimit.Take()

		if c.Options.Options.Delay > 0 {
			time.Sleep(time.Duration(c.Options.Options.Delay) * time.Second)
		}

		resp, err := doRequest(crawlSession, req)

		if inScope {
			c.Output(req, resp, err)
		}

		if err != nil {
			gologger.Warning().Msgf("Could not request seed URL %s: %s\n", req.URL, err)
			outputError := &output.Error{
				Timestamp: time.Now(),
				Endpoint:  req.RequestURL(),
				Source:    req.Source,
				Error:     err.Error(),
			}
			_ = c.Options.OutputWriter.WriteErr(outputError)
			continue
		}
		if resp == nil || resp.Resp == nil || resp.Reader == nil {
			continue
		}
		if c.Options.Options.DisableRedirects && resp.IsRedirect() {
			continue
		}

		navigationRequests := c.Options.Parser.ParseResponse(resp)
		c.Enqueue(crawlSession.Queue, navigationRequests...)
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
