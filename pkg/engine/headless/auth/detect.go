package auth

import (
	"regexp"
	"strings"

	"github.com/go-rod/rod"
	headlesstypes "github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// LoginPageDetection contains the results of analyzing a page for login signals.
type LoginPageDetection struct {
	// IsLoginPage is true if confidence >= threshold.
	IsLoginPage bool

	// Confidence is the weighted score (0.0 - 1.0).
	Confidence float64

	// LoginURL is the URL of the detected login page.
	LoginURL string

	// PasswordField is the CSS selector and metadata for the password input.
	PasswordField *FieldInfo

	// UsernameField is the CSS selector and metadata for the username/email input.
	UsernameField *FieldInfo

	// SubmitButton is the CSS selector and metadata for the submit button.
	SubmitButton *FieldInfo

	// OAuthProviders lists detected OAuth/SSO providers (e.g., "google", "github").
	OAuthProviders []string

	// HasCaptcha is true if a CAPTCHA was detected on the page.
	HasCaptcha bool
}

// FieldInfo describes a form field detected during login page analysis.
type FieldInfo struct {
	Selector string
	Name     string
	Type     string
	Label    string
}

const loginConfidenceThreshold = 0.50

// DetectLoginPage analyzes a page to determine if it's a login page.
// Uses multiple weighted signals — no single signal is definitive.
// The page parameter is used for URL/title extraction; forms come from the existing discovery.
func DetectLoginPage(page *rod.Page, forms []*headlesstypes.HTMLForm) *LoginPageDetection {
	detection := &LoginPageDetection{}
	var confidence float64

	// Get page URL and title (page may be nil in unit tests)
	var pageURL, pageTitle string
	if page != nil {
		if info, err := page.Info(); err == nil {
			pageURL = info.URL
			pageTitle = info.Title
			detection.LoginURL = pageURL
		}
	}

	// === Signal 1: Password field in form (+0.40) ===
	var loginForm *headlesstypes.HTMLForm
	for _, form := range forms {
		for _, el := range form.Elements {
			if strings.EqualFold(el.Type, "password") {
				confidence += 0.40
				detection.PasswordField = &FieldInfo{
					Selector: el.CSSSelector,
					Name:     el.Attributes["name"],
					Type:     el.Type,
				}
				loginForm = form
				break
			}
		}
		if loginForm != nil {
			break
		}
	}

	// === Signal 2: Username/email field in same form (+0.15) ===
	if loginForm != nil {
		for _, el := range loginForm.Elements {
			if isUsernameField(el) {
				confidence += 0.15
				detection.UsernameField = &FieldInfo{
					Selector: el.CSSSelector,
					Name:     el.Attributes["name"],
					Type:     el.Type,
				}
				break
			}
		}
	}

	// === Signal 3: Submit button with login text (+0.10) ===
	if loginForm != nil {
		for _, el := range loginForm.Elements {
			if isLoginSubmitButton(el) {
				confidence += 0.10
				detection.SubmitButton = &FieldInfo{
					Selector: el.CSSSelector,
					Name:     el.TextContent,
					Type:     el.Type,
				}
				break
			}
		}
		// Fallback: any submit button
		if detection.SubmitButton == nil {
			for _, el := range loginForm.Elements {
				if strings.EqualFold(el.Type, "submit") || (strings.EqualFold(el.TagName, "BUTTON") && el.Type != "reset") {
					detection.SubmitButton = &FieldInfo{
						Selector: el.CSSSelector,
						Name:     el.TextContent,
						Type:     el.Type,
					}
					break
				}
			}
		}
	}

	// === Signal 4: URL contains login keyword (+0.10) ===
	if loginURLKeywordPattern.MatchString(pageURL) {
		confidence += 0.10
	}

	// === Signal 5: Page title contains login keyword (+0.05) ===
	if loginTitlePattern.MatchString(pageTitle) {
		confidence += 0.05
	}

	// === Signal 6: "Forgot password" link (+0.05) ===
	// Check all forms' links and page elements
	for _, form := range forms {
		for _, el := range form.Elements {
			if forgotPasswordPattern.MatchString(el.TextContent) {
				confidence += 0.05
				break
			}
		}
	}

	// === Signal 7: "Register" / "Sign up" link (+0.05) ===
	for _, form := range forms {
		for _, el := range form.Elements {
			if registerPattern.MatchString(el.TextContent) {
				confidence += 0.05
				break
			}
		}
	}

	// === Signal 8: OAuth/SSO buttons (+0.05) ===
	detection.OAuthProviders = detectOAuthProviders(forms)
	if len(detection.OAuthProviders) > 0 {
		confidence += 0.05
	}

	// === Signal 9: Form method is POST (+0.03) ===
	if loginForm != nil && strings.EqualFold(loginForm.Method, "POST") {
		confidence += 0.03
	}

	// === Signal 10: Single form on page (+0.02) ===
	if len(forms) == 1 {
		confidence += 0.02
	}

	detection.Confidence = confidence
	detection.IsLoginPage = confidence >= loginConfidenceThreshold

	return detection
}

// Regex patterns for login page detection signals.
var (
	loginURLKeywordPattern = regexp.MustCompile(`(?i)/(login|signin|sign-in|auth|sso|oauth|cas|authenticate)(/|$|\?)`)
	loginTitlePattern      = regexp.MustCompile(`(?i)(log\s*in|sign\s*in|authentication|login)`)
	forgotPasswordPattern  = regexp.MustCompile(`(?i)(forgot|reset|recover).{0,10}(password|pass)`)
	registerPattern        = regexp.MustCompile(`(?i)(register|sign\s*up|create\s*(an\s*)?account|join)`)
	loginButtonPattern     = regexp.MustCompile(`(?i)^(log\s*in|sign\s*in|submit|continue|enter|authenticate)$`)
	usernameFieldPattern   = regexp.MustCompile(`(?i)(user|email|login|account|identifier|name)`)
)

// isUsernameField returns true if the element looks like a username/email input.
func isUsernameField(el *headlesstypes.HTMLElement) bool {
	if el.Type == "password" || el.Type == "hidden" || el.Type == "submit" || el.Type == "button" {
		return false
	}
	if strings.EqualFold(el.TagName, "BUTTON") {
		return false
	}

	// Check type
	if el.Type == "email" {
		return true
	}

	// Check name attribute
	name := el.Attributes["name"]
	if usernameFieldPattern.MatchString(name) {
		return true
	}

	// Check placeholder
	placeholder := el.Attributes["placeholder"]
	if usernameFieldPattern.MatchString(placeholder) {
		return true
	}

	// Check aria-label
	ariaLabel := el.Attributes["aria-label"]
	if usernameFieldPattern.MatchString(ariaLabel) {
		return true
	}

	// Check id
	if usernameFieldPattern.MatchString(el.ID) {
		return true
	}

	return false
}

// isLoginSubmitButton returns true if the element looks like a login submit button.
func isLoginSubmitButton(el *headlesstypes.HTMLElement) bool {
	if !strings.EqualFold(el.TagName, "BUTTON") && el.Type != "submit" {
		return false
	}
	return loginButtonPattern.MatchString(strings.TrimSpace(el.TextContent))
}

// OAuth provider patterns matched against button text, link text, and class names.
var oauthProviderPatterns = map[string]*regexp.Regexp{
	"google":    regexp.MustCompile(`(?i)(google|goog|gmail)`),
	"github":    regexp.MustCompile(`(?i)(github)`),
	"facebook":  regexp.MustCompile(`(?i)(facebook|fb\b)`),
	"apple":     regexp.MustCompile(`(?i)(apple|icloud)`),
	"microsoft": regexp.MustCompile(`(?i)(microsoft|azure|outlook|office\s*365)`),
	"okta":      regexp.MustCompile(`(?i)(okta)`),
	"auth0":     regexp.MustCompile(`(?i)(auth0)`),
	"saml":      regexp.MustCompile(`(?i)(saml|sso)`),
}

// detectOAuthProviders scans forms for OAuth/SSO provider signals.
func detectOAuthProviders(forms []*headlesstypes.HTMLForm) []string {
	seen := make(map[string]bool)
	for _, form := range forms {
		for _, el := range form.Elements {
			text := el.TextContent + " " + el.Classes
			if el.Attributes != nil {
				text += " " + el.Attributes["href"] + " " + el.Attributes["data-provider"]
			}
			for provider, pattern := range oauthProviderPatterns {
				if pattern.MatchString(text) {
					seen[provider] = true
				}
			}
		}
	}

	providers := make([]string, 0, len(seen))
	for p := range seen {
		providers = append(providers, p)
	}
	return providers
}
