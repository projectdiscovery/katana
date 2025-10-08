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
		body, _ := FetchGetResponseBody(page, e)
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

	// simulate clicks on links with onclick handlers to discover JS redirects
	time.Sleep(200 * time.Millisecond)

	clickableLinks, err := page.Elements("a[onclick]")
	if err == nil && len(clickableLinks) > 0 {
		maxLinks := c.Options.Options.MaxOnclickLinks
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

			time.Sleep(1 * time.Second)

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
				time.Sleep(500 * time.Millisecond)
			}
		}
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
