package browser

import (
	"net/url"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// applyBrowserHeaders seeds Cookie via the browser cookie jar (scoped to
// cookieURL) and returns the remaining custom headers for per-request
// injection. Page-wide SetExtraHeaders is intentionally avoided so values
// like Authorization are not leaked to CDN or other third-party hosts.
func applyBrowserHeaders(page *rod.Page, headers map[string]string, cookieURL string) (map[string]string, error) {
	cookies, extra := splitCustomHeaders(headers, cookieURL)
	if len(cookies) > 0 {
		if err := page.SetCookies(cookies); err != nil {
			return nil, err
		}
	}
	return extra, nil
}

// splitCustomHeaders separates Cookie (jar-scoped) from other -H values.
func splitCustomHeaders(headers map[string]string, cookieURL string) ([]*proto.NetworkCookieParam, map[string]string) {
	if len(headers) == 0 {
		return nil, nil
	}
	extra := make(map[string]string)
	var cookies []*proto.NetworkCookieParam
	for name, value := range headers {
		if strings.EqualFold(name, "Cookie") {
			cookies = append(cookies, parseCookieHeader(value, cookieURL)...)
			continue
		}
		extra[name] = value
	}
	if len(extra) == 0 {
		extra = nil
	}
	return cookies, extra
}

func parseCookieHeader(raw, targetURL string) []*proto.NetworkCookieParam {
	var cookies []*proto.NetworkCookieParam
	for _, pair := range strings.Split(raw, ";") {
		name, value, found := strings.Cut(strings.TrimSpace(pair), "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			continue
		}
		cookies = append(cookies, &proto.NetworkCookieParam{
			Name: name, Value: strings.TrimSpace(value), URL: targetURL, Path: "/",
		})
	}
	return cookies
}

// shouldAttachExtraHeaders reports whether non-cookie custom headers may be
// sent with reqURL. ScopeValidator is preferred; when unset, CookieURL's host
// is used so headers never default to every third-party subresource.
func shouldAttachExtraHeaders(reqURL string, validate ScopeValidator, cookieURL string) bool {
	if validate != nil {
		return validate(reqURL)
	}
	return sameHostname(cookieURL, reqURL)
}

func sameHostname(baseURL, reqURL string) bool {
	base, err := url.Parse(baseURL)
	if err != nil || base.Hostname() == "" {
		return false
	}
	req, err := url.Parse(reqURL)
	if err != nil || req.Hostname() == "" {
		return false
	}
	return strings.EqualFold(base.Hostname(), req.Hostname())
}

// mergeRequestHeaders builds Fetch.continueRequest headers: existing request
// headers with extra overlayed. A non-nil Headers value replaces the whole set.
func mergeRequestHeaders(existing proto.NetworkHeaders, extra map[string]string) []*proto.FetchHeaderEntry {
	if len(extra) == 0 {
		return nil
	}
	out := make([]*proto.FetchHeaderEntry, 0, len(existing)+len(extra))
	seen := make(map[string]struct{}, len(extra))
	for name, value := range extra {
		out = append(out, &proto.FetchHeaderEntry{Name: name, Value: value})
		seen[strings.ToLower(name)] = struct{}{}
	}
	for name, value := range existing {
		if _, ok := seen[strings.ToLower(name)]; ok {
			continue
		}
		out = append(out, &proto.FetchHeaderEntry{Name: name, Value: value.Str()})
	}
	return out
}

// isFetchRequestStage is true for Fetch pauses before the request is sent.
func isFetchRequestStage(e *proto.FetchRequestPaused) bool {
	return e != nil && e.ResponseStatusCode == nil && e.ResponseErrorReason == ""
}
