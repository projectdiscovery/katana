package crawler

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/engine/headless/assets"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
)

// CollectDiscoveredScripts follows literal import references using Chrome's
// credential-aware resource loader. Each stream is read to EOF before being
// marked complete. A failure is written to the manifest and exploration of
// other references continues.
func (c *Crawler) CollectDiscoveredScripts(ctx context.Context, store *assets.Store) error {
	page, err := c.launcher.GetPageFromPool()
	if err != nil {
		return err
	}
	originalPage := page.Page
	defer func() {
		page.Page = originalPage
		c.launcher.PutBrowserToPool(page)
	}()
	page.Page = page.Context(ctx)
	if err := (proto.NetworkEnable{}).Call(page.Page); err != nil {
		return err
	}
	var firstErr error
	fetched, skipped, limitSkipped := 0, 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		item, ok := store.PopDiscovery()
		if !ok {
			break
		}
		if item.Source != "recovered" && c.options.ScriptDiscoveryScope != "all" && !sameScriptOrigin(item.Parent, item.URL) {
			skipped++
			continue
		}
		if limit := c.options.ScriptDiscoveryMax; item.Source != "recovered" && limit > 0 && fetched >= limit {
			limitSkipped++
			continue
		}
		if item.Source != "recovered" {
			fetched++
		}
		response := fetchDiscoveredScript(page, item)
		if _, err := store.Save(response); err != nil && firstErr == nil {
			firstErr = err
		}
		if response.Err != nil && firstErr == nil {
			firstErr = response.Err
		}
	}
	if skipped > 0 {
		c.logger.Info("static scripts outside discovery scope skipped", "count", skipped)
	}
	if limitSkipped > 0 {
		c.logger.Info("static script discovery request limit reached", "skipped", limitSkipped, "limit", c.options.ScriptDiscoveryMax)
	}
	return firstErr
}

func sameScriptOrigin(parent, child string) bool {
	a, errA := url.Parse(parent)
	b, errB := url.Parse(child)
	return errA == nil && errB == nil && strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func fetchDiscoveredScript(page *browser.BrowserPage, item assets.Discovery) assets.Response {
	source := item.Source
	if source == "" {
		source = "discovered"
	}
	response := assets.Response{URL: item.URL, ParentURL: item.Parent, Source: source, Depth: item.Depth}
	call := proto.NetworkLoadNetworkResource{
		FrameID: page.FrameID,
		URL:     item.URL,
		Options: &proto.NetworkLoadNetworkResourceOptions{IncludeCredentials: true, DisableCache: item.Source == "recovered"},
	}
	res, err := call.Call(page.Page)
	if err != nil {
		response.Err = err
		return response
	}
	if res.Resource == nil {
		response.Err = fmt.Errorf("Chrome did not return a resource result")
		return response
	}
	resource := res.Resource
	if resource.HTTPStatusCode != nil {
		response.Status = int(*resource.HTTPStatusCode)
	}
	if !resource.Success || resource.Stream == "" {
		response.Err = fmt.Errorf("Chrome did not return a resource stream (status=%d, net_error=%s)", response.Status, resource.NetErrorName)
		return response
	}
	for name, value := range resource.Headers {
		switch {
		case strings.EqualFold(name, "content-type"):
			response.MIME = value.Str()
		case strings.EqualFold(name, "content-length"):
			response.ContentLength = value.Str()
		case strings.EqualFold(name, "content-encoding"):
			response.ContentEncoding = value.Str()
		}
	}
	defer func() { _ = (proto.IOClose{Handle: resource.Stream}).Call(page.Page) }()
	var body bytes.Buffer
	for {
		read, err := (proto.IORead{Handle: resource.Stream}).Call(page.Page)
		if err != nil {
			response.Err = err
			return response
		}
		var chunk []byte
		if read.Base64Encoded {
			chunk, err = base64.StdEncoding.DecodeString(read.Data)
			if err != nil {
				response.Err = err
				return response
			}
		} else {
			chunk = []byte(read.Data)
		}
		if _, err := body.Write(chunk); err != nil {
			response.Err = err
			return response
		}
		if read.EOF {
			break
		}
	}
	response.Body = body.Bytes()
	return response
}
