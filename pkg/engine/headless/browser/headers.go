package browser

import (
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// applyBrowserHeaders uses browser cookies for Cookie instead of a global
// extra header, which would otherwise expose the seed cookie to CDN hosts.
func applyBrowserHeaders(page *rod.Page, headers map[string]string, cookieURL string) error {
	if len(headers) == 0 {
		return nil
	}
	extra := make([]string, 0, 2*len(headers))
	var cookies []*proto.NetworkCookieParam
	for name, value := range headers {
		if strings.EqualFold(name, "Cookie") {
			cookies = parseCookieHeader(value, cookieURL)
			continue
		}
		extra = append(extra, name, value)
	}
	if len(cookies) > 0 {
		if err := page.SetCookies(cookies); err != nil {
			return err
		}
	}
	if len(extra) > 0 {
		_, err := page.SetExtraHeaders(extra)
		return err
	}
	return nil
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
