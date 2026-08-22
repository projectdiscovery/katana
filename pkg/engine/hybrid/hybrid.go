package hybrid

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/utils/chromeshell"
	"github.com/projectdiscovery/utils/errkit"
	urlutil "github.com/projectdiscovery/utils/url"
)

// Crawler is a standard crawler instance
type Crawler struct {
	*common.Shared

	browser        *rod.Browser
	chromeLauncher *launcher.Launcher // nil when attached via ChromeWSUrl
	cdpWS          *cdp.WebSocket
	// TODO: Remove the Chrome PID kill code in favor of using Leakless(true).
	// This change will be made if there are no complaints about zombie Chrome processes.
	// References:
	// https://github.com/projectdiscovery/katana/issues/632
	// https://github.com/projectdiscovery/httpx/issues/1425
	// previousPIDs map[int32]struct{} // track already running PIDs
	tempDir string
}

// proxyBypassList returns the Chrome proxy bypass list to use for proxy.
//
// Chrome bypasses the proxy for localhost, 127.0.0.0/8, [::1] and link-local
// addresses by default, even when one is configured. "<-loopback>" is the
// documented way to SUBTRACT that implicit rule, so a configured proxy is
// honoured for local targets too -- the case where an intercepting proxy is
// most often used. Empty when no proxy is set, since there is nothing to
// bypass.
func proxyBypassList(proxy string) string {
	if proxy == "" {
		return ""
	}
	return "<-loopback>"
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

	// Construct the CDP client here rather than using rod.New().ControlURL(...)
	// so the websocket handle survives into Close(). rod hides it in an
	// unexported field, and without it the connection can never be closed --
	// see Close() for why that leaks. StartWithURL, which Connect calls, is
	// exactly this.
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
	cdpWS := &cdp.WebSocket{}
	wsErr := cdpWS.Connect(dialCtx, launcherURL, nil)
	dialCancel()
	if wsErr != nil {
		if chromeLauncher != nil {
			chromeLauncher.Kill()
		}
		return nil, errkit.Wrap(wsErr, fmt.Sprintf("hybrid: failed to connect to chrome instance at %s", launcherURL))
	}
	browser := rod.New().Client(cdp.New().Start(cdpWS))
	if browserErr := browser.Connect(); browserErr != nil {
		_ = cdpWS.Close()
		if chromeLauncher != nil {
			chromeLauncher.Kill()
		}
		return nil, errkit.Wrap(browserErr, fmt.Sprintf("hybrid: failed to connect to chrome instance at %s", launcherURL))
	}

	owned := false
	defer func() {
		if owned {
			return
		}
		_ = browser.Close()
		_ = cdpWS.Close()
		if chromeLauncher != nil {
			chromeLauncher.Kill()
		}
	}()

	// create a new browser instance (default to incognito mode)
	if !options.Options.HeadlessNoIncognito {
		// Create the browser context directly rather than via browser.Incognito():
		// rod's helper takes no proxy argument, and Options.Proxy otherwise never
		// reaches a browser attached through ChromeWSUrl, because the chrome
		// launcher -- its only proxy path -- does not run in that case.
		res, err := proto.TargetCreateBrowserContext{
			ProxyServer: options.Options.Proxy,
			// "<-loopback>" SUBTRACTS Chrome's implicit proxy bypass.
			//
			// Chrome always bypasses the proxy for localhost, 127.0.0.0/8,
			// [::1] and link-local, even when one is configured; the only way
			// to disable that built-in rule is to subtract it here. Without
			// this, `-proxy` is silently ignored for exactly the local targets
			// people most often put behind an intercepting proxy, and the
			// crawl still succeeds -- so the traffic simply never appears.
			//
			// Only applied when a proxy was actually requested: with no proxy
			// there is nothing to bypass, and the token would be meaningless.
			ProxyBypassList: proxyBypassList(options.Options.Proxy),
		}.Call(browser)
		if err != nil {
			return nil, errkit.Wrap(err, "hybrid: failed to create incognito browser")
		}
		incognito := *browser
		incognito.BrowserContextID = res.BrowserContextID
		browser = &incognito
	}

	shared, err := common.NewShared(options)
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid")
	}

	crawler := &Crawler{
		Shared:         shared,
		browser:        browser,
		chromeLauncher: chromeLauncher,
		cdpWS:          cdpWS,
		// previousPIDs: previousPIDs,
		tempDir: dataStore,
	}
	owned = true

	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	if c.browser != nil {
		_ = c.browser.Close()
	}
	if c.cdpWS != nil {
		// Close AFTER browser.Close, which dispatches
		// Target.disposeBrowserContext over this same socket.
		//
		// rod's initEvents goroutine blocks on `for e := range client.Event()`,
		// and that channel closes only when cdp's read loop errors -- which
		// happens only when the CONNECTION closes, not on context cancellation
		// (the websocket Read is a blocking socket read). Disposing the browser
		// context does not close the connection, so against a browser attached
		// via ChromeWSUrl -- where there is no launcher to kill -- every crawl
		// left the socket and its goroutines behind for the browser's lifetime.
		_ = c.cdpWS.Close()
	}
	if c.chromeLauncher != nil {
		c.chromeLauncher.Kill()
	}
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
	for item := range crawlSession.Queue.PopWithContext(crawlSession.Ctx) {
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

		// Race Take() against the session context so the loop doesn't
		// block on a limiter tick when the crawl has been cancelled.
		//
		// Note: when the session is cancelled mid-Take, this inner
		// goroutine outlives the loop iteration and stays blocked on
		// the limiter until the next tick or until RateLimit.Stop() is
		// called by CrawlerOptions.Close(). The leak is bounded by
		// Close() and acceptable.
		if crawlSession.Ctx.Err() != nil {
			continue
		}
		takeDone := make(chan struct{})
		go func() {
			if c.Options.HostRateLimit != nil {
				_ = c.Options.HostRateLimit.Take(crawlSession.Hostname)
			} else if c.Options.RateLimit != nil {
				c.Options.RateLimit.Take()
			}
			close(takeDone)
		}()
		select {
		case <-crawlSession.Ctx.Done():
			continue
		case <-takeDone:
		}
		c.ApplyBackoff(crawlSession.Hostname)

		if crawlSession.Ctx.Err() != nil {
			continue
		}

		if c.Options.Options.Delay > 0 {
			select {
			case <-crawlSession.Ctx.Done():
				continue
			case <-time.After(time.Duration(c.Options.Options.Delay) * time.Second):
			}
		}

		if c.Options.Options.MaxDomainPages > 0 {
			counter := c.DomainCounter(crawlSession.Hostname)
			if counter.Add(1) > int64(c.Options.Options.MaxDomainPages) {
				continue
			}
		}

		resp, err := doRequest(crawlSession, req)

		if resp != nil && common.IsThrottled(resp.StatusCode) {
			c.RecordThrottle(crawlSession.Hostname, resp.StatusCode)
		} else if resp != nil {
			c.RecordSuccess(crawlSession.Hostname)
		}

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
	} else if options.Options.SystemChromePath != "" {
		chromeLauncher.Bin(options.Options.SystemChromePath)
	} else if !options.Options.ShowBrowser && chromeshell.Supported() {
		// Prefer chrome-headless-shell on linux/amd64 for headless crawls; skip
		// when headed since the shell binary cannot show a UI.
		if shellPath, err := chromeshell.Ensure(); err == nil {
			chromeLauncher.Bin(shellPath)
		}
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
		// Same implicit-bypass problem as the browser-context path above: without
		// this, a launched Chrome ignores the proxy for loopback targets.
		chromeLauncher.Set("proxy-bypass-list", proxyBypassList(options.Options.Proxy))
	}

	for k, v := range options.Options.ParseHeadlessOptionalArguments() {
		chromeLauncher.Set(flags.Flag(k), v)
	}

	return chromeLauncher, nil
}
