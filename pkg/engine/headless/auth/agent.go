package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth/recipe"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
)

// BrowserToolkit provides browser operations for AI login agents.
// Each method corresponds to an agent tool. The toolkit wraps a BrowserPage
// and uses CSS selectors for element targeting (more natural for LLMs than XPath).
type BrowserToolkit interface {
	// Page reading (layered — agent picks detail level)
	GetAccessibilityTree() (string, error)
	GetForms() ([]FormView, error)
	GetInteractiveElements() ([]ElementView, error)
	GetVisibleText() (string, error)

	// Navigation
	Navigate(url string) error
	GetCurrentURL() (string, error)
	WaitForNavigation(timeout time.Duration) error

	// Interaction
	TypeText(selector, text string) error
	ClearField(selector string) error
	Click(selector string) error
	SelectOption(selector, value string) error
	PressEnter() error

	// State
	GetCookies() ([]CookieEntry, error)
	TakeScreenshot() ([]byte, error)
}

// LoginResult is the structured output from an AI login agent.
type LoginResult struct {
	Success bool           `json:"success"`
	Recipe  *recipe.Recipe `json:"recipe,omitempty"`
	Session *SessionState  `json:"session,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// LoginAgent performs AI-powered login. Implemented by the external agent framework.
// The agent receives a BrowserToolkit to interact with the page and an AuthConfig
// with the credentials. It should explore the page, fill the login form, submit it,
// verify success, and return a recipe for future replay.
type LoginAgent interface {
	Login(ctx context.Context, toolkit BrowserToolkit, config *AuthConfig) (*LoginResult, error)
}

// LoginAgentFactory creates a LoginAgent from an API key.
type LoginAgentFactory func(apiKey string) (LoginAgent, error)

// loginAgentRegistry stores registered login agent factories by provider name.
var loginAgentRegistry = map[string]LoginAgentFactory{}

// RegisterLoginAgent registers a LoginAgent factory under a provider name.
// Called by external agent framework packages via init().
func RegisterLoginAgent(name string, factory LoginAgentFactory) {
	loginAgentRegistry[name] = factory
}

// NewLoginAgent creates a LoginAgent from the registry.
func NewLoginAgent(provider, apiKey string) (LoginAgent, error) {
	factory, ok := loginAgentRegistry[provider]
	if !ok {
		return nil, fmt.Errorf("unknown login agent provider: %q (registered: %v)", provider, registeredProviders())
	}
	return factory(apiKey)
}

func registeredProviders() []string {
	names := make([]string, 0, len(loginAgentRegistry))
	for name := range loginAgentRegistry {
		names = append(names, name)
	}
	return names
}

// browserToolkit implements BrowserToolkit wrapping a BrowserPage.
type browserToolkit struct {
	page *browser.BrowserPage
}

// NewBrowserToolkit creates a BrowserToolkit wrapping the given BrowserPage.
func NewBrowserToolkit(page *browser.BrowserPage) BrowserToolkit {
	return &browserToolkit{page: page}
}

func (t *browserToolkit) GetAccessibilityTree() (string, error) {
	return t.page.GetAccessibilityTree(5)
}

func (t *browserToolkit) GetForms() ([]FormView, error) {
	forms, err := t.page.GetAllForms()
	if err != nil {
		return nil, err
	}
	return convertForms(forms, t.page.Page), nil
}

func (t *browserToolkit) GetInteractiveElements() ([]ElementView, error) {
	elements, err := t.page.GetAllElements("a, button, [role='button'], input[type='submit'], [onclick]")
	if err != nil {
		return nil, err
	}
	return convertElements(elements), nil
}

func (t *browserToolkit) GetVisibleText() (string, error) {
	result, err := t.page.Eval(`() => {
		try { return (document.body ? document.body.innerText : '').substring(0, 2000); }
		catch(e) { return ''; }
	}`)
	if err != nil {
		return "", err
	}
	return result.Value.Str(), nil
}

func (t *browserToolkit) Navigate(url string) error {
	return t.page.Navigate(url)
}

func (t *browserToolkit) GetCurrentURL() (string, error) {
	info, err := t.page.Info()
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

func (t *browserToolkit) WaitForNavigation(timeout time.Duration) error {
	// Get current URL before waiting
	currentURL := ""
	if info, err := t.page.Info(); err == nil {
		currentURL = info.URL
	}
	waitForNavigation(t.page.Page, currentURL, timeout)
	return nil
}

func (t *browserToolkit) TypeText(selector, text string) error {
	return clearAndType(t.page.Page, selector, text)
}

func (t *browserToolkit) ClearField(selector string) error {
	el, err := t.page.Page.Element(selector)
	if err != nil {
		return err
	}
	_ = el.SelectAllText()
	return el.Input("")
}

func (t *browserToolkit) Click(selector string) error {
	el, err := t.page.Page.Element(selector)
	if err != nil {
		return err
	}
	return el.Click(proto.InputMouseButtonLeft, 1)
}

func (t *browserToolkit) SelectOption(selector, value string) error {
	el, err := t.page.Page.Element(selector)
	if err != nil {
		return err
	}
	return el.Select([]string{value}, true, rod.SelectorTypeText)
}

func (t *browserToolkit) PressEnter() error {
	return t.page.Page.Keyboard.Press(input.Enter)
}

func (t *browserToolkit) GetCookies() ([]CookieEntry, error) {
	return GetCookiesFromPage(t.page.Page)
}

func (t *browserToolkit) TakeScreenshot() ([]byte, error) {
	return t.page.Page.Screenshot(true, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
}
