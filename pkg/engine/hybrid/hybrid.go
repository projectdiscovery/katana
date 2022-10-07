package hybrid

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/stringsutil"
	"github.com/remeh/sizedwaitgroup"
	ps "github.com/shirou/gopsutil/v3/process"
	"go.uber.org/multierr"
)

// Crawler is a standard crawler instance
type Crawler struct {
	headers      map[string]string
	options      *types.CrawlerOptions
	httpclient   *retryablehttp.Client
	browser      *rod.Browser
	dialer       *fastdialer.Dialer
	previousPIDs map[int32]struct{} // track already running PIDs
	tempDir      string
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	httpclient, dialer, err := common.BuildClient(options.Options)
	if err != nil {
		return nil, errors.Wrap(err, "could not create http client")
	}

	dataStore, err := os.MkdirTemp("", "katana-*")
	if err != nil {
		return nil, errors.Wrap(err, "could not create temporary directory")
	}

	previousPIDs := findChromeProcesses()

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
			return nil, errors.New("the chrome browser is not installed")
		}
	}

	if options.Options.ShowBrowser {
		chromeLauncher = chromeLauncher.Headless(false)
	} else {
		chromeLauncher = chromeLauncher.Headless(true)
	}

	launcherURL, err := chromeLauncher.Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().ControlURL(launcherURL)
	if browserErr := browser.Connect(); browserErr != nil {
		return nil, browserErr
	}

	crawler := &Crawler{
		headers:      options.Options.ParseCustomHeaders(),
		options:      options,
		dialer:       dialer,
		httpclient:   httpclient,
		browser:      browser,
		previousPIDs: previousPIDs,
		tempDir:      dataStore,
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	c.dialer.Close()

	if err := c.browser.Close(); err != nil {
		return err
	}

	if err := os.RemoveAll(c.tempDir); err != nil {
		return err
	}

	return c.killChromeProcesses()
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	ctx, cancel := context.WithCancel(context.Background())
	if c.options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	parsed, err := url.Parse(rootURL)
	if err != nil {
		return errors.Wrap(err, "could not parse root URL")
	}
	hostname := parsed.Hostname()

	queue := queue.New(c.options.Options.Strategy)
	queue.Push(navigation.Request{Method: http.MethodGet, URL: rootURL, Depth: 0}, 0)
	parseResponseCallback := c.makeParseResponseCallback(queue)

	// for each seed URL we use an incognito isolated session
	incognitoBrowser, err := c.browser.Incognito()
	if err != nil {
		return err
	}

	wg := sizedwaitgroup.New(c.options.Options.Concurrency)
	running := int32(0)
	for {
		// Quit the crawling for zero items or context timeout
		if !(atomic.LoadInt32(&running) > 0) && (queue.Len() == 0 || ctx.Err() != nil) {
			break
		}
		item := queue.Pop()
		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}
		if !utils.IsURL(req.URL) {
			continue
		}
		wg.Add()
		atomic.AddInt32(&running, 1)

		go func() {
			defer wg.Done()
			defer atomic.AddInt32(&running, -1)

			c.options.RateLimit.Take()

			// Delay if the user has asked for it
			if c.options.Options.Delay > 0 {
				time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
			}
			resp, err := c.navigateRequest(ctx, queue, parseResponseCallback, incognitoBrowser, req, hostname)
			if err != nil {
				gologger.Warning().Msgf("Could not request seed URL: %s\n", err)
				return
			}
			if resp.Resp == nil || resp.Reader == nil {
				return
			}
			// process the dom-rendered response
			parser.ParseResponse(*resp, parseResponseCallback)
		}()
	}
	wg.Wait()

	return nil
}

// makeParseResponseCallback returns a parse response function callback
func (c *Crawler) makeParseResponseCallback(queue *queue.VarietyQueue) func(nr navigation.Request) {
	return func(nr navigation.Request) {
		if !utils.IsURL(nr.URL) {
			return
		}
		// Ignore blank URL items and only work on unique items
		if nr.URL == "" || !c.options.UniqueFilter.Unique(nr.RequestURL()) {
			return
		}

		// Write the found result to output
		result := &output.Result{
			Timestamp: time.Now(),
			Body:      nr.Body,
			URL:       nr.URL,
			Source:    nr.Source,
			Tag:       nr.Tag,
			Attribute: nr.Attribute,
		}
		if nr.Method != http.MethodGet {
			result.Method = nr.Method
		}
		_ = c.options.OutputWriter.Write(result)

		// Do not add to crawl queue if max items are reached
		if nr.Depth >= c.options.Options.MaxDepth {
			return
		}
		queue.Push(nr, nr.Depth)
	}
}

// routingHandler intercepts all asyncronous http requests
func (c *Crawler) makeRoutingHandler(queue *queue.VarietyQueue, depth int, rootHostname string, parseRequestCallback func(nr navigation.Request)) func(ctx *rod.Hijack) {
	return func(ctx *rod.Hijack) {
		reqURL := ctx.Request.URL()
		if !utils.IsURL(reqURL.String()) {
			return
		}

		// here we can process raw request/response in one pass
		err := ctx.LoadResponse(c.httpclient.HTTPClient, true)
		if err != nil {
			gologger.Warning().Msgf("\"%s\" on load response: %s\n", reqURL, err)
		}

		body := ctx.Response.Body()

		httpresp := &http.Response{
			StatusCode: ctx.Response.Payload().ResponseCode,
			Status:     ctx.Response.Payload().ResponsePhrase,
			Header:     ctx.Response.Headers(),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    ctx.Request.Req(),
		}

		bodyReader, _ := goquery.NewDocumentFromReader(strings.NewReader(body))
		resp := navigation.Response{
			Resp:         httpresp,
			Body:         []byte(body),
			Reader:       bodyReader,
			Options:      c.options,
			Depth:        depth,
			RootHostname: rootHostname,
		}

		// process the raw response
		parser.ParseResponse(resp, parseRequestCallback)
	}
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
