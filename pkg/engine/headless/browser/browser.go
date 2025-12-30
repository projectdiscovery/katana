package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
	rodutils "github.com/go-rod/rod/lib/utils"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser/cookie"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser/stealth"
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

	opts LauncherOptions
}

// LauncherOptions contains options for the launcher
type LauncherOptions struct {
	ChromiumPath        string
	MaxBrowsers         int
	PageMaxTimeout      time.Duration
	ShowBrowser         bool
	NoSandbox           bool
	Proxy               string
	SlowMotion          bool
	Trace               bool
	CookieConsentBypass bool
	ChromeUser          *user.User // optional chrome user to use

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

func (l *Launcher) launchBrowserWithDataDir(userDataDir string) (*rod.Browser, error) {
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
		Delete("use-mock-keychain").
		Delete("disable-ipc-flooding-protection").
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

	if l.opts.NoSandbox {
		chromeLauncher = chromeLauncher.NoSandbox(true)
	}

	if l.opts.ShowBrowser {
		chromeLauncher = chromeLauncher.Headless(false)
	}

	if l.opts.ChromiumPath != "" {
		chromeLauncher = chromeLauncher.Bin(l.opts.ChromiumPath)
	}

	if userDataDir != "" {
		chromeLauncher = chromeLauncher.UserDataDir(userDataDir)
	}

	launcherURL, err := chromeLauncher.Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().
		ControlURL(launcherURL)
	if l.opts.Trace {
		browser = browser.Trace(true)
	}

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
	close(l.browserPool)
}

// BrowserPage is a combination of a browser and a page
type BrowserPage struct {
	*rod.Page
	Browser     *rod.Browser
	cancel      context.CancelFunc
	userDataDir string

	launcher *Launcher
}

// WaitOptions controls how WaitPageLoadHeurisitics determines navigation completion.
// All durations are conservative defaults and can be tuned later via package-level variables
// or future setter methods (kept simple here to avoid breaking public API).
type WaitOptions struct {
	URLPollInterval time.Duration // interval between successive URL polls
	URLPollTimeout  time.Duration // how long to keep polling before giving up on URL change
	PostChangeWait  time.Duration // small grace period after URL change for late requests
	IdleWait        time.Duration // network-idle window when no URL change happened
	DOMStableWait   time.Duration // DOM-stable window (used after idle)
	MaxTimeout      time.Duration // absolute upper bound for all waiting
}

// defaultWaitOptions are derived from empirical measurements on modern SPA pages.
var defaultWaitOptions = WaitOptions{
	URLPollInterval: 100 * time.Millisecond,
	URLPollTimeout:  2 * time.Second,
	PostChangeWait:  300 * time.Millisecond,
	IdleWait:        1 * time.Second,
	DOMStableWait:   1 * time.Second,
	MaxTimeout:      15 * time.Second,
}

// WaitPageLoadHeurisitics waits for the page to load using multiple heuristics.
// Strategy order:
//  1. Wait for initial load event (covers classic navigation & first paint).
//  2. Poll for a URL change – the strongest signal on SPAs with client-side routing.
//  3. If URL changes, wait a short grace period + network-idle window.
//  4. If URL doesn't change, fall back to network-idle + DOM-stable windows.
//
// This keeps fast pages fast while still succeeding on noisy, long-running SPAs.
func (b *BrowserPage) WaitPageLoadHeurisitics() error {
	opts := defaultWaitOptions

	chained := b.Timeout(opts.MaxTimeout)

	// 1. Wait for the basic load event (DOMContentLoaded / load).
	_ = chained.WaitLoad()

	// 2. Capture the current URL so we can detect route changes.
	urlVal, _ := b.Eval("() => window.location.href")
	startURL := ""
	if urlVal != nil {
		startURL = urlVal.Value.Str()
	}

	// 3. Poll for a different URL for up to URLPollTimeout.
	urlChanged := false
	if startURL != "" {
		pollCount := int(opts.URLPollTimeout / opts.URLPollInterval)
		for i := 0; i < pollCount; i++ {
			time.Sleep(opts.URLPollInterval)
			cur, err := b.Eval("() => window.location.href")
			if err == nil && cur != nil && cur.Value.Str() != startURL {
				urlChanged = true
				break
			}
		}
	}

	if urlChanged {
		// 4a. URL changed – short grace period then network idle & done.
		_ = chained.WaitIdle(opts.PostChangeWait)
		return nil
	}

	// 4b. URL didn't change – fall back to broader heuristics.
	_ = chained.WaitIdle(opts.IdleWait)
	_ = b.WaitNewStable(opts.DOMStableWait)

	return nil
}

// WaitPageLoadHeuristicsFallback provides the enhanced timeouts for complex navigation
func (b *BrowserPage) WaitPageLoadHeuristicsFallback() error {
	chainedTimeout := b.Timeout(20 * time.Second)

	_ = chainedTimeout.WaitLoad()
	_ = chainedTimeout.WaitIdle(4 * time.Second)
	_ = b.WaitNewStable(2 * time.Second)

	return nil
}

// WaitStable waits until the page is stable for d duration.
func (p *BrowserPage) WaitNewStable(d time.Duration) error {
	// Enforce an upper-bound on how long we will wait for the page to become
	// stable. We simply reuse the heuristic window (d) and give the combined
	// operation 2× that duration. This guarantees that callers will be
	// released after a finite time instead of blocking forever when a page
	// keeps a long-lived connection open (analytics beacons, WebSockets, etc.).

	chained := p.Timeout(2 * d)

	var err error
	setErr := sync.Once{}

	rodutils.All(func() {
		e := chained.WaitLoad()
		setErr.Do(func() { err = e })
	}, func() {
		chained.WaitRequestIdle(d, nil, []string{}, nil)()
	}, func() {
		e := chained.WaitDOMStable(d, 0)
		setErr.Do(func() { err = e })
	})()

	return err
}

func (l *Launcher) createBrowserPageFunc() (*BrowserPage, error) {
	// Create unique temp userDataDir for this browser instance
	var tempDir string
	shouldCleanup := true

	// Deferred cleanup function that will be set after tempDir creation
	defer func() {
		if shouldCleanup && tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if l.opts.ChromeUser != nil {
		var err error
		tempDir, err = os.MkdirTemp(l.opts.ChromeUser.HomeDir, "chrome-data-*")
		if err != nil {
			return nil, errors.Wrap(err, "could not create temporary chrome data directory")
		}

		uid, err := strconv.Atoi(l.opts.ChromeUser.Uid)
		if err != nil {
			return nil, errors.Wrap(err, "invalid user ID")
		}
		gid, err := strconv.Atoi(l.opts.ChromeUser.Gid)
		if err != nil {
			return nil, errors.Wrap(err, "invalid group ID")
		}
		if err := os.Chown(tempDir, uid, gid); err != nil {
			return nil, errors.Wrap(err, "could not change ownership of chrome data directory")
		}
	} else {
		var err error
		tempDir, err = os.MkdirTemp("", "katana-chrome-data-*")
		if err != nil {
			return nil, errors.Wrap(err, "could not create temporary chrome data directory")
		}
	}

	browser, err := l.launchBrowserWithDataDir(tempDir)
	if err != nil {
		return nil, err
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, errors.Wrap(err, "could not create new page")
	}

	successfulPageCreation := false
	defer func() {
		if !successfulPageCreation {
			_ = page.Close()
			_ = browser.Close()
		}
	}()

	page = page.Sleeper(func() rodutils.Sleeper {
		return backoffCountSleeper(100*time.Millisecond, 1*time.Second, 3, func(d time.Duration) time.Duration {
			return d * 1
		})
	})
	ctx := page.GetContext()
	cancelCtx, cancel := context.WithCancel(ctx)
	page = page.Context(cancelCtx)

	browserPage := &BrowserPage{
		Page:        page,
		Browser:     browser,
		launcher:    l,
		cancel:      cancel,
		userDataDir: tempDir,
	}
	if err := browserPage.handlePageDialogBoxes(); err != nil {
		return nil, err
	}

	// Add stealth evasion JS
	_, err = page.EvalOnNewDocument(stealth.JS)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize stealth")
	}
	err = js.InitJavascriptEnv(page)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize javascript env")
	}

	// Success - cancel the deferred cleanup
	successfulPageCreation = true
	shouldCleanup = false
	return browserPage, nil
}

// GetPageFromPool returns a page from the pool
func (l *Launcher) GetPageFromPool() (*BrowserPage, error) {
	browserPage, err := l.browserPool.Get(l.createBrowserPageFunc)
	if err != nil {
		return nil, err
	}
	// TODO: should we check if the browser is alive because sometimes it
	// might die?
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

	go b.EachEvent(
		func(e *proto.PageJavascriptDialogOpening) {
			_ = proto.PageHandleJavaScriptDialog{
				Accept:     true,
				PromptText: xid.New().String(),
			}.Call(b.Page)
		},

		func(e *proto.FetchRequestPaused) {
			if b.launcher.opts.CookieConsentBypass {
				// Check if request should be blocked by cookie consent rules
				var originStr string
				if origin, ok := e.Request.Headers["Origin"]; ok {
					originStr = origin.Str()
				}
				if cookie.ShouldBlockRequest(e.Request.URL, e.ResourceType, originStr) {
					_ = proto.FetchFailRequest{
						RequestID:   e.RequestID,
						ErrorReason: proto.NetworkErrorReasonBlockedByClient,
					}.Call(b.Page)
					return
				}
			}

			if e.ResponseStatusCode == nil || e.ResponseErrorReason != "" || (*e.ResponseStatusCode >= 301 && *e.ResponseStatusCode <= 308) {
				if err := fetchContinueRequest(b.Page, e); err != nil {
					slog.Warn("fetchContinueRequest failed", "error", err)
				}
				return
			}
			body, err := fetchGetResponseBody(b.Page, e)
			if err != nil {
				// Continue the request even if we can't get the body
				if err := fetchContinueRequest(b.Page, e); err != nil {
					slog.Warn("fetchContinueRequest failed", "error", err)
				}
				return
			}
			if err := fetchContinueRequest(b.Page, e); err != nil {
				slog.Warn("fetchContinueRequest failed", "error", err)
			}

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
			httpresp.Request = httpreq

			rawBytesResponse, _ := httputil.DumpResponse(httpresp, true)

			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
			if err != nil {
				slog.Warn("could not parse response body", "error", err)
			}
			resp := &navigation.Response{
				Body:          string(body),
				StatusCode:    httpresp.StatusCode,
				Headers:       utils.FlattenHeaders(httpresp.Header),
				Raw:           string(rawBytesResponse),
				ContentLength: httpresp.ContentLength,
				Resp:          httpresp,
				Reader:        doc,
			}
			if b.launcher.opts.RequestCallback != nil {
				b.launcher.opts.RequestCallback(&output.Result{
					Timestamp: time.Now(),
					Request:   &req,
					Response:  resp,
				})
			}
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
	// Discard pages that hit a deadline or were cancelled to avoid immediately
	// returning a poisoned page that will fail every subsequent call.
	if cerr := browser.Page.GetContext().Err(); cerr != nil {
		browser.cancel()
		browser.CloseBrowserPage()
		return
	}
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

	currentPageID := browser.TargetID
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
	_ = b.Close()
	_ = b.Browser.Close()
	if b.userDataDir != "" {
		_ = os.RemoveAll(b.userDataDir)
	}
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
