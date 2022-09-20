package standard

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
)

// navigationRequest is a navigation request for the crawler
type navigationRequest struct {
	Method    string
	URL       string
	Body      string
	Depth     int
	Headers   map[string]string
	Tag       string
	Attribute string
	Source    string // source is the source of the request
}

// makeRequest makes a request to a URL returning a response interface.
func (c *Crawler) makeRequest(ctx context.Context, request navigationRequest) (navigationResponse, error) {
	response := navigationResponse{
		Depth:   request.Depth + 1,
		options: c.options,
	}
	httpReq, err := http.NewRequestWithContext(ctx, request.Method, request.URL, nil)
	if err != nil {
		return response, err
	}
	if request.Body != "" {
		httpReq.Body = ioutil.NopCloser(strings.NewReader(request.Body))
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

	resp, err := c.httpclient.Do(req)
	if resp != nil {
		defer func() {
			if resp.Body != nil && resp.StatusCode != http.StatusSwitchingProtocols {
				_, _ = io.CopyN(ioutil.Discard, resp.Body, 8*1024)
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
	data, err := ioutil.ReadAll(limitReader)
	if err != nil {
		return response, err
	}
	response.Body = data
	response.Resp = resp
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return response, errors.Wrap(err, "could not make document from reader")
	}
	return response, nil
}

// RequestURL returns the request URL for the navigation
func (n *navigationRequest) RequestURL() string {
	switch n.Method {
	case "GET":
		return n.URL
	case "POST":
		builder := &strings.Builder{}
		builder.WriteString(n.URL)
		builder.WriteString(":")
		builder.WriteString(n.Body)
		builtURL := builder.String()
		return builtURL
	}
	return ""
}

// newNavigationRequestURL generates a navigation request from a relative URL
func newNavigationRequestURL(path, source, tag, attribute string, resp navigationResponse) navigationRequest {
	requestURL := resp.AbsoluteURL(path)
	return navigationRequest{Method: "GET", URL: requestURL, Depth: resp.Depth, Source: source, Attribute: attribute, Tag: tag}
}
