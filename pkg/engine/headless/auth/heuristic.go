package auth

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth/recipe"
)

// HeuristicLogin attempts to log in without AI by detecting standard form patterns.
// It fills the detected username and password fields, submits the form, and verifies
// success by checking for URL change, new cookies, and absence of error messages.
//
// Returns the session state and a recipe for future zero-AI replay.
// Returns an error if the login form can't be detected or login fails.
func HeuristicLogin(page *rod.Page, config *AuthConfig, detection *LoginPageDetection) (*SessionState, *recipe.Recipe, error) {
	if detection == nil || !detection.IsLoginPage {
		return nil, nil, fmt.Errorf("heuristic login: page is not a login page")
	}
	if detection.PasswordField == nil {
		return nil, nil, fmt.Errorf("heuristic login: no password field detected")
	}

	var steps []recipe.Step

	// Capture cookies before login for diffing
	beforeCookies, err := GetCookiesFromPage(page)
	if err != nil {
		return nil, nil, fmt.Errorf("heuristic login: get cookies: %w", err)
	}

	// Remember the login URL to detect navigation
	info, _ := page.Info()
	loginURL := ""
	if info != nil {
		loginURL = info.URL
	}

	// Fill username field
	if detection.UsernameField != nil {
		if err := clearAndType(page, detection.UsernameField.Selector, config.Credentials.GetUsername()); err != nil {
			return nil, nil, fmt.Errorf("heuristic login: fill username: %w", err)
		}
		steps = append(steps,
			recipe.Step{Action: "clear", Selector: detection.UsernameField.Selector, Field: "username"},
			recipe.Step{Action: "type", Selector: detection.UsernameField.Selector, Value: "{{username}}", Field: "username"},
		)
	}

	// Fill password field
	if err := clearAndType(page, detection.PasswordField.Selector, config.Credentials.Password); err != nil {
		return nil, nil, fmt.Errorf("heuristic login: fill password: %w", err)
	}
	steps = append(steps,
		recipe.Step{Action: "clear", Selector: detection.PasswordField.Selector, Field: "password"},
		recipe.Step{Action: "type", Selector: detection.PasswordField.Selector, Value: "{{password}}", Field: "password"},
	)

	// Submit the form
	if detection.SubmitButton != nil {
		el, err := page.Element(detection.SubmitButton.Selector)
		if err != nil {
			return nil, nil, fmt.Errorf("heuristic login: find submit: %w", err)
		}
		if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return nil, nil, fmt.Errorf("heuristic login: click submit: %w", err)
		}
		steps = append(steps, recipe.Step{Action: "click", Selector: detection.SubmitButton.Selector, Field: "submit"})
	} else {
		// No submit button found — press Enter in the password field
		if err := page.Keyboard.Press(input.Enter); err != nil {
			return nil, nil, fmt.Errorf("heuristic login: press enter: %w", err)
		}
		steps = append(steps, recipe.Step{Action: "press_enter", Field: "submit"})
	}

	// Wait for navigation after form submission.
	// We poll for URL change rather than using WaitStable which blocks
	// indefinitely on pages with animations/live content.
	steps = append(steps, recipe.Step{Action: "wait", Value: "navigation"})
	waitForNavigation(page, loginURL, 5*time.Second)

	// === Verify Login Success ===
	success, failureReason := verifyLoginSuccess(page, loginURL, beforeCookies)
	if !success {
		return nil, nil, fmt.Errorf("heuristic login failed: %s", failureReason)
	}

	// Extract session
	session, err := ExtractSession(page)
	if err != nil {
		return nil, nil, fmt.Errorf("heuristic login: extract session: %w", err)
	}

	// Build recipe metadata (auto-detect success indicators for future use)
	afterCookies, _ := GetCookiesFromPage(page)
	meta := buildRecipeMetadata(page, beforeCookies, afterCookies, detection)

	r := &recipe.Recipe{
		LoginURL:  loginURL,
		Steps:     steps,
		Metadata:  meta,
		CreatedAt: time.Now(),
		Version:   1,
	}

	return session, r, nil
}

// verifyLoginSuccess checks multiple signals to determine if login succeeded.
// Returns (true, "") on success or (false, reason) on failure.
func verifyLoginSuccess(page *rod.Page, loginURL string, beforeCookies []CookieEntry) (bool, string) {
	info, _ := page.Info()
	currentURL := ""
	if info != nil {
		currentURL = info.URL
	}

	// Check 1: Did the URL change away from the login page?
	urlChanged := currentURL != loginURL && currentURL != "" && loginURL != ""
	if urlChanged {
		loginPath := extractPath(loginURL)
		currentPath := extractPath(currentURL)
		if loginPath == currentPath {
			urlChanged = false
		}
	}

	// Check 2: New session cookies appeared?
	afterCookies, _ := GetCookiesFromPage(page)
	cookieDiff := DiffCookies(beforeCookies, afterCookies)
	hasCookieChange := len(cookieDiff) > 0

	// Check 3: Is there an error message on the page?
	visibleText := getVisibleText(page)
	hasError := HasLoginError(visibleText)

	// Check 4: Is the password field still present? (still on login page)
	_, pwdErr := page.Element(`input[type="password"]`)
	passwordFieldGone := pwdErr != nil

	slog.Info("[auth] Post-login verification",
		slog.String("current_url", currentURL),
		slog.Bool("url_changed", urlChanged),
		slog.Bool("new_cookies", hasCookieChange),
		slog.Int("new_cookie_count", len(cookieDiff)),
		slog.Bool("error_detected", hasError),
		slog.Bool("password_gone", passwordFieldGone),
	)

	if hasError {
		return false, "error message detected on page"
	}
	if urlChanged && hasCookieChange {
		return true, ""
	}
	if urlChanged && passwordFieldGone {
		return true, ""
	}
	if hasCookieChange && passwordFieldGone {
		return true, ""
	}
	if urlChanged {
		return true, ""
	}

	// Still on the same page with no new cookies and password field visible
	return false, "no navigation or cookie change detected"
}

// buildRecipeMetadata auto-detects success indicators for the recipe.
func buildRecipeMetadata(page *rod.Page, before, after []CookieEntry, detection *LoginPageDetection) recipe.Metadata {
	meta := recipe.Metadata{}

	// Detect which cookie is the session cookie
	diff := DiffCookies(before, after)
	meta.SessionCookie = DetectNewSessionCookie(diff)

	// Where did we land after login?
	if info, err := page.Info(); err == nil {
		meta.SuccessURL = info.URL
	}

	// Find logout button for session monitoring
	meta.LogoutSelector = DetectLogoutSelector(page)

	// Store the password field selector for quick login page re-identification
	if detection.PasswordField != nil {
		meta.PasswordSelector = detection.PasswordField.Selector
	}

	return meta
}

// clearAndType clears a field and types text into it.
func clearAndType(page *rod.Page, selector, text string) error {
	el, err := page.Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %s: %w", selector, err)
	}

	// Clear existing content
	if err := el.SelectAllText(); err != nil {
		// SelectAllText may fail on empty fields — that's OK
		_ = err
	}
	if err := el.Input(""); err != nil {
		// Input empty string to clear — ignore errors
		_ = err
	}

	// Type the new value
	if err := el.Input(text); err != nil {
		return fmt.Errorf("input failed: %s: %w", selector, err)
	}

	return nil
}

// getVisibleText extracts the first 2000 characters of visible text from the page.
func getVisibleText(page *rod.Page) string {
	result, err := page.Eval(`() => {
		try { return (document.body ? document.body.innerText : '').substring(0, 2000); }
		catch(e) { return ''; }
	}`)
	if err != nil {
		return ""
	}
	return result.Value.Str()
}

// extractPath returns just the path component of a URL.
func extractPath(rawURL string) string {
	idx := strings.Index(rawURL, "://")
	if idx >= 0 {
		rest := rawURL[idx+3:]
		slashIdx := strings.Index(rest, "/")
		if slashIdx >= 0 {
			path := rest[slashIdx:]
			// Strip query string
			if qIdx := strings.Index(path, "?"); qIdx >= 0 {
				path = path[:qIdx]
			}
			return path
		}
		return "/"
	}
	return rawURL
}

// waitForNavigation polls for URL change or new cookies after form submission.
// Much faster than WaitStable on pages with animations/live content.
func waitForNavigation(page *rod.Page, previousURL string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	// Short initial wait for the form submission to trigger navigation
	time.Sleep(500 * time.Millisecond)

	for time.Now().Before(deadline) {
		// Check if URL changed (most reliable signal)
		if info, err := page.Info(); err == nil && info != nil {
			if info.URL != previousURL && info.URL != "" {
				// URL changed — give the new page a moment to settle
				time.Sleep(500 * time.Millisecond)
				return
			}
		}

		// Check if new cookies appeared (session cookie set = login happened)
		cookies, err := GetCookiesFromPage(page)
		if err == nil && len(cookies) > 0 {
			// Check for session-like cookies that weren't there before
			for _, c := range cookies {
				if c.HTTPOnly && c.Value != "" {
					time.Sleep(500 * time.Millisecond)
					return
				}
			}
		}

		time.Sleep(300 * time.Millisecond)
	}
}
