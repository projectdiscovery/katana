package hybrid

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/utils/errkit"
	mapsutil "github.com/projectdiscovery/utils/maps"
	sliceutil "github.com/projectdiscovery/utils/slice"
	stringsutil "github.com/projectdiscovery/utils/strings"
	urlutil "github.com/projectdiscovery/utils/url"
)

func (c *Crawler) navigateRequest(s *common.CrawlSession, request *navigation.Request) (*navigation.Response, error) {
	depth := request.Depth + 1
	response := &navigation.Response{
		Depth:        depth,
		RootHostname: s.Hostname,
	}

	page, err := s.Browser.Context(s.Ctx).Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: could not create target")
	}
	// Keep the original page reference for cleanup — page.Context() returns a clone,
	// so closing only the clone after timeout would leak the original tab.
	cleanupPage := page
	// sessionPage is bound to the crawl session context so that DOM/HTML
	// operations with fresh timeouts still respect external cancellation.
	sessionPage := page.Context(s.Ctx)
	timeout := time.Duration(c.Options.Options.Timeout) * time.Second
	timeoutCtx, timeoutCancel := context.WithTimeout(s.Ctx, timeout)
	defer timeoutCancel()
	page = sessionPage.Context(timeoutCtx)

	defer func() {
		if err := cleanupPage.Close(); err != nil {
			gologger.Error().Msgf("Error closing page: %v\n", err)
		}
	}()
	c.addHeadersToPage(page)

	if err := runBeforeNavigate(timeoutCtx, c.hooks, sessionPage, request); err != nil {
		return nil, err
	}

	pageRouter := NewHijack(page)
	pageRouter.SetPattern(&proto.FetchRequestPattern{
		URLPattern:   "*",
		RequestStage: proto.FetchRequestStageResponse,
	})

	xhrRequests := []navigation.Request{}
	// handlerPage is cancelled before waiting on the router, so a body read
	// stuck on a streaming response does not hold the visit open
	handlerPage, cancelHandlers := page.WithCancel()
	waitRouter := pageRouter.Start(func(e *proto.FetchRequestPaused) error {
		URL, err := urlutil.Parse(e.Request.URL)
		if err != nil {
			return errkit.Wrap(err, "hybrid: could not parse URL")
		}

		headers := make(map[string][]string)
		for _, h := range e.ResponseHeaders {
			headers[h.Name] = []string{h.Value}
		}
		var (
			statusCode     int
			statucCodeText string
		)
		if e.ResponseStatusCode != nil {
			statusCode = *e.ResponseStatusCode
		}
		if e.ResponseStatusText != "" {
			statucCodeText = e.ResponseStatusText
		} else {
			statucCodeText = http.StatusText(statusCode)
		}
		httpreq, err := http.NewRequestWithContext(s.Ctx, e.Request.Method, URL.String(), strings.NewReader(e.Request.PostData))
		if err != nil {
			return errkit.Wrap(err, "hybrid: could not new request")
		}
		// Note: headers are originally sent using `c.addHeadersToPage` below changes are done so that
		// headers are reflected in request dump
		// Headers, CustomHeaders, and Cookies are present in e.Request.Headers. We need to consider all of them and not only CustomHeaders
		// Otherwise, we will miss headers and output will be inconsistent
		if httpreq != nil {
			for k, v := range e.Request.Headers {
				httpreq.Header.Set(k, v.String())
			}
		}

		requestHeaders := make(map[string][]string)
		for name, value := range e.Request.Headers {
			requestHeaders[name] = []string{value.Str()}
		}

		shouldCapture := func(xhrExtraction bool) bool {
			resourceTypes := []proto.NetworkResourceType{
				proto.NetworkResourceTypeXHR,
				proto.NetworkResourceTypeFetch,
				proto.NetworkResourceTypeScript,
			}

			return xhrExtraction && slices.Contains(resourceTypes, e.ResourceType)
		}

		if shouldCapture(c.Options.Options.XhrExtraction) {
			networkReq := navigation.Request{
				URL:    httpreq.URL.String(),
				Method: httpreq.Method,
				Body:   e.Request.PostData,
			}
			if len(httpreq.Header) > 0 {
				networkReq.Headers = utils.FlattenHeaders(httpreq.Header)
			} else {
				networkReq.Headers = utils.FlattenHeaders(requestHeaders)
			}
			xhrRequests = append(xhrRequests, networkReq)
		}

		// Streaming responses (such as SSE / EventSource or WebSockets) do not
		// complete while the page is alive. Calling FetchGetResponseBody on them
		// blocks CDP until visit timeout. Skip body retrieval and parsing.
		if isStreamingResponse(e.ResourceType, e.ResponseHeaders) {
			return FetchContinueRequest(handlerPage, e)
		}

		// trim trailing /
		normalizedheadlessURL := strings.TrimSuffix(e.Request.URL, "/")
		matchOriginalURL := stringsutil.EqualFoldAny(request.URL, e.Request.URL, normalizedheadlessURL)

		body, _ := FetchGetResponseBody(handlerPage, e)
		parse := c.shouldParse(matchOriginalURL, e.Request.URL, headers, body)
		// A repeated subresource has nothing new to enqueue. Skip the body
		// copies, goquery, and fingerprinting. The main document is still
		// recorded so the crawl does not end with "hybrid: response is nil".
		if !matchOriginalURL && !parse {
			if c.Options.Options.DisableRedirects && statusCode >= 300 && statusCode <= 399 {
				return nil
			}
			return FetchContinueRequest(handlerPage, e)
		}

		httpresp := &http.Response{
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			StatusCode:    statusCode,
			Status:        statucCodeText,
			Header:        headers,
			Body:          io.NopCloser(bytes.NewReader(body)),
			Request:       httpreq,
			ContentLength: int64(len(body)),
		}

		var rawBytesRequest, rawBytesResponse []byte
		if r, err := retryablehttp.FromRequest(httpreq); err == nil {
			rawBytesRequest, _ = r.Dump()
		} else {
			rawBytesRequest, _ = httputil.DumpRequestOut(httpreq, true)
		}
		rawBytesResponse, _ = httputil.DumpResponse(httpresp, true)

		bodyReader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		var technologies map[string]interface{}
		if c.Options.Wappalyzer != nil {
			fingerprints := c.Options.Wappalyzer.Fingerprint(headers, body)
			technologies = make(map[string]interface{}, len(fingerprints))
			for k := range fingerprints {
				technologies[k] = struct{}{}
			}
		}
		resp := &navigation.Response{
			Resp:          httpresp,
			Body:          string(body),
			Reader:        bodyReader,
			Depth:         depth,
			RootHostname:  s.Hostname,
			Technologies:  mapsutil.GetKeys(technologies),
			StatusCode:    statusCode,
			Headers:       utils.FlattenHeaders(headers),
			Raw:           string(rawBytesResponse),
			ContentLength: httpresp.ContentLength,
			KnowledgeBase: c.Options.BuildKnowledgeBase(string(body), httpreq, httpresp),
		}
		response.ContentLength = resp.ContentLength

		// Rejected main docs still record response metadata so the crawl does
		// not end with "hybrid: response is nil"; only parse/enqueue is suppressed.
		if matchOriginalURL {
			request.Raw = string(rawBytesRequest)
			response = resp
		}

		if parse {
			navigationRequests := c.Options.Parser.ParseResponse(resp)
			c.Enqueue(s.Queue, navigationRequests...)
		}

		// do not continue following the request if it's a redirect and redirects are disabled
		if c.Options.Options.DisableRedirects && resp.IsRedirect() {
			return nil
		}
		return FetchContinueRequest(handlerPage, e)
	})
	routerDone := make(chan struct{})
	go func() {
		defer close(routerDone)
		_ = waitRouter()
	}()
	defer func() {
		// Cancel the streaming body read before disabling fetch, then wait
		// until that handler has returned.
		cancelHandlers()
		if err := pageRouter.Stop(); err != nil {
			gologger.Warning().Msgf("%s\n", err)
		}
		// a handler still running could enqueue links after the visit is
		// marked done, when the queue may already have closed
		<-routerDone
	}()

	navigatedURLs := sliceutil.NewSyncSlice[string]()
	navigatedURLs.Append(request.URL)

	pageCtx, cancelPageEvents := page.WithCancel()
	defer cancelPageEvents()

	waitFrameEvents := pageCtx.EachEvent(func(e *proto.PageFrameNavigated) {
		if e.Frame.ParentID == "" {
			frameURL := e.Frame.URL
			if frameURL != "" && frameURL != request.URL {
				navigatedURLs.Append(frameURL)
			}
		}
	})
	go waitFrameEvents()

	// Arm the main-document failure watcher BEFORE navigate (correctness-
	// critical): a watcher installed after Navigate could miss the main
	// request's requestWillBeSent and never learn its requestId. If arming
	// fails, we log and proceed without it — the per-nav timeout still bounds
	// the page exactly as it does today; the fallback simply never fires.
	//
	// Setup CDP calls run on `page` (navigation timeout); the event
	// subscription is parented to the session context so it keeps observing
	// through the DOM/HTML reads, which use fresh timeouts and can outlive the
	// navigation deadline. The deferred stop() ends it when this returns.
	mainDocWatch, watchErr := armMainDocWatcher(page, s.Ctx)
	if watchErr != nil {
		gologger.Warning().Msgf("main-doc failure watcher not armed for %s: %s (navigation still bounded by timeout)", request.URL, watchErr)
	}
	defer func() {
		if mainDocWatch != nil {
			mainDocWatch.stop()
		}
	}()

	// Arm the lifecycle listener before navigate so the event is not missed.
	// Which event we wait for depends on the page-load strategy.
	strategy := c.Options.Options.PageLoadStrategy

	var waitNavigation func()
	switch strategy {
	case "none":
		// no lifecycle wait
	case "domcontentloaded":
		waitNavigation = page.WaitNavigation(proto.PageLifecycleEventNameDOMContentLoaded)
	case "load":
		waitNavigation = page.WaitNavigation(proto.PageLifecycleEventNameLoad)
	case "networkidle":
		waitNavigation = waitAnyLifecycleEvent(page, proto.PageLifecycleEventNameNetworkIdle)
	default:
		waitNavigation = waitAnyLifecycleEvent(page, proto.PageLifecycleEventNameFirstMeaningfulPaint, proto.PageLifecycleEventNameNetworkIdle)
	}

	err = page.Navigate(request.URL)
	if err != nil {
		if c.Options.Options.DisableRedirects && response.IsRedirect() {
			return response, nil
		}
		return nil, errkit.Wrap(err, "hybrid: could not navigate target")
	}

	// Race the lifecycle wait against the main-document failure signal. A
	// healthy page completes through the lifecycle event exactly as today; a
	// page whose main document terminally fails (truncated read-until-close,
	// browser block, abort-after-partial) returns the transport failure now
	// instead of blocking to the per-nav timeout. The watcher ONLY signals
	// failure — success stays owned by the lifecycle wait.
	if waitNavigation != nil {
		navDone := make(chan struct{}, 1)
		go func() {
			waitNavigation()
			navDone <- struct{}{}
		}()
	raceLoop:
		for {
			select {
			case <-navDone:
				// Lifecycle event (or ctx-bound unblock) won: fall through to the
				// existing post-nav path unchanged.
				break raceLoop
			case <-mainDocWatch.notifyChan():
				// A failure verdict finalized. Revalidate against current state:
				// a replacement navigation may have superseded it between the
				// finalizer's check and this receive. Stale -> keep waiting.
				// Current -> fast-fail now, before the stability wait, click
				// step, DOM/HTML reads, and the response acceptance check. The
				// loser (waitNavigation goroutine) is bounded by timeoutCtx/page
				// close via the deferred timeoutCancel.
				if failed, reason := mainDocWatch.failedNow(); failed {
					return nil, fmt.Errorf("%w: %s", errMainDocumentFailed, reason)
				}
			case <-timeoutCtx.Done():
				// Per-nav timeout: fall through to the existing path, which checks
				// timeoutCtx.Done() at its next step exactly as today.
				break raceLoop
			}
		}
	}

	// Post-navigation stability wait, strategy-specific
	switch strategy {
	case "none":
		gologger.Debug().Msgf("page-load-strategy=none: skipping stability wait\n")

	case "domcontentloaded":
		waitTime := time.Duration(c.Options.Options.DOMWaitTime) * time.Second
		gologger.Debug().Msgf("page-load-strategy=domcontentloaded: waiting %s for DOM\n", waitTime)
		if err := mainDocWatch.sleepUnlessFailed(timeoutCtx, waitTime); err != nil {
			return nil, err
		}

	case "load":
		gologger.Debug().Msgf("page-load-strategy=load: basic load wait only\n")
		if err := mainDocWatch.sleepUnlessFailed(timeoutCtx, 500*time.Millisecond); err != nil {
			return nil, err
		}

	default:
		timeStable := time.Duration(c.Options.Options.TimeStable) * time.Second

		if timeout < timeStable {
			gologger.Warning().Msgf("timeout is less than time stable, setting time stable to half of timeout to avoid timeout\n")
			timeStable = timeout / 2
			gologger.Warning().Msgf("setting time stable to %s\n", timeStable)
		}

		if timeStable > 0 {
			if err := page.WaitStable(timeStable); err != nil {
				gologger.Warning().Msgf("could not wait for page to be stable: %s\n", err)
			}
		} else {
			gologger.Debug().Msgf("time-stable is non-positive: skipping stability wait\n")
		}
	}

	// simulate clicks on links with onclick handlers to discover JS redirects
	select {
	case <-timeoutCtx.Done():
		return nil, timeoutCtx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	maxLinks := c.Options.Options.MaxOnclickLinks
	clickableLinks, err := page.Elements("a[onclick]")
	if maxLinks > 0 && err == nil && len(clickableLinks) > 0 {
		linksToProcess := len(clickableLinks)
		if linksToProcess > maxLinks {
			linksToProcess = maxLinks
		}

		gologger.Debug().Msgf("Found %d clickable links with onclick handlers, processing %d", len(clickableLinks), linksToProcess)

		for idx := 0; idx < linksToProcess; idx++ {
			link := clickableLinks[idx]
			beforeURL, err := page.Info()
			if err != nil {
				gologger.Error().Msgf("Could not get page info: %v", err)
				continue
			}
			beforeURLStr := ""
			if beforeURL != nil {
				beforeURLStr = beforeURL.URL
			}

			// try to click the link using rod's Click method
			clickErr := link.Click(proto.InputMouseButtonLeft, 1)
			if clickErr != nil {
				gologger.Debug().Msgf("Could not click link %d: %v", idx, clickErr)
				continue
			}

			gologger.Debug().Msgf("Clicked onclick link [%d] at URL: %s", idx, beforeURLStr)

			select {
			case <-timeoutCtx.Done():
				return nil, timeoutCtx.Err()
			case <-time.After(1 * time.Second):
			}

			// check if URL changed (indicates redirect occurred)
			currentURL, _ := page.Info()
			if currentURL != nil && currentURL.URL != beforeURLStr {
				gologger.Debug().Msgf("detected navigation to: %s", currentURL.URL)
				navigatedURLs.Append(currentURL.URL)

				if navErr := page.Navigate(request.URL); navErr != nil {
					gologger.Warning().Msgf("Failed to navigate back to %s after onclick redirect: %v", request.URL, navErr)
					if reloadErr := page.Reload(); reloadErr != nil {
						gologger.Error().Msgf("Failed to reload page after navigation error: %v", reloadErr)
						break
					}
				}
				select {
				case <-timeoutCtx.Done():
					return nil, timeoutCtx.Err()
				case <-time.After(500 * time.Millisecond):
				}
			}
		}
	}

	// Attempt to get the full DOM tree for shadow DOM traversal.
	// Use basePage (pre-timeout) with a fresh timeout so that DOM inspection
	// does not share the navigation timeout budget. If it fails (e.g. timeout
	// on complex SPAs), we still proceed with regular page HTML.
	var domResult *proto.DOMGetDocumentResult
	domPage := sessionPage.Timeout(timeout)
	var getDocumentDepth = int(-1)
	getDocument := &proto.DOMGetDocument{Depth: &getDocumentDepth, Pierce: true}
	domResult, domErr := getDocument.Call(domPage)
	if domErr != nil {
		gologger.Warning().Msgf("could not get dom for %s: %s (continuing with page HTML)", request.URL, domErr)
	}

	// Use basePage with a fresh timeout for HTML retrieval so it succeeds
	// even if the navigation or DOM timeout was exhausted.
	body, err := sessionPage.Timeout(timeout).HTML()
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: could not get html")
	}

	parsed, err := urlutil.Parse(request.URL)
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: url could not be parsed")
	}

	// Precedence guard: a main document that reached loadingFailed is a
	// transport failure, no artifact — regardless of whatever the async
	// response-stage hijack left in `response`. Keyed on the watcher's recorded
	// failure flag (write-once under the watcher's own lock, which the hijack
	// callback cannot flip), this closes the silent "partial-as-complete" store
	// for a page whose main document failed after waitNavigation returned (e.g.
	// via the per-nav timeout) rather than via the race above.
	if mainDocWatch != nil {
		if failed, reason := mainDocWatch.settledFailure(s.Ctx); failed {
			return nil, fmt.Errorf("%w: %s", errMainDocumentFailed, reason)
		}
	}

	if response == nil || response.Resp == nil {
		// err is guaranteed to be nil, due to previous checks.
		return nil, errors.New("hybrid: response is nil")
	}
	response.Resp.Request.URL = parsed.URL

	// Create a copy of interpolated shadow DOM elements and parse them separately
	if domResult != nil && domResult.Root != nil {
		var builder strings.Builder
		traverseDOMNode(domResult.Root, &builder)

		responseCopy := *response
		responseCopy.Body = builder.String()

		responseCopy.Reader, _ = goquery.NewDocumentFromReader(strings.NewReader(responseCopy.Body))
		if responseCopy.Reader != nil {
			navigationRequests := c.Options.Parser.ParseResponse(&responseCopy)
			c.Enqueue(s.Queue, navigationRequests...)
		}
	}

	response.Body = body
	if response.Reader != nil {
		response.Reader.Url, _ = url.Parse(request.URL)
		if c.Options.Options.FormExtraction {
			response.Forms = append(response.Forms, utils.ParseFormFields(response.Reader)...)
		}
	}

	response.Reader, err = goquery.NewDocumentFromReader(strings.NewReader(response.Body))
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: could not parse html")
	}

	response.XhrRequests = xhrRequests

	// enqueue JS-triggered navigation URLs that were detected
	navigatedURLs.Each(func(i int, navURL string) error {
		if navURL != request.URL {
			parsed, err := urlutil.Parse(navURL)
			if err == nil {
				navReq := &navigation.Request{
					URL:          parsed.String(),
					Depth:        depth,
					RootHostname: s.Hostname,
				}
				c.Enqueue(s.Queue, navReq)
				gologger.Debug().Msgf("enqueued JS navigation: %s", navURL)
			}
		}
		return nil
	})

	// AfterLoad gets its own timeout, like the DOM and HTML reads above:
	// timeoutCtx is the navigation's budget and may be nearly spent by now.
	hookCtx, hookCancel := context.WithTimeout(s.Ctx, timeout)
	defer hookCancel()
	if err := runAfterLoad(hookCtx, c.hooks, sessionPage, request, response); err != nil {
		return nil, err
	}

	return response, nil
}

// errMainDocumentFailed is the named sentinel returned when the main document's
// request terminally failed (Network.loadingFailed) so the configured lifecycle
// event can never fire. It is a transport failure: our wrapper classifies the
// returned error via isTransportFailure → no artifact, the same class the
// per-nav timeout produces today, reached promptly instead of at the timeout.
var errMainDocumentFailed = errors.New("hybrid: main document request failed")

// mainDocSettleWindow is the short grace after the main document's
// loadingFailed before a failure verdict finalizes, letting an immediately
// superseding top-frame navigation (a client-side redirect / re-nav with a new
// requestId) replace it first. It only ever delays a FAILURE verdict; it never
// touches a successful page.
const mainDocSettleWindow = 250 * time.Millisecond

// mainDocWatcher is the rod/CDP I/O layer that drives the pure mainDocNav state
// machine. It subscribes to Network.requestWillBeSent / loadingFailed /
// loadingFinished on one page, maps each event onto the state machine's plain-
// struct methods under a mutex, and signals `notify` each time a failure verdict
// is finalized after the settle window. Consumers must revalidate with
// failedNow(): a notification can be stale by the time it is received.
//
// It is self-contained (rod/CDP only). The correlation logic lives in the
// mainDocNav state machine below.
type mainDocWatcher struct {
	mainFrameID proto.PageFrameID

	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	nav     mainDocNav
	failGen int
	reason  string
	// failedAt is when the currently pending failure was observed; the
	// acceptance check waits until failedAt+mainDocSettleWindow before judging.
	failedAt time.Time

	// notify carries "a failure verdict finalized; recheck failedNow()". It is
	// buffered (1) with a non-blocking send so the finalizer never blocks and
	// coalesced notifications are harmless; it is never closed, so a stale
	// notification consumed by a waiter does not poison later waits.
	notify chan struct{}
}

// armMainDocWatcher enables the Network domain, reads the main frame id, and
// installs the subscription — all BEFORE the caller navigates. It returns an
// error (never a silent empty watcher) so a failure to arm is logged, not
// mistaken for "no failure observed".
func armMainDocWatcher(page *rod.Page, parent context.Context) (*mainDocWatcher, error) {
	if page == nil {
		return nil, errors.New("hybrid: cannot arm main-doc watcher: no page")
	}
	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		return nil, fmt.Errorf("hybrid: enable CDP Network domain: %w", err)
	}
	tree, err := (proto.PageGetFrameTree{}).Call(page)
	if err != nil {
		return nil, fmt.Errorf("hybrid: read page frame tree: %w", err)
	}
	if tree.FrameTree == nil || tree.FrameTree.Frame == nil {
		return nil, errors.New("hybrid: page frame tree has no main frame")
	}

	subCtx, cancel := context.WithCancel(parent)
	w := &mainDocWatcher{
		mainFrameID: tree.FrameTree.Frame.ID,
		ctx:         subCtx,
		cancel:      cancel,
		notify:      make(chan struct{}, 1),
	}
	// EachEvent hands back a BLOCKING consume loop bound to subCtx; cancelling
	// subCtx (stop, or the parent timeout) tears the subscription down.
	consume := page.Context(subCtx).EachEvent(
		w.onRequestWillBeSent,
		w.onLoadingFailed,
		w.onLoadingFinished,
	)
	go consume()
	return w, nil
}

// onRequestWillBeSent maps a CDP request event onto the state machine. The main
// document is discriminated reliably by requestId == loaderId on a main-frame
// Document request: subresources share the main frame id and loaderId
// but carry their OWN requestId, so requestId == loaderId is false for them.
// A redirect continuation reuses the same requestId with redirectResponse set.
func (w *mainDocWatcher) onRequestWillBeSent(e *proto.NetworkRequestWillBeSent) {
	isMainDoc := e.Type == proto.NetworkResourceTypeDocument &&
		e.FrameID == w.mainFrameID &&
		string(e.RequestID) == string(e.LoaderID)
	w.mu.Lock()
	w.nav.onRequestWillBeSent(string(e.RequestID), isMainDoc, e.RedirectResponse != nil)
	w.mu.Unlock()
}

// onLoadingFailed records a failure and, when it freshly fails the watched main
// document, arms a settle timer guarded by a generation so a superseding
// navigation (or a later distinct failure) cannot be finalized by a stale timer.
func (w *mainDocWatcher) onLoadingFailed(e *proto.NetworkLoadingFailed) {
	w.mu.Lock()
	before := w.nav.failed
	w.nav.onLoadingFailed(string(e.RequestID))
	if !before && w.nav.failed {
		if e.ErrorText != "" {
			w.reason = e.ErrorText
		} else if e.BlockedReason != "" {
			w.reason = string(e.BlockedReason)
		} else {
			w.reason = "loadingFailed"
		}
		w.failGen++
		gen := w.failGen
		w.failedAt = time.Now()
		w.mu.Unlock()
		go w.finalizeAfter(gen)
		return
	}
	w.mu.Unlock()
}

// onLoadingFinished retires the watched main document from failure-watching.
func (w *mainDocWatcher) onLoadingFinished(e *proto.NetworkLoadingFinished) {
	w.mu.Lock()
	w.nav.onLoadingFinished(string(e.RequestID))
	w.mu.Unlock()
}

// finalizeAfter waits the settle window, then finalizes a failure verdict IFF
// this is still the current failure generation (not superseded or overtaken).
func (w *mainDocWatcher) finalizeAfter(gen int) {
	if mainDocSettleWindow > 0 {
		select {
		case <-w.ctx.Done():
			return
		case <-time.After(mainDocSettleWindow):
		}
	}
	w.mu.Lock()
	if w.failGen != gen {
		w.mu.Unlock()
		return
	}
	w.nav.settleElapsed()
	v := w.nav.verdict()
	w.mu.Unlock()
	if v == mdFailed {
		select {
		case w.notify <- struct{}{}:
		default:
		}
	}
}

// notifyChan returns the failure-notification channel, or nil (blocks forever
// in a select) when no watcher is armed.
func (w *mainDocWatcher) notifyChan() <-chan struct{} {
	if w == nil {
		return nil
	}
	return w.notify
}

// failedNow revalidates a notification against current state: true only if
// the watched main document's failure is finalized and not since superseded or
// retired. Safe on a nil watcher (reports false).
func (w *mainDocWatcher) failedNow() (bool, string) {
	if w == nil {
		return false, ""
	}
	w.mu.Lock()
	failed := w.nav.verdict() == mdFailed
	w.mu.Unlock()
	if !failed {
		return false, ""
	}
	return true, w.failureReason()
}

// sleepUnlessFailed waits d, returning errMainDocumentFailed early if a
// revalidated main-document failure is signalled meanwhile. Stale
// notifications are ignored and the wait resumes for the remaining time. It
// returns nil when d elapses or ctx ends (the caller's next step handles ctx
// exactly as before). On a nil watcher it is a plain ctx-aware sleep.
func (w *mainDocWatcher) sleepUnlessFailed(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return nil
		case <-w.notifyChan():
			if failed, reason := w.failedNow(); failed {
				return fmt.Errorf("%w: %s", errMainDocumentFailed, reason)
			}
		}
	}
}

// settledFailure is the acceptance-point check: it reports whether the watched
// main document failed and was neither superseded nor retired, honouring the
// same settle window the race uses. A failure observed less than
// mainDocSettleWindow ago is waited out (so an immediately superseding
// navigation can still replace it) and then re-checked against current state;
// a replacement that itself fails restarts the wait for its own window. If ctx
// ends during the wait, the observed failure stands: it was real, and the
// crawl is ending anyway.
func (w *mainDocWatcher) settledFailure(ctx context.Context) (bool, string) {
	for {
		w.mu.Lock()
		pending := w.nav.failed && !w.nav.finished
		reason := w.reason
		remaining := time.Until(w.failedAt.Add(mainDocSettleWindow))
		w.mu.Unlock()
		if !pending {
			return false, ""
		}
		if reason == "" {
			reason = "loadingFailed"
		}
		if remaining <= 0 {
			return true, reason
		}
		select {
		case <-ctx.Done():
			return true, reason
		case <-time.After(remaining):
		}
	}
}

// failureReason returns the CDP errorText/blockedReason captured for the sentinel.
func (w *mainDocWatcher) failureReason() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.reason == "" {
		return "loadingFailed"
	}
	return w.reason
}

// stop tears down the subscription and its settle goroutines.
func (w *mainDocWatcher) stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (c *Crawler) addHeadersToPage(page *rod.Page) {
	if len(c.Headers) == 0 {
		return
	}

	var arr []string

	for k, v := range c.Headers {
		switch {
		case stringsutil.EqualFoldAny(k, "User-Agent"):
			userAgentParams := &proto.NetworkSetUserAgentOverride{
				UserAgent: v,
			}
			if err := page.SetUserAgent(userAgentParams); err != nil {
				gologger.Error().Msgf("headless: could not set user agent: %v", err)
			}
		default:
			arr = append(arr, k, v)
		}
	}

	if len(arr) > 0 {
		_, err := page.SetExtraHeaders(arr)
		if err != nil {
			gologger.Error().Msgf("headless: could not set extra headers: %v", err)
		}
	}
}

// traverseDOMNode performs traversal of node completely building a pseudo-HTML
// from it including the Shadow DOM, Pseudo elements and other children.
//
// TODO: Remove this method when we implement human-like browser navigation
// which will anyway use browser APIs to find elements instead of goquery
// where they will have shadow DOM information.
func traverseDOMNode(node *proto.DOMNode, builder *strings.Builder) {
	buildDOMFromNode(node, builder)
	if node.TemplateContent != nil {
		traverseDOMNode(node.TemplateContent, builder)
	}
	if node.ContentDocument != nil {
		traverseDOMNode(node.ContentDocument, builder)
	}
	for _, children := range node.Children {
		traverseDOMNode(children, builder)
	}
	for _, shadow := range node.ShadowRoots {
		traverseDOMNode(shadow, builder)
	}
	for _, pseudo := range node.PseudoElements {
		traverseDOMNode(pseudo, builder)
	}
}

const (
	elementNode = 1
)

var knownElements = map[string]struct{}{
	"a": {}, "applet": {}, "area": {}, "audio": {}, "base": {}, "blockquote": {}, "body": {}, "button": {}, "embed": {}, "form": {}, "frame": {}, "html": {}, "iframe": {}, "img": {}, "import": {}, "input": {}, "isindex": {}, "link": {}, "meta": {}, "object": {}, "script": {}, "svg": {}, "table": {}, "video": {},
}

func buildDOMFromNode(node *proto.DOMNode, builder *strings.Builder) {
	if node.NodeType != elementNode {
		return
	}
	if _, ok := knownElements[node.LocalName]; !ok {
		return
	}
	builder.WriteRune('<')
	builder.WriteString(node.LocalName)
	builder.WriteRune(' ')
	if len(node.Attributes) > 0 {
		for i := 0; i < len(node.Attributes); i = i + 2 {
			builder.WriteString(node.Attributes[i])
			builder.WriteRune('=')
			builder.WriteString("\"")
			builder.WriteString(node.Attributes[i+1])
			builder.WriteString("\"")
			builder.WriteRune(' ')
		}
	}
	builder.WriteRune('>')
	builder.WriteString("</")
	builder.WriteString(node.LocalName)
	builder.WriteRune('>')
}

// subresourceParseHeaders are the response headers that change what the
// parsers extract from an otherwise identical body.
var subresourceParseHeaders = []string{"Content-Location", "Content-Type", "Link", "Location", "Refresh"}

// shouldParse reports whether a hijacked response is parsed for new links.
// Page-similarity applies only to the main document, and that check is the
// body alone. A subresource is parsed once for the same request URL, body,
// and subresourceParseHeaders, so a shared script is not parsed on every
// page. The same bytes at another URL, or with different headers, are parsed
// again. An empty body is always parsed, so redirect headers still apply.
func (c *Crawler) shouldParse(isMainDoc bool, reqURL string, headers map[string][]string, body []byte) bool {
	if !isMainDoc {
		if c.Options.Options.DisableUniqueFilter || len(body) == 0 {
			return true
		}
		return c.Options.UniqueFilter.UniqueContent(subresourceIdentity(reqURL, headers, body))
	}
	if !c.Options.Options.DisableUniqueFilter && !c.Options.UniqueFilter.UniqueContent(body) {
		return false
	}
	return c.Options.ContentSimilarity == nil || c.Options.ContentSimilarity.Accept(body)
}

// subresourceIdentity is a digest of the inputs ParseResponse reads for a
// subresource. Hashing the parts avoids copying the body into one buffer.
func subresourceIdentity(reqURL string, headers map[string][]string, body []byte) []byte {
	h := md5.New()
	_, _ = io.WriteString(h, reqURL)
	_, _ = h.Write([]byte{0})
	for _, name := range subresourceParseHeaders {
		_, _ = io.WriteString(h, name)
		_, _ = h.Write([]byte{0})
		_, _ = io.WriteString(h, headerValues(headers, name))
		_, _ = h.Write([]byte{0})
	}
	_, _ = h.Write(body)
	return h.Sum(nil)
}

func headerValues(headers map[string][]string, name string) string {
	if len(headers) == 0 {
		return ""
	}
	for _, key := range []string{name, http.CanonicalHeaderKey(name), strings.ToLower(name)} {
		if values, ok := headers[key]; ok {
			return strings.Join(values, ",")
		}
	}
	for key, values := range headers {
		if strings.EqualFold(key, name) {
			return strings.Join(values, ",")
		}
	}
	return ""
}

// waitAnyLifecycleEvent is rod's Page.WaitNavigation for several events.
// Small or blank pages never fire FirstMeaningfulPaint, so waiting on it
// alone stalls until the request times out. Unlike rod, it only accepts the
// main frame's events from the next navigation: enabling lifecycle events
// replays the current document's networkIdle, and iframes fire their own.
func waitAnyLifecycleEvent(page *rod.Page, names ...proto.PageLifecycleEventName) func() {
	_ = proto.PageSetLifecycleEventsEnabled{Enabled: true}.Call(page)
	var loader proto.NetworkLoaderID
	wait := page.EachEvent(func(e *proto.PageLifecycleEvent) bool {
		if e.FrameID != page.FrameID {
			return false
		}
		if e.Name == proto.PageLifecycleEventNameInit {
			loader = e.LoaderID
			return false
		}
		return loader != "" && e.LoaderID == loader && slices.Contains(names, e.Name)
	})
	return func() {
		wait()
		_ = proto.PageSetLifecycleEventsEnabled{Enabled: false}.Call(page)
	}
}

// isStreamingResponse reports whether a paused response is a streaming protocol
// (such as EventSource/SSE or WebSocket) or streaming Content-Type where calling
// FetchGetResponseBody blocks waiting for EOF until timeout.
func isStreamingResponse(resourceType proto.NetworkResourceType, headers []*proto.FetchHeaderEntry) bool {
	if resourceType == proto.NetworkResourceTypeEventSource || resourceType == proto.NetworkResourceTypeWebSocket {
		return true
	}
	for _, h := range headers {
		if strings.EqualFold(h.Name, "content-type") {
			ct := strings.ToLower(h.Value)
			if strings.Contains(ct, "text/event-stream") ||
				strings.Contains(ct, "application/stream+json") ||
				strings.Contains(ct, "application/x-ndjson") {
				return true
			}
		}
	}
	return false
}

// mainDocNav is a pure, dependency-free correlation state machine that decides
// whether a hybrid-crawl main-document request has terminally FAILED. It never
// decides success — a healthy page is owned by the concurrent lifecycle-event
// wait.
//
// It is fed plain-struct events mapped from CDP by the I/O layer and models
// CDP's actual redirect semantics: a redirect REUSES the same requestId (with
// redirectResponse set), while a brand-new main-frame Document request is a
// replacement navigation that re-points the watch and discards any pending
// failure from the document the page navigated away from.
type mainDocNav struct {
	// watched is the requestId of the main document currently being tracked.
	// Empty until the initial main document's requestWillBeSent is observed.
	watched string
	// captured is true once the initial main document has been identified.
	captured bool
	// failed is true when loadingFailed was observed on the watched request and
	// has not since been superseded by a replacement navigation or retired by a
	// loadingFinished.
	failed bool
	// finished is true when loadingFinished was observed on the watched request:
	// a terminal success that makes a later loadingFailed on that id a no-op.
	finished bool
	// settleDone records that the post-failure settle window has elapsed, so a
	// superseding navigation no longer has a chance to replace the failure.
	settleDone bool
}

// mdVerdict is the finalized decision the I/O layer consults.
type mdVerdict int

const (
	// mdPending means there is no terminal failure to report: the watcher stays
	// silent and the lifecycle-event wait / per-nav timeout governs as today.
	mdPending mdVerdict = iota
	// mdFailed means the final main document terminally failed: fast-fail now.
	mdFailed
)

// onRequestWillBeSent tracks the main document across redirects and replacement
// navigations.
//
//   - isMainDoc is the I/O layer's reliable "this is THE main document" signal:
//     for the initial request it is requestId == the loaderId returned by
//     Page.navigate; for a replacement it is a main-frame type:Document request.
//     frameId+type alone does not discriminate the main document from same-frame
//     subresources, so the I/O layer resolves isMainDoc and passes it.
//   - hasRedirectResponse marks a redirect continuation, which REUSES the
//     watched requestId; it must not be mistaken for a replacement navigation.
func (m *mainDocNav) onRequestWillBeSent(reqID string, isMainDoc bool, hasRedirectResponse bool) {
	if reqID == "" {
		return
	}
	// A redirect continuation reuses the watched requestId: keep watching it,
	// regardless of how the I/O layer classified this hop.
	if m.captured && reqID == m.watched && hasRedirectResponse {
		return
	}
	if !isMainDoc {
		return
	}
	// Initial capture of the main document.
	if !m.captured {
		m.watched = reqID
		m.captured = true
		return
	}
	// Same request id, no redirect marker: nothing changes.
	if reqID == m.watched {
		return
	}
	// A NEW main-frame Document request is a replacement navigation: re-point to
	// it and discard any pending failure from the document we navigated away
	// from, so the watcher only ever reports the FINAL main document's failure.
	m.watched = reqID
	m.failed = false
	m.finished = false
	m.settleDone = false
}

// onLoadingFailed records a terminal failure of the watched main document. A
// failure that arrives after loadingFinished is ignored (terminal precedence).
func (m *mainDocNav) onLoadingFailed(reqID string) {
	if !m.captured || reqID != m.watched || m.finished {
		return
	}
	m.failed = true
	m.settleDone = false
}

// onLoadingFinished retires the watched main document from failure-watching. It
// is NOT a success signal to act on; it only guarantees a later loadingFailed on
// that id cannot fire, because a finished main document means the lifecycle
// event is coming and the concurrent wait will complete the page.
func (m *mainDocNav) onLoadingFinished(reqID string) {
	if !m.captured || reqID != m.watched {
		return
	}
	m.finished = true
	m.failed = false
	m.settleDone = false
}

// settleElapsed marks that the settle window after a failure has passed, letting
// a verdict finalize. It is a no-op unless a failure is currently pending.
func (m *mainDocNav) settleElapsed() {
	if m.failed && !m.finished {
		m.settleDone = true
	}
}

// verdict is the finalized decision: mdFailed only when a failure was observed,
// not retired by a finish, not superseded by a replacement, and the settle
// window has elapsed. Otherwise mdPending — the watcher never signals success.
func (m *mainDocNav) verdict() mdVerdict {
	if m.failed && !m.finished && m.settleDone {
		return mdFailed
	}
	return mdPending
}
