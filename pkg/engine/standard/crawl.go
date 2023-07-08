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
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
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

	rawRequestBytes, _ := req.Dump()
	request.Raw = string(rawRequestBytes)

	if err != nil {
		return response, err
	}
	if resp.StatusCode == http.StatusSwitchingProtocols {
		return response, nil
	}
	limitReader := io.LimitReader(resp.Body, int64(c.Options.Options.BodyReadSize))
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return response, err
	}
	if !c.Options.UniqueFilter.UniqueContent(data) {
		return &navigation.Response{}, nil
	}

	technologies := c.Options.Wappalyzer.Fingerprint(resp.Header, data)
	response.Technologies = mapsutil.GetKeys(technologies)

	resp.Body = io.NopCloser(strings.NewReader(string(data)))

	response.Body = string(data)
	response.Resp = resp
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(data))
	response.Reader.Url, _ = url.Parse(request.URL)
	response.StatusCode = resp.StatusCode
	response.Headers = utils.FlattenHeaders(resp.Header)
	if c.Options.Options.FormExtraction {
		response.Forms = append(response.Forms, utils.ParseFormFields(response.Reader)...)
	}

	resp.ContentLength = int64(len(data))

	rawResponseBytes, _ := httputil.DumpResponse(resp, true)
	response.Raw = string(rawResponseBytes)

	if err != nil {
		return response, errorutil.NewWithTag("standard", "could not make document from reader").Wrap(err)
	}

	return response, nil
}
