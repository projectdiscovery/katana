package endpoints

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func mustRequest(t *testing.T, method, rawURL string, headers map[string]string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	req := &http.Request{Method: method, URL: u, Header: http.Header{}}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func responseWithCT(ct string) *http.Response {
	h := http.Header{}
	if ct != "" {
		h.Set("Content-Type", ct)
	}
	return &http.Response{Header: h}
}

func TestExtract_GraphQLByPath(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://example.com/graphql", map[string]string{
		"Content-Type": "application/json",
	})
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil {
		t.Fatal("expected entry, got nil")
	}
	if out["class"] != "graphql" {
		t.Errorf("class = %v, want graphql", out["class"])
	}
}

func TestExtract_SOAPByHeader(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://example.com/ws/UserService", map[string]string{
		"Content-Type": "text/xml; charset=utf-8",
		"SOAPAction":   "GetUser",
	})
	out := e.Extract("", req, nil)
	if out == nil || out["class"] != "soap" {
		t.Fatalf("class = %v, want soap (out=%#v)", out["class"], out)
	}
}

func TestExtract_RESTByJSONMutating(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://api.example.com/users", map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer eyJhbGc",
	})
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil || out["class"] != "rest" {
		t.Fatalf("class = %v, want rest (out=%#v)", out["class"], out)
	}
	if out["auth"] != "bearer" {
		t.Errorf("auth = %v, want bearer", out["auth"])
	}
	if out["content_type"] != "application/json" {
		t.Errorf("content_type = %v, want application/json", out["content_type"])
	}
}

func TestExtract_RESTByAPIPath(t *testing.T) {
	e := New()
	req := mustRequest(t, "GET", "https://example.com/api/v1/users?q=alice&limit=10", nil)
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil || out["class"] != "rest" {
		t.Fatalf("class = %v, want rest (out=%#v)", out["class"], out)
	}
	params, _ := out["params"].([]string)
	want := []string{"q", "limit"}
	if !reflect.DeepEqual(params, want) {
		t.Errorf("params = %v, want %v", params, want)
	}
}

func TestExtract_XHRJSONGet(t *testing.T) {
	e := New()
	req := mustRequest(t, "GET", "https://example.com/feed", nil)
	out := e.Extract("", req, responseWithCT("application/json; charset=utf-8"))
	if out == nil || out["class"] != "xhr" {
		t.Fatalf("class = %v, want xhr (out=%#v)", out["class"], out)
	}
}

func TestExtract_FormPost(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://example.com/submit", map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})
	out := e.Extract("", req, nil)
	if out == nil || out["class"] != "rest" {
		t.Fatalf("class = %v, want rest (out=%#v)", out["class"], out)
	}
}

func TestExtract_PlainHTMLIgnored(t *testing.T) {
	e := New()
	req := mustRequest(t, "GET", "https://example.com/about.html", nil)
	out := e.Extract("", req, responseWithCT("text/html"))
	if out != nil {
		t.Errorf("expected nil for plain HTML, got %#v", out)
	}
}

func TestExtract_NilRequest(t *testing.T) {
	e := New()
	if got := e.Extract("", nil, nil); got != nil {
		t.Errorf("expected nil for nil request, got %#v", got)
	}
}

func TestExtract_BasicAuthScheme(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://api.example.com/v1/login", map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Basic dXNlcjpwYXNz",
	})
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil {
		t.Fatal("expected entry, got nil")
	}
	if out["auth"] != "basic" {
		t.Errorf("auth = %v, want basic", out["auth"])
	}
}

func TestExtract_AuthSchemeUnknownOmitted(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://api.example.com/v1/login", map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "weird-no-space-token",
	})
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil {
		t.Fatal("expected entry, got nil")
	}
	if _, ok := out["auth"]; ok {
		t.Errorf("expected auth field to be omitted for malformed header, got %#v", out["auth"])
	}
}

func TestExtract_SOAPByContentTypeOnly(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://example.com/svc", map[string]string{
		"Content-Type": "application/soap+xml; charset=utf-8",
	})
	out := e.Extract("", req, nil)
	if out == nil || out["class"] != "soap" {
		t.Fatalf("class = %v, want soap (out=%#v)", out["class"], out)
	}
	if out["content_type"] != "application/soap+xml" {
		t.Errorf("content_type = %v, want application/soap+xml", out["content_type"])
	}
}

func TestExtract_CaseInsensitiveContentType(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://api.example.com/v1/users", map[string]string{
		"Content-Type": "Application/JSON; charset=UTF-8",
	})
	out := e.Extract("", req, responseWithCT("Application/JSON"))
	if out == nil || out["class"] != "rest" {
		t.Fatalf("class = %v, want rest (out=%#v)", out["class"], out)
	}
	if out["content_type"] != "application/json" {
		t.Errorf("content_type = %v, want application/json", out["content_type"])
	}
}

func TestExtract_GraphQLByContentType(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://example.com/query", map[string]string{
		"Content-Type": "application/graphql",
	})
	out := e.Extract("", req, nil)
	if out == nil || out["class"] != "graphql" {
		t.Fatalf("class = %v, want graphql (out=%#v)", out["class"], out)
	}
}

func TestExtract_GraphQLFalsePositivePath(t *testing.T) {
	// "/notgraphql/blob" must not be classified as graphql; the request also
	// lacks any API-shaped content type, so it should be ignored entirely.
	e := New()
	req := mustRequest(t, "GET", "https://example.com/notgraphql/blob", map[string]string{
		"Content-Type": "text/html",
	})
	out := e.Extract("", req, responseWithCT("text/html"))
	if out != nil {
		t.Errorf("expected nil for /notgraphql/blob, got %#v", out)
	}
}

func TestExtract_GraphQLNestedPathSegment(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://example.com/api/graphql", map[string]string{
		"Content-Type": "application/json",
	})
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil || out["class"] != "graphql" {
		t.Fatalf("class = %v, want graphql (out=%#v)", out["class"], out)
	}
}

func TestExtract_URLStripsUserinfoAndFragment(t *testing.T) {
	e := New()
	req := mustRequest(t, "POST", "https://user:pass@api.example.com/v1/users?token=abc#frag", map[string]string{
		"Content-Type": "application/json",
	})
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil {
		t.Fatal("expected entry, got nil")
	}
	got, _ := out["url"].(string)
	if strings.Contains(got, "user:pass") {
		t.Errorf("url leaks userinfo: %q", got)
	}
	if strings.Contains(got, "#frag") {
		t.Errorf("url leaks fragment: %q", got)
	}
	if want := "https://api.example.com/v1/users?token=abc"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
}

func TestExtract_DedupParamsCaseSensitive(t *testing.T) {
	e := New()
	req := mustRequest(t, "GET", "https://api.example.com/v1/items?Q=1&q=2&Q=3", nil)
	out := e.Extract("", req, responseWithCT("application/json"))
	if out == nil {
		t.Fatal("expected entry, got nil")
	}
	params, _ := out["params"].([]string)
	want := []string{"Q", "q"}
	if !reflect.DeepEqual(params, want) {
		t.Errorf("params = %v, want %v", params, want)
	}
}

func TestName(t *testing.T) {
	if got := New().Name(); got != Name {
		t.Errorf("Name() = %q, want %q", got, Name)
	}
}
