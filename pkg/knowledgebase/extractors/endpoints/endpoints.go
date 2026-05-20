// Package endpoints is a knowledgebase.Extractor that classifies a crawled
// request as an API endpoint (REST, GraphQL, SOAP, AJAX/XHR) by inspecting
// method, URL, headers, and content types. Emits per-response, one entry
// per qualifying request.
package endpoints

import (
	"net/http"
	"strings"
)

const Name = "endpoints"

type Extractor struct{}

func New() *Extractor { return &Extractor{} }

func (e *Extractor) Name() string { return Name }

func (e *Extractor) Extract(_ string, req *http.Request, resp *http.Response) map[string]any {
	if req == nil {
		return nil
	}

	method := strings.ToUpper(req.Method)
	urlStr := ""
	if req.URL != nil {
		urlStr = req.URL.String()
	}
	reqCT := req.Header.Get("Content-Type")
	respCT := ""
	if resp != nil {
		respCT = resp.Header.Get("Content-Type")
	}
	soapAction := req.Header.Get("SOAPAction")
	hasAuth := req.Header.Get("Authorization") != ""

	class := classify(method, urlStr, reqCT, respCT, soapAction)
	if class == "" {
		return nil
	}

	out := map[string]any{
		"class":  class,
		"method": method,
		"url":    urlStr,
	}
	if ct := primaryContentType(respCT, reqCT); ct != "" {
		out["content_type"] = ct
	}
	if hasAuth {
		out["auth"] = authScheme(req.Header.Get("Authorization"))
	}
	if req.URL != nil {
		if params := paramNames(req.URL.RawQuery); len(params) > 0 {
			out["params"] = params
		}
	}
	return out
}

func classify(method, urlStr, reqCT, respCT, soapAction string) string {
	pathLower := strings.ToLower(urlStr)

	if soapAction != "" || containsAny(reqCT, "soap+xml") || containsAny(respCT, "soap+xml") {
		return "soap"
	}
	if strings.Contains(pathLower, "/graphql") ||
		containsAny(reqCT, "application/graphql") ||
		containsAny(respCT, "application/graphql") {
		return "graphql"
	}

	isJSON := containsAny(reqCT, "application/json") || containsAny(respCT, "application/json")
	isXML := containsAny(reqCT, "application/xml") || containsAny(respCT, "application/xml")
	isForm := containsAny(reqCT, "application/x-www-form-urlencoded") || containsAny(reqCT, "multipart/form-data")

	apiPath := matchesAPIPath(pathLower)
	mutating := isMutatingVerb(method)

	if (isJSON || isXML) && (mutating || apiPath) {
		return "rest"
	}
	if isJSON && method == "GET" {
		return "xhr"
	}
	if isForm && mutating {
		return "rest"
	}
	return ""
}

var apiPathSegments = []string{
	"/api/", "/v1/", "/v2/", "/v3/", "/rest/", "/rpc/",
	"/jsonrpc", "/.well-known/", "/oauth/", "/openapi",
}

func matchesAPIPath(pathLower string) bool {
	for _, seg := range apiPathSegments {
		if strings.Contains(pathLower, seg) {
			return true
		}
	}
	return false
}

func isMutatingVerb(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

func containsAny(haystack, needle string) bool {
	if haystack == "" {
		return false
	}
	return strings.Contains(strings.ToLower(haystack), needle)
}

func primaryContentType(respCT, reqCT string) string {
	pick := respCT
	if pick == "" {
		pick = reqCT
	}
	if pick == "" {
		return ""
	}
	if i := strings.Index(pick, ";"); i >= 0 {
		pick = pick[:i]
	}
	return strings.TrimSpace(strings.ToLower(pick))
}

func authScheme(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	if i := strings.Index(authHeader, " "); i > 0 {
		return strings.ToLower(authHeader[:i])
	}
	return "unknown"
}

func paramNames(rawQuery string) []string {
	if rawQuery == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, kv := range strings.Split(rawQuery, "&") {
		if kv == "" {
			continue
		}
		name := kv
		if i := strings.Index(kv, "="); i >= 0 {
			name = kv[:i]
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}
