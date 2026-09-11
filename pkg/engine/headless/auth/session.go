package auth

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/go-rod/rod"
	"github.com/projectdiscovery/utils/errkit"
)

// CookieState is a browser-wide snapshot used to verify that a login changed
// authentication state. Cookies are browser-wide because SSO flows can finish
// on a different origin from the application login page.
type CookieState map[string]string

// SessionState captures the browser state commonly used to persist web
// authentication. Storage is compared only while the flow remains on the same
// origin; cookies are browser-wide and cover cross-origin SSO redirects.
type SessionState struct {
	Cookies        CookieState
	Origin         string
	LocalStorage   string
	SessionStorage string
}

// CaptureCookieState snapshots cookie values without exposing them to logs.
func CaptureCookieState(page *rod.Page) (CookieState, error) {
	if page == nil {
		return nil, errkit.New("recorded-flow: page is nil")
	}
	cookies, err := page.Browser().GetCookies()
	if err != nil {
		return nil, errkit.Wrap(err, "recorded-flow: failed to inspect session cookies")
	}
	state := make(CookieState, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		key := fmt.Sprintf("%s\x00%s\x00%s", cookie.Domain, cookie.Path, cookie.Name)
		state[key] = cookie.Value
	}
	return state, nil
}

// CookieStateChanged reports whether a cookie was added or its value changed.
// Deletions alone do not prove authentication.
func CookieStateChanged(before, after CookieState) bool {
	for key, value := range after {
		if previous, ok := before[key]; !ok || previous != value {
			return true
		}
	}
	return false
}

// CaptureSessionState snapshots cookies plus same-origin web storage.
func CaptureSessionState(page *rod.Page) (SessionState, error) {
	cookies, err := CaptureCookieState(page)
	if err != nil {
		return SessionState{}, err
	}
	state := SessionState{Cookies: cookies}
	if info, infoErr := page.Info(); infoErr == nil {
		if parsed, parseErr := url.Parse(info.URL); parseErr == nil {
			state.Origin = parsed.Scheme + "://" + parsed.Host
		}
	}
	// Storage can be unavailable on opaque/security-restricted origins. Cookie
	// verification remains useful, so an evaluation error is not fatal.
	if value, evalErr := page.Eval(`() => JSON.stringify({
		local: Object.entries(localStorage).sort(),
		session: Object.entries(sessionStorage).sort()
	})`); evalErr == nil && value != nil {
		var storage struct {
			Local   json.RawMessage `json:"local"`
			Session json.RawMessage `json:"session"`
		}
		raw := value.Value.Str()
		if json.Unmarshal([]byte(raw), &storage) == nil {
			state.LocalStorage = string(storage.Local)
			state.SessionStorage = string(storage.Session)
		}
	}
	return state, nil
}

// SessionStateChanged reports evidence that replay established browser auth.
func SessionStateChanged(before, after SessionState) bool {
	if CookieStateChanged(before.Cookies, after.Cookies) {
		return true
	}
	return before.Origin != "" &&
		before.Origin == after.Origin &&
		(before.LocalStorage != after.LocalStorage || before.SessionStorage != after.SessionStorage)
}
