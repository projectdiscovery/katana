package hybrid

import (
	"bytes"
	"context"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/navigation"
)

func (c *Crawler) navigateRequest(ctx context.Context, browser *rod.Browser, request navigation.Request) (*navigation.Response, error) {
	response := &navigation.Response{
		Depth:   request.Depth + 1,
		Options: c.options,
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: request.URL})
	if err != nil {
		return nil, err
	}
	defer page.Close()

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
