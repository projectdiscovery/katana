package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxBodyCapture bounds how much of a request body is read to learn its parameter names.
const maxBodyCapture = 1 << 20

// hit is one request as the target server received it.
type hit struct {
	Method string
	Path   string
	Params []string
	// BodyParams are the parameter names from the request body. Query names stay in Params.
	BodyParams []string
	Body       bool
	Status     int
}

// recorder is a reverse proxy in front of a target that records every request,
// so coverage is measured from the server side rather than from katana's output.
type recorder struct {
	target *url.URL
	proxy  *httputil.ReverseProxy
	// fallback is the page the app serves for paths it does not have, if it answers them with 200.
	fallback     []byte
	fallbackETag string
	mu           sync.Mutex
	hits         []hit
}

func newRecorder(target *url.URL) *recorder {
	r := &recorder{target: target}
	r.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			// keep the proxy host so absolute links and redirects the app builds point back through the proxy
			pr.Out.Host = pr.In.Host
			// the transport then negotiates and decodes compression itself, so bodies compare byte for byte
			pr.Out.Header.Del("Accept-Encoding")
		},
		// recorded here rather than after ServeHTTP, which for a websocket returns only when the socket closes
		ModifyResponse: func(resp *http.Response) error {
			rewriteSameOriginRedirect(resp, target)
			status := resp.StatusCode
			if r.isFallback(resp) {
				// a single page app serving its shell for an unknown path is a 404 in all but status
				status = http.StatusNotFound
			}
			r.record(resp.Request, status)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			status := http.StatusBadGateway
			if errors.Is(err, context.Canceled) {
				// katana gave up on the request, which says nothing about the server
				status = statusClientClosed
			}
			r.record(req, status)
			w.WriteHeader(status)
		},
	}
	return r
}

// rewriteSameOriginRedirect turns an absolute redirect back to the target into a
// relative one, so katana follows it through the recorder instead of the upstream.
func rewriteSameOriginRedirect(resp *http.Response, target *url.URL) {
	loc := resp.Header.Get("Location")
	if loc == "" {
		return
	}
	redirect, err := url.Parse(loc)
	if err != nil || !redirect.IsAbs() || redirect.Scheme != target.Scheme || redirect.Host != target.Host {
		return
	}
	if redirect.Path == "" {
		redirect.Path = "/"
	}
	redirect.Scheme = ""
	redirect.Host = ""
	redirect.User = nil
	resp.Header.Set("Location", redirect.String())
}

// calibrate learns the app's fallback page by requesting a path that cannot exist.
func (r *recorder) calibrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.target.JoinPath("crawl-bench-"+strconv.FormatInt(time.Now().UnixNano(), 36)).String(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyCapture))
	if err != nil {
		return err
	}
	r.fallback = body
	r.fallbackETag = resp.Header.Get("ETag")
	return nil
}

func (r *recorder) isFallback(resp *http.Response) bool {
	if len(r.fallback) == 0 {
		return false
	}
	if resp.StatusCode == http.StatusNotModified {
		// a browser revalidating the fallback it already got
		return r.fallbackETag != "" && resp.Header.Get("ETag") == r.fallbackETag
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	if resp.ContentLength >= 0 && resp.ContentLength != int64(len(r.fallback)) {
		return false
	}
	// read one byte past the fallback size so a longer body never matches
	head, _ := io.ReadAll(io.LimitReader(resp.Body, int64(len(r.fallback))+1))
	resp.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(head), resp.Body), resp.Body}
	return bytes.Equal(head, r.fallback)
}

// statusClientClosed follows nginx's convention for a client that left before the response.
const statusClientClosed = 499

type hitKey struct{}

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h := &hit{Method: req.Method, Path: req.URL.Path, Params: queryKeys(req.URL.Query())}
	if req.Body != nil && req.ContentLength != 0 {
		body, _ := io.ReadAll(io.LimitReader(req.Body, maxBodyCapture))
		req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), req.Body))
		if len(body) > 0 {
			h.Body = true
			h.BodyParams = bodyKeys(req.Header.Get("Content-Type"), body)
			h.Params = append(h.Params, h.BodyParams...)
		}
	}
	r.proxy.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), hitKey{}, h)))
}

func (r *recorder) record(req *http.Request, status int) {
	h, ok := req.Context().Value(hitKey{}).(*hit)
	if !ok {
		return
	}
	h.Status = status
	r.mu.Lock()
	r.hits = append(r.hits, *h)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []hit {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]hit(nil), r.hits...)
}

func queryKeys(values url.Values) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	return keys
}

func bodyKeys(contentType string, body []byte) []string {
	switch {
	case strings.Contains(contentType, "json"):
		var obj map[string]json.RawMessage
		if json.Unmarshal(body, &obj) != nil {
			return nil
		}
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	case strings.Contains(contentType, "multipart/form-data"):
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return nil
		}
		var keys []string
		mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				return keys
			}
			keys = append(keys, part.FormName())
		}
	default:
		values, err := url.ParseQuery(string(body))
		if err != nil {
			return nil
		}
		return queryKeys(values)
	}
}
