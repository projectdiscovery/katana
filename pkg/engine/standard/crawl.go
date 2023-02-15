package standard

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	mapsutil "github.com/projectdiscovery/utils/maps"
)

// makeRequest makes a request to a URL returning a response interface.
func (c *Crawler) makeRequest(ctx context.Context, request navigation.Request, rootHostname string, depth int, httpclient *retryablehttp.Client) (navigation.Response, error) {
	response := navigation.Response{
		Depth:        request.Depth + 1,
		RootHostname: rootHostname,
	}
	ctx = context.WithValue(ctx, navigation.Depth{}, depth)
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
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := httpclient.Do(req)
	if resp != nil {
		defer func() {
			if resp.Body != nil && resp.StatusCode != http.StatusSwitchingProtocols {
				_, _ = io.CopyN(io.Discard, resp.Body, 8*1024)
			}
			_ = resp.Body.Close()
		}()
	}
	if err != nil {
		return response, err
	}
	if resp.StatusCode == http.StatusSwitchingProtocols {
		return response, nil
	}
	limitReader := io.LimitReader(resp.Body, int64(c.options.Options.BodyReadSize))
	data, err := io.ReadAll(limitReader)
	if err != nil {
		return response, err
	}
	if !c.options.UniqueFilter.UniqueContent(data) {
		return navigation.Response{}, nil
	}

	technologies := c.options.Wappalyzer.Fingerprint(resp.Header, data)
	response.Technologies = mapsutil.GetKeys(technologies)

	resp.Body = io.NopCloser(strings.NewReader(string(data)))

	response.Body = string(data)
	response.Resp = resp
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return response, errorutil.NewWithTag("standard", "could not make document from reader").Wrap(err)
	}
	return response, nil
}
