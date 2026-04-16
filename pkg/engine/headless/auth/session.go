package auth

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// SessionState captures the browser authentication state after a successful login.
// It includes cookies (including httpOnly via CDP), localStorage tokens, and any
// Authorization headers observed in XHR traffic.
type SessionState struct {
	Cookies      []CookieEntry     `json:"cookies"`
	LocalStorage map[string]string  `json:"local_storage,omitempty"`
	AuthHeaders  map[string]string  `json:"auth_headers,omitempty"`
}

// CookieEntry represents a single browser cookie.
type CookieEntry struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"http_only"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"same_site,omitempty"`
}

// ExtractSession captures the full authentication state from a browser page.
// Uses CDP Network.getCookies for httpOnly cookies and JS eval for localStorage.
func ExtractSession(page *rod.Page) (*SessionState, error) {
	session := &SessionState{}

	// Extract all cookies including httpOnly via CDP
	cookieResult, err := proto.NetworkGetCookies{}.Call(page)
	if err != nil {
		return nil, fmt.Errorf("extract cookies: %w", err)
	}
	for _, c := range cookieResult.Cookies {
		session.Cookies = append(session.Cookies, CookieEntry{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  float64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		})
	}

	// Extract localStorage (may fail on some pages that restrict access)
	localStorageResult, err := page.Eval(`() => {
		try {
			const items = {};
			for (let i = 0; i < localStorage.length; i++) {
				const key = localStorage.key(i);
				items[key] = localStorage.getItem(key);
			}
			return items;
		} catch(e) {
			return {};
		}
	}`)
	if err == nil {
		storage := make(map[string]string)
		if umErr := localStorageResult.Value.Unmarshal(&storage); umErr == nil && len(storage) > 0 {
			session.LocalStorage = storage
		}
	}

	return session, nil
}

// ApplySession restores authentication state onto a browser page.
// Rod shares cookies at the Browser level, so setting them on one page
// makes them available to all pages in the same browser instance.
// ApplySession restores authentication state onto a browser page.
// Cookies are set via CDP Network.setCookie (one at a time for reliability)
// with both Domain and URL fields to ensure proper domain association.
func ApplySession(page *rod.Page, session *SessionState) error {
	if session == nil {
		return nil
	}

	// Set cookies via CDP — use individual SetCookie calls with URL field
	// for maximum compatibility. The URL field tells Chrome which domain
	// to associate the cookie with even when the page is on about:blank.
	for _, c := range session.Cookies {
		// Build the URL for this cookie's domain
		cookieURL := buildCookieURL(c)

		param := proto.NetworkSetCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
		}
		if cookieURL != "" {
			param.URL = cookieURL
		}
		if c.Expires > 0 {
			ts := proto.TimeSinceEpoch(c.Expires)
			param.Expires = ts
		}
		if c.SameSite != "" {
			param.SameSite = proto.NetworkCookieSameSite(c.SameSite)
		}
		if _, err := param.Call(page); err != nil {
			// Log but continue — some cookies may fail (e.g., expired)
			continue
		}
	}

	// Set localStorage entries
	if len(session.LocalStorage) > 0 {
		for key, value := range session.LocalStorage {
			_, err := page.Eval(`(k, v) => { try { localStorage.setItem(k, v); } catch(e) {} }`, key, value)
			if err != nil {
				continue // localStorage may not be available on all pages
			}
		}
	}

	return nil
}

// buildCookieURL constructs a URL for CDP Network.setCookie from a cookie's domain.
// The URL field is more reliable than Domain alone for associating cookies,
// especially when the page is on about:blank.
func buildCookieURL(c CookieEntry) string {
	domain := c.Domain
	if domain == "" {
		return ""
	}
	// Strip leading dot from domain (e.g., ".example.com" → "example.com")
	domain = strings.TrimPrefix(domain, ".")

	scheme := "https"
	if !c.Secure {
		scheme = "http"
	}
	path := c.Path
	if path == "" {
		path = "/"
	}
	return scheme + "://" + domain + path
}

// GetCookieNames returns the names of all cookies in the session.
func (s *SessionState) GetCookieNames() []string {
	names := make([]string, len(s.Cookies))
	for i, c := range s.Cookies {
		names[i] = c.Name
	}
	return names
}

// DiffCookies returns cookies that are new or changed compared to the before state.
func DiffCookies(before, after []CookieEntry) []CookieEntry {
	beforeMap := make(map[string]string, len(before))
	for _, c := range before {
		beforeMap[c.Name] = c.Value
	}

	var diff []CookieEntry
	for _, c := range after {
		if oldVal, exists := beforeMap[c.Name]; !exists || oldVal != c.Value {
			diff = append(diff, c)
		}
	}
	return diff
}

// GetCookiesFromPage extracts cookies from a page via CDP (convenience wrapper).
func GetCookiesFromPage(page *rod.Page) ([]CookieEntry, error) {
	result, err := proto.NetworkGetCookies{}.Call(page)
	if err != nil {
		return nil, err
	}
	entries := make([]CookieEntry, 0, len(result.Cookies))
	for _, c := range result.Cookies {
		entries = append(entries, CookieEntry{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  float64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		})
	}
	return entries, nil
}

// loginURLPattern matches common login/auth URL path segments.
var loginURLPattern = regexp.MustCompile(`(?i)/(login|signin|sign-in|auth|sso|oauth|cas|authenticate)(/|$|\?)`)

// IsLoginURL returns true if the URL looks like a login page based on path patterns.
func IsLoginURL(rawURL string) bool {
	return loginURLPattern.MatchString(rawURL)
}

// errorTextPattern matches common authentication error messages in page text.
var errorTextPattern = regexp.MustCompile(`(?i)(invalid|incorrect|wrong|failed|denied|error|unauthorized).{0,30}(credential|password|login|username|email|authentication)`)

// HasLoginError checks if visible text contains authentication error messages.
func HasLoginError(visibleText string) bool {
	return errorTextPattern.MatchString(visibleText)
}

// DetectNewSessionCookie examines cookie diffs to find the most likely session cookie.
// Heuristic: prefer httpOnly cookies, then cookies with "session" or "sid" in the name.
func DetectNewSessionCookie(diff []CookieEntry) string {
	if len(diff) == 0 {
		return ""
	}

	// Prefer httpOnly cookies (most likely session identifiers)
	for _, c := range diff {
		if c.HTTPOnly {
			return c.Name
		}
	}

	// Look for session-like names
	sessionPattern := regexp.MustCompile(`(?i)(session|sess|sid|token|auth|jwt)`)
	for _, c := range diff {
		if sessionPattern.MatchString(c.Name) {
			return c.Name
		}
	}

	// Fall back to the first new cookie
	return diff[0].Name
}

// DetectLogoutSelector tries to find a logout link/button on the current page.
// Returns the CSS selector or empty string if not found.
func DetectLogoutSelector(page *rod.Page) string {
	selectors := []string{
		`a[href*="logout"]`,
		`a[href*="signout"]`,
		`a[href*="sign-out"]`,
		`a[href*="log-out"]`,
		`button[class*="logout"]`,
		`button[class*="signout"]`,
	}

	for _, sel := range selectors {
		el, err := page.Element(sel)
		if err == nil && el != nil {
			visible, _ := el.Visible()
			if visible {
				return sel
			}
		}
	}

	// Try text-based detection
	textSelectors := []struct {
		sel  string
		text string
	}{
		{"a", "(?i)^(log\\s*out|sign\\s*out)$"},
		{"button", "(?i)^(log\\s*out|sign\\s*out)$"},
	}

	for _, ts := range textSelectors {
		els, err := page.Elements(ts.sel)
		if err != nil {
			continue
		}
		re := regexp.MustCompile(ts.text)
		for _, el := range els {
			text, err := el.Text()
			if err != nil {
				continue
			}
			if re.MatchString(strings.TrimSpace(text)) {
				// Get a usable selector for this element
				id, _ := el.Attribute("id")
				if id != nil && *id != "" {
					return fmt.Sprintf(`%s#%s`, ts.sel, *id)
				}
				href, _ := el.Attribute("href")
				if href != nil && *href != "" {
					return fmt.Sprintf(`%s[href="%s"]`, ts.sel, *href)
				}
			}
		}
	}

	return ""
}
