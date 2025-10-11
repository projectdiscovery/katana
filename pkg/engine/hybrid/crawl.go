package hybrid

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
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
	stringsutil "github.com/projectdiscovery/utils/strings"
	urlutil "github.com/projectdiscovery/utils/url"
)

func (c *Crawler) navigateRequest(s *common.CrawlSession, request *navigation.Request) (*navigation.Response, error) {
	depth := request.Depth + 1
	response := &navigation.Response{
		Depth:        depth,
		RootHostname: s.Hostname,
	}

	page, err := s.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: could not create target")
	}
	defer func() {
		if err := page.Close(); err != nil {
			gologger.Error().Msgf("Error closing page: %v\n", err)
		}
	}()
	c.addHeadersToPage(page)

	pageRouter := NewHijack(page)
	pageRouter.SetPattern(&proto.FetchRequestPattern{
		URLPattern:   "*",
		RequestStage: proto.FetchRequestStageResponse,
	})

	xhrRequests := []navigation.Request{}
	go pageRouter.Start(func(e *proto.FetchRequestPaused) error {
		URL, err := urlutil.Parse(e.Request.URL)
		if err != nil {
			return errkit.Wrap(err, "hybrid: could not parse URL")
		}
		
		// Check if we should handle this request with proxy filtering
		shouldUseProxyFiltering := c.Options.Options.ProxyFiltering && 
			c.Options.ProxyFilterPipeline != nil && 
			c.Options.ProxyFilterPipeline.IsEnabled()
		
		var body []byte
		var statusCode int
		var statucCodeText string
		var headers map[string][]string
		
		if shouldUseProxyFiltering {
			// Handle request through our proxy-aware HTTP client
			body, statusCode, statucCodeText, headers, err = c.handleProxyFilteredRequest(e, s.Hostname)
			if err != nil {
				gologger.Debug().Msgf("Proxy filtered request failed, falling back to browser: %v", err)
				// Fall back to browser handling
				body, _ = FetchGetResponseBody(page, e)
				headers = make(map[string][]string)
				for _, h := range e.ResponseHeaders {
					headers[h.Name] = []string{h.Value}
				}
				if e.ResponseStatusCode != nil {
					statusCode = *e.ResponseStatusCode
				}
				if e.ResponseStatusText != "" {
					statucCodeText = e.ResponseStatusText
				} else {
					statucCodeText = http.StatusText(statusCode)
				}
			}
		} else {
			// Traditional browser handling
			body, _ = FetchGetResponseBody(page, e)
			headers = make(map[string][]string)
			for _, h := range e.ResponseHeaders {
				headers[h.Name] = []string{h.Value}
			}
			if e.ResponseStatusCode != nil {
				statusCode = *e.ResponseStatusCode
			}
			if e.ResponseStatusText != "" {
				statucCodeText = e.ResponseStatusText
			} else {
				statucCodeText = http.StatusText(statusCode)
			}
		}
		
		httpreq, err := http.NewRequest(e.Request.Method, URL.String(), strings.NewReader(e.Request.PostData))
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
		}
		response.ContentLength = resp.ContentLength

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

		// trim trailing /
		normalizedheadlessURL := strings.TrimSuffix(e.Request.URL, "/")
		matchOriginalURL := stringsutil.EqualFoldAny(request.URL, e.Request.URL, normalizedheadlessURL)
		if matchOriginalURL {
			request.Raw = string(rawBytesRequest)
			response = resp
		}

		// process the raw response
		navigationRequests := c.Options.Parser.ParseResponse(resp)
		c.Enqueue(s.Queue, navigationRequests...)

		// do not continue following the request if it's a redirect and redirects are disabled
		if c.Options.Options.DisableRedirects && resp.IsRedirect() {
			return nil
		}
		return FetchContinueRequest(page, e)
	})() //nolint
	defer func() {
		if err := pageRouter.Stop(); err != nil {
			gologger.Warning().Msgf("%s\n", err)
		}
	}()

	timeout := time.Duration(c.Options.Options.Timeout) * time.Second
	page = page.Timeout(timeout)

	// wait the page to be fully loaded and becoming idle
	waitNavigation := page.WaitNavigation(proto.PageLifecycleEventNameFirstMeaningfulPaint)

	err = page.Navigate(request.URL)
	if err != nil {
		if c.Options.Options.DisableRedirects && response.IsRedirect() {
			return response, nil
		}
		return nil, errkit.Wrap(err, "hybrid: could not navigate target")
	}

	waitNavigation()

	// Wait the page to be stable a duration
	timeStable := time.Duration(c.Options.Options.TimeStable) * time.Second

	if timeout < timeStable {
		gologger.Warning().Msgf("timeout is less than time stable, setting time stable to half of timeout to avoid timeout\n")
		timeStable = timeout / 2
		gologger.Warning().Msgf("setting time stable to %s\n", timeStable)
	}

	if err := page.WaitStable(timeStable); err != nil {
		gologger.Warning().Msgf("could not wait for page to be stable: %s\n", err)
	}

	var getDocumentDepth = int(-1)
	getDocument := &proto.DOMGetDocument{Depth: &getDocumentDepth, Pierce: true}
	result, err := getDocument.Call(page)
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: could not get dom")
	}
	var builder strings.Builder
	traverseDOMNode(result.Root, &builder)

	body, err := page.HTML()
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: could not get html")
	}

	parsed, err := urlutil.Parse(request.URL)
	if err != nil {
		return nil, errkit.Wrap(err, "hybrid: url could not be parsed")
	}

	if response == nil || response.Resp == nil {
		// err is guaranteed to be nil, due to previous checks.
		return nil, errors.New("hybrid: response is nil")
	}
	response.Resp.Request.URL = parsed.URL

	// Create a copy of intrapolated shadow DOM elements and parse them separately
	responseCopy := *response
	responseCopy.Body = builder.String()

	responseCopy.Reader, _ = goquery.NewDocumentFromReader(strings.NewReader(responseCopy.Body))
	if responseCopy.Reader != nil {
		navigationRequests := c.Options.Parser.ParseResponse(&responseCopy)
		c.Enqueue(s.Queue, navigationRequests...)
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

	// Handle JavaScript interactions (clicks, onclick events, etc.) if enabled
	if c.Options.Options.JavaScriptInteractions {
		gologger.Debug().Msgf("Starting JavaScript interactions with timeout: %d seconds", c.Options.Options.Timeout)
		
		// First, let's verify the page is actually responsive
		pageTitle, err := page.Eval("() => document.title")
		if err != nil {
			gologger.Debug().Msgf("Page is not responsive: %v", err)
		} else {
			gologger.Debug().Msgf("Page is responsive, title: %s", pageTitle.Value.String())
		}
		
		// Check if our target element exists
		linkExists, err := page.Eval("() => document.querySelector('a[onclick]') !== null")
		if err != nil {
			gologger.Debug().Msgf("Cannot check for onclick links: %v", err)
		} else {
			gologger.Debug().Msgf("Onclick link exists: %v", linkExists.Value.Bool())
		}
		
		if jsInteractionRequests, err := c.handleJavaScriptInteractions(page, response); err == nil {
			gologger.Debug().Msgf("Found %d new URLs from JavaScript interactions", len(jsInteractionRequests))
			for _, req := range jsInteractionRequests {
				gologger.Debug().Msgf("Discovered URL: %s", req.URL)
			}
			c.Enqueue(s.Queue, jsInteractionRequests...)
		} else {
			gologger.Warning().Msgf("JavaScript interaction handling failed: %s", err)
		}
	}

	return response, nil
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

// handleJavaScriptInteractions finds and clicks JavaScript-enabled elements to discover hidden URLs
func (c *Crawler) handleJavaScriptInteractions(page *rod.Page, response *navigation.Response) ([]*navigation.Request, error) {
	var navigationRequests []*navigation.Request
	
	gologger.Debug().Msgf("Attempting pure JavaScript approach to find and click elements")
	
	// Get current URL before clicking
	currentURL, err := page.Eval("() => window.location.href")
	if err != nil {
		return navigationRequests, errkit.Wrap(err, "could not get current URL")
	}
	
	// Use pure JavaScript to find and click onclick elements
	clickResult, err := page.Eval(`() => {
		const elements = document.querySelectorAll('a[onclick], [onclick]');
		const results = [];
		
		for (let i = 0; i < Math.min(elements.length, 3); i++) {
			const element = elements[i];
			const text = element.textContent || element.innerText || 'unnamed';
			
			try {
				// Trigger the click event
				element.click();
				results.push({
					text: text.substring(0, 50),
					clicked: true
				});
			} catch (e) {
				results.push({
					text: text.substring(0, 50),
					clicked: false,
					error: e.message
				});
			}
		}
		
		return results;
	}`)
	
	if err != nil {
		return navigationRequests, errkit.Wrap(err, "could not execute JavaScript click")
	}
	
	gologger.Debug().Msgf("JavaScript click results: %v", clickResult.Value)
	
	// Wait for potential navigation
	time.Sleep(1000 * time.Millisecond)
	
	// Check if URL changed (indicating navigation)
	newURL, err := page.Eval("() => window.location.href")
	if err == nil && newURL.Value.String() != currentURL.Value.String() {
		gologger.Debug().Msgf("Navigation detected: %s -> %s", currentURL.Value.String(), newURL.Value.String())
		
		// Create navigation request for the new URL
		newReq := &navigation.Request{
			Method: "GET",
			URL:    newURL.Value.String(),
			Depth:  response.Depth,
		}
		navigationRequests = append(navigationRequests, newReq)
	}

	return navigationRequests, nil
}

// handleProxyFilteredRequest handles a request through our proxy-aware HTTP client
func (c *Crawler) handleProxyFilteredRequest(e *proto.FetchRequestPaused, rootHostname string) ([]byte, int, string, map[string][]string, error) {
	// Validate input parameters
	if e == nil {
		return nil, 0, "", nil, errkit.New("fetch request event cannot be nil")
	}
	if e.Request.URL == "" {
		return nil, 0, "", nil, errkit.New("request URL cannot be empty")
	}

	// Get the appropriate HTTP client based on proxy filtering
	httpClient := c.Options.ProxyConfig.GetClient(e.Request.URL, rootHostname)
	if httpClient == nil {
		return nil, 0, "", nil, errkit.New("failed to get HTTP client - this should not happen")
	}
	
	// Create HTTP request
	req, err := http.NewRequest(e.Request.Method, e.Request.URL, strings.NewReader(e.Request.PostData))
	if err != nil {
		return nil, 0, "", nil, errkit.Wrap(err, "could not create HTTP request")
	}
	
	// Set headers from browser request
	for k, v := range e.Request.Headers {
		req.Header.Set(k, v.String())
	}
	
	// Set user agent if not already set
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", utils.WebUserAgent())
	}
	
	// Convert to retryable request
	retryableReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, 0, "", nil, errkit.Wrap(err, "could not create retryable request")
	}
	
	// Make the request with timeout handling
	resp, err := httpClient.Do(retryableReq)
	if err != nil {
		// Provide more detailed error information
		return nil, 0, "", nil, errkit.Wrap(err, "HTTP request failed for URL: "+e.Request.URL)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			gologger.Warning().Msgf("Failed to close response body for %s: %v", e.Request.URL, closeErr)
		}
	}()
	
	// Read response body with size limit for safety
	const maxBodySize = 50 * 1024 * 1024 // 50MB limit
	limitedReader := io.LimitReader(resp.Body, maxBodySize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, 0, "", nil, errkit.Wrap(err, "could not read response body")
	}
	
	// Convert headers
	headers := make(map[string][]string)
	for k, v := range resp.Header {
		headers[k] = v
	}
	
	// Log proxy decision for debugging
	if c.Options.Options.Debug {
		usingProxy := httpClient == c.Options.ProxyConfig.ProxyClient
		if usingProxy {
			gologger.Debug().Msgf("Hybrid mode: Request to %s sent through proxy (status: %d)", e.Request.URL, resp.StatusCode)
		} else {
			gologger.Debug().Msgf("Hybrid mode: Request to %s sent directly (status: %d)", e.Request.URL, resp.StatusCode)
		}
	}
	
	return body, resp.StatusCode, resp.Status, headers, nil
}

// simulateElementClick simulates clicking an element and captures any resulting navigation or content changes
func (c *Crawler) simulateElementClick(page *rod.Page, element *rod.Element, response *navigation.Response, navigationRequests *[]*navigation.Request) error {
	// Get current page URL for comparison
	currentURL, err := page.Eval("() => window.location.href")
	if err != nil {
		return errkit.Wrap(err, "could not get current URL")
	}

	// Set up navigation monitoring
	page.EachEvent(func(e *proto.PageFrameNavigated) {
		// Navigation detected
	})()

	// Try to click the element using JavaScript (more reliable than physical click)
	_, err = element.Eval("() => this.click()")
	if err != nil {
		return errkit.Wrap(err, "could not click element")
	}

	// Wait a bit for any navigation or content changes
	time.Sleep(500 * time.Millisecond)

	// Check if navigation occurred
	newURL, err := page.Eval("() => window.location.href")
	if err == nil && newURL.Value.String() != currentURL.Value.String() {
		// Navigation occurred, create a new navigation request
		newReq := &navigation.Request{
			Method:       "GET",
			URL:          newURL.Value.String(),
			Depth:        response.Depth,
			RootHostname: response.RootHostname,
			Source:       response.Resp.Request.URL.String(),
			Tag:          "javascript-click",
			Attribute:    "navigation",
		}
		*navigationRequests = append(*navigationRequests, newReq)
		gologger.Debug().Msgf("JavaScript click triggered navigation to: %s", newURL.Value.String())
		return nil
	}

	// No navigation, check for dynamic content changes
	return c.extractDynamicContent(page, response, navigationRequests)
}

// extractDynamicContent extracts URLs from dynamically generated content after JavaScript interactions
func (c *Crawler) extractDynamicContent(page *rod.Page, response *navigation.Response, navigationRequests *[]*navigation.Request) error {
	// Get updated HTML content
	body, err := page.HTML()
	if err != nil {
		return errkit.Wrap(err, "could not get updated HTML")
	}

	// Parse the updated content
	reader, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return errkit.Wrap(err, "could not parse updated HTML")
	}

	// Create a temporary response object for parsing
	tempResponse := &navigation.Response{
		Resp:         response.Resp,
		Body:         body,
		Reader:       reader,
		Depth:        response.Depth,
		RootHostname: response.RootHostname,
	}

	// Extract navigation requests using existing parser
	dynamicRequests := c.Options.Parser.ParseResponse(tempResponse)
	
	// Filter out requests that might be duplicates
	for _, req := range dynamicRequests {
		// Only add requests that are different from the current page
		if req.URL != response.Resp.Request.URL.String() {
			req.Tag = "javascript-dynamic"
			req.Attribute = "generated"
			req.Source = response.Resp.Request.URL.String()
			*navigationRequests = append(*navigationRequests, req)
		}
	}

	return nil
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
