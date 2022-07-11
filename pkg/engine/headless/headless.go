package headless

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-rod/rod"
	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/urlutil"
)

type HeadlessEngine struct {
	options    *types.CrawlerOptions
	httpclient *retryablehttp.Client
	dialer     *fastdialer.Dialer
	browser    *rod.Browser
	router     *rod.HijackRouter
	networkMap map[string]*NetworkPair
}

// New returns a new crawler instance
func New(options *types.CrawlerOptions) (*HeadlessEngine, error) {
	return NewWithClients(options, nil, nil)
}

// New returns a new crawler instance
func NewWithClients(options *types.CrawlerOptions, dialer *fastdialer.Dialer, httpClient *retryablehttp.Client) (*HeadlessEngine, error) {
	browser := rod.New()
	if err := browser.Connect(); err != nil {
		return nil, err
	}

	headlessEngine := &HeadlessEngine{
		options:    options,
		dialer:     dialer,
		httpclient: httpClient,
		browser:    browser,
		networkMap: make(map[string]*NetworkPair),
	}

	if err := headlessEngine.hijackRequests(); err != nil {
		return nil, err
	}

	return headlessEngine, nil
}

// Close closes the crawler process
func (headlessEngine *HeadlessEngine) Close() {
	headlessEngine.dialer.Close()
	_ = headlessEngine.router.Stop()
	_ = headlessEngine.browser.Close()
}

func (headlessEngine *HeadlessEngine) hijackRequests() error {
	router := headlessEngine.browser.HijackRequests()
	err := router.Add("*", "", func(ctx *rod.Hijack) {
		for k, v := range headlessEngine.options.Options.CustomHeaders.AsMap() {
			ctx.Request.Req().Header.Set(k, v.(string))
		}

		err := ctx.LoadResponse(headlessEngine.httpclient.HTTPClient, true)
		if err != nil {
			return
		}

		requestURL, _ := urlutil.Parse(ctx.Request.URL().String())

		headlessEngine.networkMap[requestURL.String()] = &NetworkPair{
			Request: ctx.Request.Req(),
			Response: &http.Response{
				Header:     ctx.Response.Headers(),
				Body:       io.NopCloser(strings.NewReader(ctx.Response.Body())),
				StatusCode: ctx.Response.Payload().ResponseCode,
				Status:     ctx.Response.Payload().ResponsePhrase,
				Request:    ctx.Request.Req(),
			},
		}
	})
	if err != nil {
		return err
	}

	headlessEngine.router = router

	go headlessEngine.router.Run()

	return nil
}
