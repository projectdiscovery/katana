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
	"github.com/remeh/sizedwaitgroup"
)

// browserAgent is one isolated Chrome instance used as a hybrid crawl worker.
type browserAgent struct {
	browser        *rod.Browser
	chromeLauncher *launcher.Launcher // nil when attached via ChromeWSUrl
	cdpWS          *cdp.WebSocket
	tempDir        string
	ownsTempDir    bool
}

// Crawler is a standard crawler instance
type Crawler struct {
	*common.Shared

	agents []*browserAgent
	// browser is the first agent, kept for callers/tests that expect a primary handle.
	browser *rod.Browser
	// hooks holds the optional per-page lifecycle callbacks; see SetHooks.
	hooks Hooks
	// cookies shares cookies between agents. It is nil with one agent.
	cookies *cookieStore
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

func ownsBrowser(browser *rod.Browser, chromeLauncher *launcher.Launcher) bool {
	return chromeLauncher != nil || browser.BrowserContextID != ""
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	agentsCount := hybridBrowserAgents(options.Options)
	if agentsCount > 1 {
		gologger.Info().Msgf("hybrid: using %d browser agents (max %d)", agentsCount, maxHybridBrowserAgents)
	}

	agents := make([]*browserAgent, 0, agentsCount)
	cleanup := func() {
		for _, a := range agents {
			_ = a.close()
		}
	}

	for i := 0; i < agentsCount; i++ {
		agent, err := launchBrowserAgent(options, i)
		if err != nil {
			cleanup()
			return nil, err
		}
		agents = append(agents, agent)
	}

	shared, err := common.NewShared(options)
	if err != nil {
		cleanup()
		return nil, errkit.Wrap(err, "hybrid")
	}

	crawler := &Crawler{
		Shared:  shared,
		agents:  agents,
		browser: agents[0].browser,
	}
	if len(agents) > 1 {
		crawler.cookies = newCookieStore()
	}
	return crawler, nil
}

func launchBrowserAgent(options *types.CrawlerOptions, index int) (*browserAgent, error) {
	var dataStore string
	var ownsTempDir bool
	var err error

	if options.Options.ChromeDataDir != "" {
		dataStore = options.Options.ChromeDataDir
	} else {
		dataStore, err = os.MkdirTemp("", fmt.Sprintf("katana-%d-*", index))
		if err != nil {
			return nil, errkit.Wrap(err, "hybrid: could not create temporary directory")
		}
		ownsTempDir = true
	}

	var launcherURL string
	var chromeLauncher *launcher.Launcher

	if options.Options.ChromeWSUrl != "" {
		launcherURL = options.Options.ChromeWSUrl
	} else {
		chromeLauncher, err = buildChromeLauncher(options, dataStore)
		if err != nil {
			if ownsTempDir {
				_ = os.RemoveAll(dataStore)
			}
			return nil, err
		}
		launcherURL, err = chromeLauncher.Launch()
		if err != nil {
			if ownsTempDir {
				_ = os.RemoveAll(dataStore)
			}
			return nil, err
		}
	}

	// Construct the CDP client here rather than using rod.New().ControlURL(...)
	// so the websocket handle survives into close(). rod hides it in an
	// unexported field, and without it the connection can never be closed --
	// see close() for why that leaks. StartWithURL, which Connect calls, is
	// exactly this.
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 30*time.Second)
	cdpWS := &cdp.WebSocket{}
	wsErr := cdpWS.Connect(dialCtx, launcherURL, nil)
	dialCancel()
	if wsErr != nil {
		if chromeLauncher != nil {
			chromeLauncher.Kill()
		}
		if ownsTempDir {
			_ = os.RemoveAll(dataStore)
		}
		return nil, errkit.Wrap(wsErr, fmt.Sprintf("hybrid: failed to connect to chrome instance at %s", launcherURL))
	}
	browser := rod.New().Client(cdp.New().Start(cdpWS))
	if browserErr := browser.Connect(); browserErr != nil {
		_ = cdpWS.Close()
		if chromeLauncher != nil {
			chromeLauncher.Kill()
		}
		if ownsTempDir {
			_ = os.RemoveAll(dataStore)
		}
		return nil, errkit.Wrap(browserErr, fmt.Sprintf("hybrid: failed to connect to chrome instance at %s", launcherURL))
	}

	owned := false
	defer func() {
		if owned {
			return
		}
		if ownsBrowser(browser, chromeLauncher) {
			_ = browser.Close()
		}
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
			// "<-loopback>" SUBTRACTS Chrome's implicit proxy bypass; see
			// proxyBypassList. Only applied when a proxy was actually requested.
			ProxyBypassList: proxyBypassList(options.Options.Proxy),
		}.Call(browser)
		if err != nil {
			return nil, errkit.Wrap(err, "hybrid: failed to create incognito browser")
		}
		incognito := *browser
		incognito.BrowserContextID = res.BrowserContextID
		browser = &incognito
	}

	owned = true
	return &browserAgent{
		browser:        browser,
		chromeLauncher: chromeLauncher,
		cdpWS:          cdpWS,
		tempDir:        dataStore,
		ownsTempDir:    ownsTempDir,
	}, nil
}

func (a *browserAgent) close() error {
	if a == nil {
		return nil
	}
	if a.browser != nil && ownsBrowser(a.browser, a.chromeLauncher) {
		_ = a.browser.Close()
	}
	if a.cdpWS != nil {
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
		_ = a.cdpWS.Close()
	}
	if a.chromeLauncher != nil {
		a.chromeLauncher.Kill()
	}
	if a.ownsTempDir && a.tempDir != "" {
		if err := os.RemoveAll(a.tempDir); err != nil {
			return err
		}
	}
	return nil
}

// BrowserContextID returns the id of the browser context this crawler created
// in New, or "" if it has no browser. It lets an embedding program dispose the
// context out of band: when a crawl dies without running Close, the context
// otherwise stays resident in a shared, long-lived browser. A shared browser
// (ChromeWSUrl) always runs a single agent, so this is the only context then.
func (c *Crawler) BrowserContextID() string {
	if c.browser == nil {
		return ""
	}
	return string(c.browser.BrowserContextID)
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	var firstErr error
	for _, a := range c.agents {
		if err := a.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
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

// Do executes the crawling loop with one page at a time per browser agent.
// Multiple agents (from -browser-agents) each own an isolated Chrome process so
// CDP work does not contend on a single browser target.
func (c *Crawler) Do(crawlSession *common.CrawlSession, doRequest common.DoRequestFunc) error {
	agents := c.agents
	if len(agents) == 0 {
		return errkit.New("hybrid: no browser agents available")
	}

	browserCh := make(chan *rod.Browser, len(agents))
	for _, a := range agents {
		browserCh <- a.browser
	}

	wg := sizedwaitgroup.New(len(agents))
	c.consume(crawlSession, doRequest, browserCh, &wg)
	wg.Wait()

	if err := crawlSession.Ctx.Err(); err != nil {
		return err
	}
	return nil
}

// consume pops queued requests until the queue goes idle and hands each one to
// a free browser agent.
func (c *Crawler) consume(crawlSession *common.CrawlSession, doRequest common.DoRequestFunc, browserCh chan *rod.Browser, wg *sizedwaitgroup.SizedWaitGroup) {
	for item := range crawlSession.Queue.PopWithContext(crawlSession.Ctx) {
		if ctxErr := crawlSession.Ctx.Err(); ctxErr != nil {
			break
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

		done := crawlSession.Queue.Track()
		wg.Add()
		go func(req *navigation.Request, inScope bool) {
			defer wg.Done()
			defer done()

			select {
			case <-crawlSession.Ctx.Done():
				return
			case browser := <-browserCh:
				defer func() { browserCh <- browser }()

				session := *crawlSession
				session.Browser = browser
				c.visit(&session, req, inScope, doRequest)
			}
		}(req, inScope)
	}
}

// visit requests one page and enqueues its links.
func (c *Crawler) visit(crawlSession *common.CrawlSession, req *navigation.Request, inScope bool, doRequest common.DoRequestFunc) {
	// Race Take() against the session context so the loop doesn't
	// block on a limiter tick when the crawl has been cancelled.
	//
	// Note: when the session is cancelled mid-Take, this inner
	// goroutine outlives the loop iteration and stays blocked on
	// the limiter until the next tick or until RateLimit.Stop() is
	// called by CrawlerOptions.Close(). The leak is bounded by
	// Close() and acceptable.
	if crawlSession.Ctx.Err() != nil {
		return
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
		return
	case <-takeDone:
	}
	c.ApplyBackoff(crawlSession.Hostname)

	if crawlSession.Ctx.Err() != nil {
		return
	}

	if c.Options.Options.Delay > 0 {
		select {
		case <-crawlSession.Ctx.Done():
			return
		case <-time.After(time.Duration(c.Options.Options.Delay) * time.Second):
		}
	}

	if c.Options.Options.MaxDomainPages > 0 {
		counter := c.DomainCounter(crawlSession.Hostname)
		if counter.Add(1) > int64(c.Options.Options.MaxDomainPages) {
			return
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
		return
	}
	if resp == nil || resp.Resp == nil || resp.Reader == nil {
		return
	}
	if c.Options.Options.DisableRedirects && resp.IsRedirect() {
		return
	}

	navigationRequests := c.Options.Parser.ParseResponse(resp)
	c.Enqueue(crawlSession.Queue, navigationRequests...)
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

	if options.Options.Proxy != "" {
		proxyURL, err := urlutil.Parse(options.Options.Proxy)
		if err != nil {
			return nil, err
		}
		chromeLauncher.Set("proxy-server", proxyURL.String())
		// Same implicit-bypass problem as the browser-context path above: without
		// this, a launched Chrome ignores the proxy for loopback targets.
		// Applied whenever a proxy is set, including -hh (Headless is mutually
		// exclusive with HeadlessHybrid, so the previous Headless guard never ran).
		chromeLauncher.Set("proxy-bypass-list", proxyBypassList(options.Options.Proxy))
	}

	for k, v := range options.Options.ParseHeadlessOptionalArguments() {
		chromeLauncher.Set(flags.Flag(k), v)
	}

	return chromeLauncher, nil
}
