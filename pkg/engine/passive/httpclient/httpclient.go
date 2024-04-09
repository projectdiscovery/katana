package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/useragent"
)

type HttpClient struct {
	Client *http.Client
}

type BasicAuth struct {
	Username string
	Password string
}

func NewHttpClient(timeout int) *HttpClient {
	Transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Dial: (&net.Dialer{
			Timeout: time.Duration(timeout) * time.Second,
		}).Dial,
	}

	client := &http.Client{
		Transport: Transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}

	httpClient := &HttpClient{Client: client}

	return httpClient
}

func (hc *HttpClient) Get(ctx context.Context, getURL, cookies string, headers map[string]string) (*http.Response, error) {
	return hc.HTTPRequest(ctx, http.MethodGet, getURL, cookies, headers, nil, BasicAuth{})
}

func (hc *HttpClient) SimpleGet(ctx context.Context, getURL string) (*http.Response, error) {
	return hc.HTTPRequest(ctx, http.MethodGet, getURL, "", map[string]string{}, nil, BasicAuth{})
}

func (hc *HttpClient) Post(ctx context.Context, postURL, cookies string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return hc.HTTPRequest(ctx, http.MethodPost, postURL, cookies, headers, body, BasicAuth{})
}

func (hc *HttpClient) SimplePost(ctx context.Context, postURL, contentType string, body io.Reader) (*http.Response, error) {
	return hc.HTTPRequest(ctx, http.MethodPost, postURL, "", map[string]string{"Content-Type": contentType}, body, BasicAuth{})
}

func (hc *HttpClient) HTTPRequest(ctx context.Context, method, requestURL, cookies string, headers map[string]string, body io.Reader, basicAuth BasicAuth) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, err
	}

	userAgent := useragent.PickRandom()
	req.Header.Set("User-Agent", userAgent.String())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("Connection", "close")

	if basicAuth.Username != "" || basicAuth.Password != "" {
		req.SetBasicAuth(basicAuth.Username, basicAuth.Password)
	}

	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return httpRequestWrapper(hc.Client, req)
}

func (hc *HttpClient) DiscardHTTPResponse(response *http.Response) {
	if response != nil {
		_, err := io.Copy(io.Discard, response.Body)
		if err != nil {
			gologger.Warning().Msgf("Could not discard response body: %s\n", err)
			return
		}
		response.Body.Close()
	}
}

func (hc *HttpClient) Close() {
	hc.Client.CloseIdleConnections()
}

func httpRequestWrapper(client *http.Client, request *http.Request) (*http.Response, error) {
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		requestURL, _ := url.QueryUnescape(request.URL.String())

		gologger.Debug().MsgFunc(func() string {
			buffer := new(bytes.Buffer)
			_, _ = buffer.ReadFrom(response.Body)
			return fmt.Sprintf("Response for failed request against %s:\n%s", requestURL, buffer.String())
		})
		return response, fmt.Errorf("unexpected status code %d received from %s", response.StatusCode, requestURL)
	}
	return response, nil
}
