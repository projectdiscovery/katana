package hybrid

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/navigation"
)

func (c *Crawler) navigateRequest(parseResponseCallback func(nr navigation.Request), browser *rod.Browser, request navigation.Request, rootHostname string, crawlerGraph *navigation.Graph) ([]*navigation.Response, error) {
	depth := request.Depth + 1
	response := &navigation.Response{
		Depth:        depth,
		Options:      c.options,
		RootHostname: rootHostname,
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, errors.Wrap(err, "could not create target")
	}
	defer page.Close()

	var asyncronousResponses []*navigation.Response
	pageRouter := NewHijack(page)
	pageRouter.SetPattern(&proto.FetchRequestPattern{
		URLPattern:   "*",
		RequestStage: proto.FetchRequestStageResponse,
	})
	go pageRouter.Start(func(e *proto.FetchRequestPaused) error {
		URL, _ := url.Parse(e.Request.URL)
		body, _ := FetchGetResponseBody(page, e)
		headers := make(map[string][]string)
		for _, h := range e.ResponseHeaders {
			headers[h.Name] = []string{h.Value}
		}
		var statuscode int
		if e.ResponseStatusCode != nil {
			statuscode = *e.ResponseStatusCode
		}
		httpresp := &http.Response{
			StatusCode: statuscode,
			Status:     e.ResponseStatusText,
			Header:     headers,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request: &http.Request{
				Method: e.Request.Method,
				URL:    URL,
				Body:   io.NopCloser(strings.NewReader(e.Request.PostData)),
			},
		}
		if !c.options.UniqueFilter.UniqueContent(body) {
			return FetchContinueRequest(page, e)
		}

		bodyReader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		resp := &navigation.Response{
			Resp:         httpresp,
			Body:         []byte(body),
			Reader:       bodyReader,
			Options:      c.options,
			Depth:        depth,
			RootHostname: rootHostname,
		}

		asyncronousResponses = append(asyncronousResponses, resp)

		// process the raw response
		// parser.ParseResponse(*resp, parseResponseCallback)

		return FetchContinueRequest(page, e)
	})() //nolint
	defer func() {
		if err := pageRouter.Stop(); err != nil {
			gologger.Warning().Msgf("%s\n", err)
		}
	}()

	timeout := time.Duration(c.options.Options.Timeout) * time.Second
	page = page.Timeout(timeout)

	// wait the page to be fully loaded and becoming idle
	waitNavigation := page.WaitNavigation(proto.PageLifecycleEventNameFirstMeaningfulPaint)

	if err := page.Navigate(request.URL); err != nil {
		return nil, errors.Wrap(err, "could not navigate target")
	}
	waitNavigation()

	// Wait for the window.onload event
	if err := page.WaitLoad(); err != nil {
		gologger.Warning().Msgf("\"%s\" on wait load: %s\n", request.URL, err)
	}

	// wait for idle the network requests
	if err := page.WaitIdle(timeout); err != nil {
		gologger.Warning().Msgf("\"%s\" on wait idle: %s\n", request.URL, err)
	}

	var getDocumentDepth = int(-1)
	getDocument := &proto.DOMGetDocument{Depth: &getDocumentDepth, Pierce: true}
	result, err := getDocument.Call(page)
	if err != nil {
		return nil, errors.Wrap(err, "could not get dom")
	}
	var builder strings.Builder
	traverseDOMNode(result.Root, &builder)

	body, err := page.HTML()
	if err != nil {
		return nil, errors.Wrap(err, "could not get html")
	}

	parsed, _ := url.Parse(request.URL)
	response.Resp = &http.Response{Header: make(http.Header), Request: &http.Request{URL: parsed}}

	// Create a copy of intrapolated shadow DOM elements and parse them separately
	responseShadowDom := *response
	responseShadowDom.Body = []byte(builder.String())
	if !c.options.UniqueFilter.UniqueContent(responseShadowDom.Body) {
		return nil, nil
	}

	responseShadowDom.Reader, _ = goquery.NewDocumentFromReader(bytes.NewReader(responseShadowDom.Body))

	response.Body = []byte(body)
	if !c.options.UniqueFilter.UniqueContent(response.Body) {
		return nil, nil
	}
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
	if err != nil {
		return nil, errors.Wrap(err, "could not parse html")
	}

	responses := []*navigation.Response{response, &responseShadowDom}

	return append(responses, asyncronousResponses...), nil
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
