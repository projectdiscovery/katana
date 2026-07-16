// Package endpoints is a knowledgebase.Extractor that classifies a crawled
// request as an API endpoint (REST, GraphQL, SOAP, AJAX/XHR) by inspecting
// method, URL, headers, and content types. Emits per-response, one entry
// per qualifying request.
//
// Output schema (all values strings unless noted):
//
//	class        : "rest" | "graphql" | "soap" | "xhr"
//	method       : uppercase HTTP verb
//	url          : request URL with userinfo and fragment stripped
//	content_type : lowercased media type without parameters (optional)
//	auth         : lowercased auth scheme, e.g. "bearer" / "basic" (optional)
//	params       : []string of query parameter names in first-seen order (optional)
//
// The extractor is read-only with respect to its inputs: it MUST NOT mutate
// req or resp (headers, body, URL).
package endpoints

import (
	"net/http"
	"net/url"
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
	urlStr := safeURLString(req.URL)
	pathLower := ""
	if req.URL != nil {
		pathLower = strings.ToLower(req.URL.Path)
	}
	reqCT := req.Header.Get("Content-Type")
	respCT := ""
	if resp != nil {
		respCT = resp.Header.Get("Content-Type")
	}
	soapAction := req.Header.Get("SOAPAction")

	class := classify(method, pathLower, reqCT, respCT, soapAction)
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
	if scheme := authScheme(req.Header.Get("Authorization")); scheme != "" {
		out["auth"] = scheme
	}
	if req.URL != nil {
		if params := paramNames(req.URL.RawQuery); len(params) > 0 {
			out["params"] = params
		}
	}
	return out
}

// classify decides which endpoint family a request belongs to. Decision tree
// (first match wins):
//
//  1. SOAPAction header or soap+xml content-type on either side -> "soap"
//  2. "graphql" path component or application/graphql content-type -> "graphql"
//  3. JSON/XML body and (mutating verb OR API-looking path)       -> "rest"
//  4. JSON GET to a non-API path                                  -> "xhr"
//  5. form-urlencoded or multipart on a mutating verb             -> "rest"
//  6. otherwise                                                   -> "" (skip)
//
// pathLower is the lowercased URL path (no scheme/host/query) so that
// substring/segment checks aren't fooled by hostnames or query params.
func classify(method, pathLower, reqCT, respCT, soapAction string) string {
	if soapAction != "" || containsAny(reqCT, "soap+xml") || containsAny(respCT, "soap+xml") {
		return "soap"
	}
	if hasPathSegment(pathLower, "graphql") ||
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

// apiPathSegments are substrings whose presence in the URL path strongly
// suggests an API endpoint. Kept intentionally generous; the classifier still
// requires a structured content-type or a mutating verb to actually return
// "rest", so individual false positives don't surface as endpoints on their
// own.
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

// hasPathSegment reports whether name appears as a full "/"-separated segment
// of pathLower (e.g. "/graphql", "/api/graphql", but not "/notgraphql/blob").
func hasPathSegment(pathLower, name string) bool {
	for _, seg := range strings.Split(pathLower, "/") {
		if seg == name {
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

// authScheme returns the lowercased scheme of an Authorization header (the
// token before the first space, e.g. "bearer" or "basic"). Returns "" when
// the header is empty or doesn't follow the "<scheme> <credentials>" shape,
// in which case the auth field is omitted from the output to avoid emitting
// noise for non-RFC headers.
func authScheme(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	if i := strings.Index(authHeader, " "); i > 0 {
		return strings.ToLower(authHeader[:i])
	}
	return ""
}

// paramNames returns the deduplicated names of query parameters in first-seen
// order. Dedup is intentionally case-sensitive: "Q" and "q" are kept as two
// names because most APIs treat parameter casing as significant.
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

// safeURLString returns u as a string with userinfo and fragment stripped so
// credentials embedded in the URL never reach knowledgebase output.
func safeURLString(u *url.URL) string {
	if u == nil {
		return ""
	}
	clone := *u
	clone.User = nil
	clone.Fragment = ""
	clone.RawFragment = ""
	return clone.String()
}
