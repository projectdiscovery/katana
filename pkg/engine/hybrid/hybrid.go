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
	"github.com/remeh/sizedwaitgroup"
)

// browserAgent is one isolated Chrome instance used as a hybrid crawl worker.
type browserAgent struct {
	browser        *rod.Browser
	chromeLauncher *launcher.Launcher // nil when attached via ChromeWSUrl
	tempDir        string
	ownsTempDir    bool
}

// Crawler is a standard crawler instance
type Crawler struct {
	*common.Shared

	agents []*browserAgent
	// browser is the first agent, kept for callers/tests that expect a primary handle.
	browser *rod.Browser
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	agentsCount := hybridBrowserAgents(options.Options)
	if agentsCount > 1 {
		gologger.Info().Msgf("hybrid: using %d browser agents (from -c, max %d)", agentsCount, maxHybridBrowserAgents)
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

	browser := rod.New().ControlURL(launcherURL)
	if browserErr := browser.Connect(); browserErr != nil {
		if chromeLauncher != nil {
			chromeLauncher.Kill()
		}
		if ownsTempDir {
			_ = os.RemoveAll(dataStore)
		}
		return nil, errkit.Wrap(browserErr, fmt.Sprintf("hybrid: failed to connect to chrome instance at %s", launcherURL))
	}

	if !options.Options.HeadlessNoIncognito {
		incognito, err := browser.Incognito()
		if err != nil {
			_ = browser.Close()
			if chromeLauncher != nil {
				chromeLauncher.Kill()
			}
			if ownsTempDir {
				_ = os.RemoveAll(dataStore)
			}
			return nil, errkit.Wrap(err, "hybrid: failed to create incognito browser")
		}
		browser = incognito
	}

	return &browserAgent{
		browser:        browser,
		chromeLauncher: chromeLauncher,
		tempDir:        dataStore,
		ownsTempDir:    ownsTempDir,
	}, nil
}

func (a *browserAgent) close() error {
	if a == nil {
		return nil
	}
	if a.browser != nil {
		_ = a.browser.Close()
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
// Multiple agents (from -c) each own an isolated Chrome process so CDP work
// does not contend on a single browser target.
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

		wg.Add()
		go func(req *navigation.Request, inScope bool) {
			defer wg.Done()

			select {
			case <-crawlSession.Ctx.Done():
				return
			case browser := <-browserCh:
				defer func() { browserCh <- browser }()

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

				session := *crawlSession
				session.Browser = browser

				resp, err := doRequest(&session, req)

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
		}(req, inScope)
	}
	wg.Wait()

	if err := crawlSession.Ctx.Err(); err != nil {
		return err
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
