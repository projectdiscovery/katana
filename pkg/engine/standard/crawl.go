package standard

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	mapsutil "github.com/projectdiscovery/utils/maps"
)

// makeRequest makes a request to a URL returning a response interface.
func (c *Crawler) makeRequest(s *common.CrawlSession, request *navigation.Request) (*navigation.Response, error) {
	response := &navigation.Response{
		Depth:        request.Depth + 1,
		RootHostname: s.Hostname,
	}
	ctx := context.WithValue(s.Ctx, navigation.Depth{}, request.Depth)
	httpReq, err := http.NewRequestWithContext(ctx, request.Method, request.URL, nil)
	if err != nil {
		return response, err
	}
	if request.Body != "" && request.Method != "GET" {
		httpReq.Body = io.NopCloser(strings.NewReader(request.Body))
	}
	req, err := retryablehttp.FromRequest(httpReq)
	if err != nil {
		return response, err
	}
	req.Header.Set("User-Agent", utils.WebUserAgent())

	// Set the headers for the request.
	for k, v := range request.Headers {
		req.Header.Set(k, v)
		if k == "Host" {
			req.Host = v
		}
	}

	for k, v := range c.Headers {
		req.Header.Set(k, v)
		if k == "Host" {
			req.Host = v
		}
	}

	// Apply cookies
	if c.Shared.Jar != nil {
		cookies := c.Shared.Jar.Cookies(req.Request.URL)
		for _, cookie := range cookies {
			req.Request.AddCookie(cookie)
		}
	}

	resp, err := s.HttpClient.Do(req)
	if resp != nil {
		defer func() {
			if resp.Body != nil && resp.StatusCode != http.StatusSwitchingProtocols {
				_, _ = io.Copy(io.Discard, resp.Body)
			}
			_ = resp.Body.Close()
		}()
	}

	// Collect cookies from the response
	if c.Shared.Jar != nil && resp != nil {
		c.Shared.Jar.SetCookies(req.Request.URL, resp.Cookies())
	}

	rawRequestBytes, _ := req.Dump()
	request.Raw = string(rawRequestBytes)

	if err != nil {
		return response, err
	}

	// If the response is empty, perform a defensive return.
	if resp == nil {
		return response, errorutil.NewWithTag("standard", "nil response from http client")
	}

	if resp.StatusCode == http.StatusSwitchingProtocols {
		return response, nil
	}

	limitReader := io.LimitReader(resp.Body, int64(c.Options.Options.BodyReadSize))
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return response, err
	}
	// Skip unique content filtering if disabled
	if !c.Options.Options.DisableUniqueFilter {
		if !c.Options.UniqueFilter.UniqueContent(data) {
			return &navigation.Response{}, nil
		}
	}

	if c.Options.Wappalyzer != nil {
		technologies := c.Options.Wappalyzer.Fingerprint(resp.Header, data)
		response.Technologies = mapsutil.GetKeys(technologies)
	}

	// Restore the read data to resp.Body for further use.
	resp.Body = io.NopCloser(strings.NewReader(string(data)))

	response.Body = string(data)
	response.Resp = resp

	// First, attempt to parse using goquery. If the parsing fails, simply return safely (to avoid accessing response.Reader before checking err).
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		// Even if the parsing fails, try to attach the original response for debugging purposes.
		rawResponseBytes, _ := httputil.DumpResponse(resp, true)
		response.Raw = string(rawResponseBytes)
		return response, errorutil.NewWithTag("standard", "could not make document from reader").Wrap(err)
	}
	// Only set the `response.Reader` and its URL after the parsing is successful to avoid accessing a nil pointer.
	response.Reader = doc
	response.Reader.Url, _ = url.Parse(request.URL)

	response.StatusCode = resp.StatusCode
	response.Headers = utils.FlattenHeaders(resp.Header)
	if c.Options.Options.FormExtraction {
		response.Forms = append(response.Forms, utils.ParseFormFields(response.Reader)...)
	}

	// Use the actual length of the read data as ContentLength
	resp.ContentLength = int64(len(data))
	response.ContentLength = resp.ContentLength

	rawResponseBytes, _ := httputil.DumpResponse(resp, true)
	response.Raw = string(rawResponseBytes)

	return response, nil
}
