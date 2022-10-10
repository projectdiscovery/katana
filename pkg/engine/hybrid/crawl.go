package hybrid

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
)

func (c *Crawler) navigateRequest(ctx context.Context, queue *queue.VarietyQueue, parseResponseCallback func(nr navigation.Request), browser *rod.Browser, request navigation.Request, rootHostname string) (*navigation.Response, error) {
	depth := request.Depth + 1
	response := &navigation.Response{
		Depth:        depth,
		Options:      c.options,
		RootHostname: rootHostname,
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	defer page.Close()

	if c.options.Options.NativeHttp {
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

			bodyReader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
			resp := navigation.Response{
				Resp:         httpresp,
				Body:         []byte(body),
				Reader:       bodyReader,
				Options:      c.options,
				Depth:        depth,
				RootHostname: rootHostname,
			}

			// process the raw response
			parser.ParseResponse(resp, parseResponseCallback)
			return FetchContinueRequest(page, e)
		})() //nolint
		defer func() {
			if err := pageRouter.Stop(); err != nil {
				gologger.Warning().Msgf("%s\n", err)
			}
		}()
	} else {
		pageRouter := page.HijackRequests()
		err = pageRouter.Add("*", "", c.makeRoutingHandler(queue, depth, rootHostname, parseResponseCallback))
		if err != nil {
			return nil, err
		}
		go pageRouter.Run()
		defer func() {
			if err := pageRouter.Stop(); err != nil {
				gologger.Warning().Msgf("%s\n", err)
			}
		}()
	}

	timeout := time.Duration(c.options.Options.Timeout) * time.Second
	page = page.Timeout(timeout)

	// wait the page to be fully loaded and becoming idle
	waitNavigation := page.WaitNavigation(proto.PageLifecycleEventNameDOMContentLoaded)

	if err := page.Navigate(request.URL); err != nil {
		return nil, err
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

	body, err := page.HTML()
	if err != nil {
		return nil, err
	}

	response.Body = []byte(body)
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
	if err != nil {
		return nil, err
	}

	return response, nil
}
