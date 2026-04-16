package auth

import (
	"testing"

	headlesstypes "github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/assert"
)

// mockForm creates a minimal HTMLForm for testing detection signals.
func mockForm(method string, elements ...*headlesstypes.HTMLElement) *headlesstypes.HTMLForm {
	return &headlesstypes.HTMLForm{
		TagName:     "FORM",
		Method:      method,
		CSSSelector: "form",
		Elements:    elements,
	}
}

func mockElement(tagName, inputType, name, text string, attrs map[string]string) *headlesstypes.HTMLElement {
	if attrs == nil {
		attrs = make(map[string]string)
	}
	if name != "" {
		attrs["name"] = name
	}
	return &headlesstypes.HTMLElement{
		TagName:     tagName,
		Type:        inputType,
		TextContent: text,
		CSSSelector: "input[name='" + name + "']",
		Attributes:  attrs,
		ID:          name,
	}
}

func TestDetectLoginPage_TypicalLoginForm(t *testing.T) {
	form := mockForm("POST",
		mockElement("INPUT", "email", "email", "", map[string]string{"placeholder": "Email"}),
		mockElement("INPUT", "password", "password", "", nil),
		mockElement("BUTTON", "submit", "", "Sign In", nil),
	)

	// DetectLoginPage takes a *rod.Page — nil is OK for unit tests since
	// we pass forms directly and the page is only used for Info() which
	// we don't need for signal-based detection of form structure.
	detection := DetectLoginPage(nil, []*headlesstypes.HTMLForm{form})

	assert.True(t, detection.IsLoginPage)
	assert.GreaterOrEqual(t, detection.Confidence, 0.50)
	assert.NotNil(t, detection.PasswordField)
	assert.NotNil(t, detection.UsernameField)
	assert.NotNil(t, detection.SubmitButton)
}

func TestDetectLoginPage_PasswordFieldOnly(t *testing.T) {
	// Just a password field with no clear username or submit — should still detect
	form := mockForm("POST",
		mockElement("INPUT", "text", "identifier", "", nil),
		mockElement("INPUT", "password", "pass", "", nil),
	)

	detection := DetectLoginPage(nil, []*headlesstypes.HTMLForm{form})

	assert.True(t, detection.IsLoginPage, "password field alone should give confidence >= 0.50")
	assert.NotNil(t, detection.PasswordField)
}

func TestDetectLoginPage_SearchForm(t *testing.T) {
	// A search form should NOT be detected as login
	form := mockForm("GET",
		mockElement("INPUT", "text", "q", "", map[string]string{"placeholder": "Search..."}),
		mockElement("BUTTON", "submit", "", "Search", nil),
	)

	detection := DetectLoginPage(nil, []*headlesstypes.HTMLForm{form})

	assert.False(t, detection.IsLoginPage)
	assert.Nil(t, detection.PasswordField)
}

func TestDetectLoginPage_RegistrationForm(t *testing.T) {
	// Registration form with password — this SHOULD detect as login-like
	// (registration and login pages share password fields)
	form := mockForm("POST",
		mockElement("INPUT", "text", "name", "", nil),
		mockElement("INPUT", "email", "email", "", nil),
		mockElement("INPUT", "password", "password", "", nil),
		mockElement("INPUT", "password", "confirm_password", "", nil),
		mockElement("BUTTON", "submit", "", "Create Account", nil),
	)

	detection := DetectLoginPage(nil, []*headlesstypes.HTMLForm{form})

	// Has a password field so it detects, but confidence might be lower
	assert.NotNil(t, detection.PasswordField)
	assert.GreaterOrEqual(t, detection.Confidence, 0.40) // at least the password signal fires
}

func TestDetectLoginPage_EmptyForms(t *testing.T) {
	detection := DetectLoginPage(nil, nil)
	assert.False(t, detection.IsLoginPage)
	assert.Equal(t, float64(0), detection.Confidence)
}

func TestDetectLoginPage_OAuthButtons(t *testing.T) {
	form := mockForm("POST",
		mockElement("INPUT", "password", "password", "", nil),
		mockElement("BUTTON", "", "", "Sign in with Google", nil),
		mockElement("BUTTON", "", "", "Sign in with GitHub", nil),
	)

	detection := DetectLoginPage(nil, []*headlesstypes.HTMLForm{form})

	assert.True(t, detection.IsLoginPage)
	assert.Contains(t, detection.OAuthProviders, "google")
	assert.Contains(t, detection.OAuthProviders, "github")
}

func TestIsUsernameField(t *testing.T) {
	tests := []struct {
		name     string
		element  *headlesstypes.HTMLElement
		expected bool
	}{
		{"email type", mockElement("INPUT", "email", "email", "", nil), true},
		{"name contains user", mockElement("INPUT", "text", "username", "", nil), true},
		{"name contains login", mockElement("INPUT", "text", "login_id", "", nil), true},
		{"placeholder email", mockElement("INPUT", "text", "field1", "", map[string]string{"placeholder": "Enter your email"}), true},
		{"password field", mockElement("INPUT", "password", "password", "", nil), false},
		{"hidden field", mockElement("INPUT", "hidden", "csrf_token", "", nil), false},
		{"generic text", mockElement("INPUT", "text", "query", "", nil), false},
		{"button", mockElement("BUTTON", "submit", "submit", "Submit", nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isUsernameField(tt.element))
		})
	}
}

func TestIsLoginSubmitButton(t *testing.T) {
	tests := []struct {
		name     string
		element  *headlesstypes.HTMLElement
		expected bool
	}{
		{"sign in button", mockElement("BUTTON", "submit", "", "Sign In", nil), true},
		{"log in button", mockElement("BUTTON", "submit", "", "Log In", nil), true},
		{"login button", mockElement("BUTTON", "submit", "", "Login", nil), true},
		{"continue button", mockElement("BUTTON", "submit", "", "Continue", nil), true},
		{"search button", mockElement("BUTTON", "submit", "", "Search", nil), false},
		{"save button", mockElement("BUTTON", "submit", "", "Save Changes", nil), false},
		{"non-button element", mockElement("INPUT", "text", "", "Sign In", nil), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isLoginSubmitButton(tt.element))
		})
	}
}
