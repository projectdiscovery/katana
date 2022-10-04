package hybrid

import (
	"bytes"
	"context"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
)

func (c *Crawler) navigateRequest(ctx context.Context, queue *queue.VarietyQueue, parseResponseCallback func(nr navigation.Request), browser *rod.Browser, request navigation.Request) (*navigation.Response, error) {
	depth := request.Depth + 1
	response := &navigation.Response{
		Depth:   depth,
		Options: c.options,
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, err
	}
	defer page.Close()

	pageRouter := page.HijackRequests()
	if err := pageRouter.Add("*", "", c.makeRoutingHandler(queue, depth, parseResponseCallback)); err != nil {
		return nil, err
	}
	go pageRouter.Run()
	defer func() {
		if err := pageRouter.Stop(); err != nil {
			gologger.Warning().Msgf("%s\n", err)
		}
	}()

	if err := page.Navigate(request.URL); err != nil {
		return nil, err
	}

	timeout := time.Duration(c.options.Options.Timeout * int(time.Second))

	// wait the page to be fully loaded and becoming idle
	page.Timeout(timeout).WaitNavigation(proto.PageLifecycleEventNameDOMContentLoaded)

	// Wait for the window.onload event
	if err := page.WaitLoad(); err != nil {
		return nil, err
	}

	// wait for idle the network requests
	if err := page.WaitIdle(timeout); err != nil {
		return nil, err
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
