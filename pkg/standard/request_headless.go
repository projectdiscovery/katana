package standard

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

type NetworkPair struct {
	Request  *http.Request
	Response *http.Response
}

// makeHeadlessRequest starts with making a simple headless requests, we will further improve context and navigation later
func (c *Crawler) makeHeadlessRequest(request navigationRequest) (navigationResponse, error) {
	response := navigationResponse{
		Depth:   request.Depth + 1,
		options: c.options,
	}

	// Launch a new browser with default options, and connect to it.
	browser := rod.New()
	if err := browser.Connect(); err != nil {
		panic(err)
	}

	// Even you forget to close, rod will close it after main process ends.
	defer browser.MustClose()

	router := browser.HijackRequests()
	defer router.Stop()

	networkMap := make(map[string]*NetworkPair)

	router.MustAdd("*", func(ctx *rod.Hijack) {
		for k, v := range request.Headers {
			ctx.Request.Req().Header.Set(k, v)
		}
		for k, v := range c.options.Options.CustomHeaders.AsMap() {
			ctx.Request.Req().Header.Set(k, v.(string))
		}

		err := ctx.LoadResponse(c.httpclient.HTTPClient, true)
		if err != nil {
			panic(err)
		}
		networkMap[ctx.Request.URL().String()] = &NetworkPair{
			Request: ctx.Request.Req(),
			Response: &http.Response{
				Header:     ctx.Response.Headers(),
				Body:       io.NopCloser(strings.NewReader(ctx.Response.Body())),
				StatusCode: ctx.Response.Payload().ResponseCode,
				Status:     ctx.Response.Payload().ResponsePhrase,
			},
		}
	})

	go router.Run()

	// Create a new page
	page, err := browser.Page(proto.TargetCreateTarget{URL: request.URL})
	if err != nil {
		panic(err)
	}

	// we need to process only the HTML as redirects and refreshes are already handled automatically by the browser
	html, err := page.HTML()
	if err != nil {
		panic(err)
	}

	limitReader := io.LimitReader(strings.NewReader(html), int64(c.options.Options.BodyReadSize))
	data, err := ioutil.ReadAll(limitReader)
	if err != nil {
		return response, err
	}
	response.Body = data
	// rebuild http response

	if networkPair, ok := networkMap[request.RequestURL()]; ok {
		response.Resp = networkPair.Response
	}

	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return response, errors.Wrap(err, "could not make document from reader")
	}
	return response, nil
}
