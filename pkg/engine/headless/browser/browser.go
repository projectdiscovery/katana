package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	rodutils "github.com/go-rod/rod/lib/utils"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/js"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/rs/xid"
)

// Launcher is a high level controller to launch browsers
// and do the execution on them.
type Launcher struct {
	browserPool rod.Pool[BrowserPage]

	userDataDir string
	opts        LauncherOptions
}

// LauncherOptions contains options for the launcher
type LauncherOptions struct {
	ChromiumPath   string
	MaxBrowsers    int
	PageMaxTimeout time.Duration
	ShowBrowser    bool
	Proxy          string
	SlowMotion     bool
	ChromeUser     *user.User // optional chrome user to use

	ScopeValidator  ScopeValidator
	RequestCallback func(*output.Result)
}

type ScopeValidator func(string) bool

// NewLauncher returns a new launcher instance
func NewLauncher(opts LauncherOptions) (*Launcher, error) {
	l := &Launcher{
		opts:        opts,
		browserPool: rod.NewPool[BrowserPage](opts.MaxBrowsers),
	}
	return l, nil
}

func (l *Launcher) ScopeValidator() ScopeValidator {
	return l.opts.ScopeValidator
}

func (l *Launcher) launchBrowser() (*rod.Browser, error) {
	chromeLauncher := launcher.New().
		Leakless(true).
		Set("disable-gpu", "true").
		Set("ignore-certificate-errors", "true").
		Set("disable-crash-reporter", "true").
		Set("disable-notifications", "true").
		Set("hide-scrollbars", "true").
		Set("window-size", fmt.Sprintf("%d,%d", 1080, 1920)).
		Set("mute-audio", "true").
		Set("incognito", "true").
		// Set("proxy-server", opts.Proxy).
		Delete("use-mock-keychain").
		Delete("disable-ipc-flooding-protection"). // somehow this causes loops
		Headless(true)

	for _, flag := range headlessFlags {
		splitted := strings.TrimPrefix(flag, "--")
		values := strings.Split(splitted, "=")
		if len(values) == 2 {
			chromeLauncher = chromeLauncher.Set(flags.Flag(values[0]), strings.Split(values[1], ",")...)
		} else {
			chromeLauncher = chromeLauncher.Set(flags.Flag(splitted), "true")
		}
	}

	if l.opts.ShowBrowser {
		chromeLauncher = chromeLauncher.Headless(false)
	}

	if l.opts.ChromiumPath != "" {
		chromeLauncher = chromeLauncher.Bin(l.opts.ChromiumPath)
	}
	chromeLauncher = chromeLauncher.Logger(os.Stderr)

	if l.opts.ChromeUser != nil {
		tempDir, err := os.MkdirTemp(l.opts.ChromeUser.HomeDir, "chrome-data-*")
		if err != nil {
			return nil, errors.Wrap(err, "could not create temporary chrome data directory")
		}
		uid, _ := strconv.Atoi(l.opts.ChromeUser.Uid)
		gid, _ := strconv.Atoi(l.opts.ChromeUser.Gid)

		// Sets proper ownership of the Chrome data directory
		if err := os.Chown(tempDir, uid, gid); err != nil {
			return nil, errors.Wrap(err, "could not change ownership of chrome data directory")
		}
		chromeLauncher = chromeLauncher.Set("user-data-dir", tempDir)
		l.userDataDir = tempDir
	}

	launcherURL, err := chromeLauncher.Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().
		ControlURL(launcherURL)

	if l.opts.SlowMotion {
		browser = browser.SlowMotion(1 * time.Second)
	}
	if browserErr := browser.Connect(); browserErr != nil {
		return nil, browserErr
	}

	return browser, nil
}

// Close closes the launcher
func (l *Launcher) Close() {
	l.browserPool.Cleanup(func(b *BrowserPage) {
		b.cancel()
		b.CloseBrowserPage()
	})
	if l.userDataDir != "" {
		_ = os.RemoveAll(l.userDataDir)
	}
}

// BrowserPage is a combination of a browser and a page
type BrowserPage struct {
	*rod.Page
	Browser *rod.Browser
	cancel  context.CancelFunc

	launcher *Launcher
}

// WaitPageLoadHeurisitics waits for the page to load using heuristics
func (b *BrowserPage) WaitPageLoadHeurisitics() error {
	chainedTimeout := b.Timeout(30 * time.Second)

	_ = chainedTimeout.WaitLoad()
	_ = chainedTimeout.WaitIdle(1 * time.Second)
	_ = b.WaitNewStable(500 * time.Millisecond)
	return nil
}

// WaitStable waits until the page is stable for d duration.
func (p *BrowserPage) WaitNewStable(d time.Duration) error {
	var err error

	setErr := sync.Once{}

	rodutils.All(func() {
		e := p.WaitLoad()
		setErr.Do(func() { err = e })
	}, func() {
		p.WaitRequestIdle(d, nil, []string{}, nil)()
	}, func() {
		//	e := p.WaitDOMStable(d, 0)
		//setErr.Do(func() { err = e })
	})()

	return err
}

func (l *Launcher) createBrowserPageFunc() (*BrowserPage, error) {
	browser, err := l.launchBrowser()
	if err != nil {
		return nil, err
	}
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, errors.Wrap(err, "could not create new page")
	}
	page = page.Sleeper(func() rodutils.Sleeper {
		return backoffCountSleeper(30*time.Millisecond, 1*time.Second, 5, func(d time.Duration) time.Duration {
			return d * 2
		})
	})
	ctx := page.GetContext()
	cancelCtx, cancel := context.WithCancel(ctx)
	page = page.Context(cancelCtx)

	browserPage := &BrowserPage{
		Page:     page,
		Browser:  browser,
		launcher: l,
		cancel:   cancel,
	}
	browserPage.handlePageDialogBoxes()

	err = js.InitJavascriptEnv(page)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize javascript env")
	}
	return browserPage, nil
}

// GetPageFromPool returns a page from the pool
func (l *Launcher) GetPageFromPool() (*BrowserPage, error) {
	browserPage, err := l.browserPool.Get(l.createBrowserPageFunc)
	if err != nil {
		return nil, err
	}
	return browserPage, nil
}

// backoffCountSleeper returns a sleeper that uses backoff strategy but stops after max attempts.
// It combines the functionality of BackoffSleeper and CountSleeper.
func backoffCountSleeper(initInterval, maxInterval time.Duration, maxAttempts int, algorithm func(time.Duration) time.Duration) rodutils.Sleeper {
	backoff := rodutils.BackoffSleeper(initInterval, maxInterval, algorithm)
	count := rodutils.CountSleeper(maxAttempts)

	return rodutils.EachSleepers(backoff, count)
}

func (b *BrowserPage) handlePageDialogBoxes() error {
	err := proto.FetchEnable{
		Patterns: []*proto.FetchRequestPattern{
			{
				URLPattern:   "*",
				RequestStage: proto.FetchRequestStageResponse,
			},
		},
	}.Call(b.Page)
	if err != nil {
		return errors.Wrap(err, "could not enable fetch domain")
	}

	// Handle all the javascript dialogs and accept them
	// with optional text to ensure it doesn't block screenshots.
	go b.Page.EachEvent(
		func(e *proto.PageJavascriptDialogOpening) {
			_ = proto.PageHandleJavaScriptDialog{
				Accept:     true,
				PromptText: xid.New().String(),
			}.Call(b.Page)
		},

		func(e *proto.FetchRequestPaused) {
			if e.ResponseStatusCode == nil || e.ResponseErrorReason != "" || *e.ResponseStatusCode >= 301 && *e.ResponseStatusCode <= 308 {
				fetchContinueRequest(b.Page, e)
				return
			}
			body, err := fetchGetResponseBody(b.Page, e)
			if err != nil {
				return
			}
			_ = fetchContinueRequest(b.Page, e)

			httpreq, err := netHTTPRequestFromProto(e.Request)
			if err != nil {
				return
			}

			rawBytesRequest, _ := httputil.DumpRequestOut(httpreq, true)

			req := navigation.Request{
				Method:  httpreq.Method,
				URL:     httpreq.URL.String(),
				Body:    e.Request.PostData,
				Headers: utils.FlattenHeaders(httpreq.Header),
				Raw:     string(rawBytesRequest),
			}

			httpresp := netHTTPResponseFromProto(e, body)

			rawBytesResponse, _ := httputil.DumpResponse(httpresp, true)

			resp := &navigation.Response{
				Body:          string(body),
				StatusCode:    httpresp.StatusCode,
				Headers:       utils.FlattenHeaders(httpresp.Header),
				Raw:           string(rawBytesResponse),
				ContentLength: httpresp.ContentLength,
			}
			b.launcher.opts.RequestCallback(&output.Result{
				Timestamp: time.Now(),
				Request:   &req,
				Response:  resp,
			})
		},
	)()
	return nil
}

func fetchContinueRequest(page *rod.Page, e *proto.FetchRequestPaused) error {
	return proto.FetchContinueRequest{
		RequestID: e.RequestID,
	}.Call(page)
}

// fetchGetResponseBody get request body.
func fetchGetResponseBody(page *rod.Page, e *proto.FetchRequestPaused) ([]byte, error) {
	m := proto.FetchGetResponseBody{
		RequestID: e.RequestID,
	}
	r, err := m.Call(page)
	if err != nil {
		return nil, err
	}

	if !r.Base64Encoded {
		return []byte(r.Body), nil
	}

	bs, err := base64.StdEncoding.DecodeString(r.Body)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

func netHTTPRequestFromProto(e *proto.NetworkRequest) (*http.Request, error) {
	req, err := http.NewRequest(e.Method, e.URL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not create new request")
	}
	for k, v := range e.Headers {
		req.Header.Set(k, v.Str())
	}
	if e.PostData != "" {
		req.Body = io.NopCloser(strings.NewReader(e.PostData))
		req.ContentLength = int64(len(e.PostData))
	}
	return req, nil
}

func netHTTPResponseFromProto(e *proto.FetchRequestPaused, body []byte) *http.Response {
	httpresp := &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		StatusCode:    *e.ResponseStatusCode,
		Status:        e.ResponseStatusText,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	for _, header := range e.ResponseHeaders {
		httpresp.Header.Set(header.Name, header.Value)
	}
	return httpresp
}

func (l *Launcher) PutBrowserToPool(browser *BrowserPage) {
	// If the browser is not connected, close it
	if !isBrowserConnected(browser.Browser) {
		browser.cancel()
		browser.CloseBrowserPage()
		return
	}

	pages, err := browser.Browser.Pages()
	if err != nil {
		browser.cancel()
		browser.CloseBrowserPage()
		return
	}

	// Close all pages except the current one
	currentPageID := browser.Page.TargetID
	for _, page := range pages {
		if page.TargetID != currentPageID {
			_ = page.Close()
		}
	}
	l.browserPool.Put(browser)
}

func isBrowserConnected(browser *rod.Browser) bool {
	getVersionResult, err := proto.BrowserGetVersion{}.Call(browser)
	if err != nil {
		return false
	}
	if getVersionResult == nil || getVersionResult.Product == "" {
		return false
	}
	return true
}

func (b *BrowserPage) CloseBrowserPage() {
	b.Page.Close()
	b.Browser.Close()
}

// taken from playwright
var headlessFlags = []string{
	"--disable-field-trial-config", // https://source.chromium.org/chromium/chromium/src/+/main:testing/variations/README.md
	"--disable-background-networking",
	"--enable-features=NetworkService,NetworkServiceInProcess",
	"--disable-background-timer-throttling",
	"--disable-backgrounding-occluded-windows",
	"--disable-back-forward-cache", // Avoids surprises like main request not being intercepted during page.goBack().
	"--disable-breakpad",
	"--disable-client-side-phishing-detection",
	"--disable-component-extensions-with-background-pages",
	"--disable-component-update", // Avoids unneeded network activity after startup.
	"--no-default-browser-check",
	"--disable-default-apps",
	"--disable-dev-shm-usage",
	"--disable-extensions",
	// AvoidUnnecessaryBeforeUnloadCheckSync - https://github.com/microsoft/playwright/issues/14047
	// Translate - https://github.com/microsoft/playwright/issues/16126
	// HttpsUpgrades - https://github.com/microsoft/playwright/pull/27605
	// PaintHolding - https://github.com/microsoft/playwright/issues/28023
	"--disable-features=ImprovedCookieControls,LazyFrameLoading,GlobalMediaControls,DestroyProfileOnBrowserClose,MediaRouter,DialMediaRouteProvider,AcceptCHFrame,AutoExpandDetailsElement,CertificateTransparencyComponentUpdater,AvoidUnnecessaryBeforeUnloadCheckSync,Translate,HttpsUpgrades,PaintHolding",
	"--allow-pre-commit-input",
	"--disable-hang-monitor",
	"--disable-popup-blocking",
	"--disable-prompt-on-repost",
	"--disable-renderer-backgrounding",
	"--force-color-profile=srgb",
	"--metrics-recording-only",
	"--no-first-run",
	"--enable-automation",
	"--password-store=basic",
	"--use-mock-keychain",
	// See https://chromium-review.googlesource.com/c/chromium/src/+/2436773
	"--no-service-autorun",
	"--export-tagged-pdf",
	// https://chromium-review.googlesource.com/c/chromium/src/+/4853540
	"--disable-search-engine-choice-screen",
	// https://issues.chromium.org/41491762
	"--unsafely-disable-devtools-self-xss-warnings",
}
