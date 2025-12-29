package browser

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

const (
	// buttonsCSSSelector is the css selector for all buttons
	buttonsCSSSelector = "button, input[type='button'], input[type='submit']"
	// linksCSSSelector is the css selector for all anchor tags
	linksCSSSelector = "a"
)

// isElementDisabled checks if a button element is disabled
func isElementDisabled(element *types.HTMLElement) bool {
	if element.Attributes == nil {
		return false
	}

	// Standard HTML disabled attribute
	if _, disabled := element.Attributes["disabled"]; disabled {
		return true
	}

	// Tailwind or framework class-based detection
	if classAttr, ok := element.Attributes["class"]; ok {
		classList := strings.Fields(classAttr)
		for _, class := range classList {
			if class == "cursor-not-allowed" || class == "pointer-events-none" {
				return true
			}
		}
	}

	// Optionally support ARIA disabled
	if aria, ok := element.Attributes["aria-disabled"]; ok && (aria == "true" || aria == "1") {
		return true
	}

	return false
}

// FindNavigation attempts to find more navigations on the page which could
// be done to find more links and pages.
//
// This includes the following -
//  1. Forms
//  2. Buttons
//  3. Links
//  4. Elements with event listeners
//
// The navigations found are unique across the page. The caller
// needs to ensure they are unique globally before doing further actions with details.
func (b *BrowserPage) FindNavigations() ([]*types.Action, error) {
	unique := make(map[string]struct{})

	navigations := make([]*types.Action, 0)

	forms, err := b.GetAllForms()
	if err != nil {
		return nil, errors.Wrap(err, "could not get forms")
	}
	for _, form := range forms {
		for _, element := range form.Elements {
			if element.TagName != "BUTTON" {
				continue
			}
			// TODO: Check if this button is already in the unique map
			// and if so remove it
			unique[element.Hash()] = struct{}{}
		}
		hash := form.Hash()
		if _, found := unique[hash]; found {
			continue
		}
		unique[hash] = struct{}{}

		navigations = append(navigations, &types.Action{
			Type: types.ActionTypeFillForm,
			Form: form,
		})
	}

	buttons, err := b.GetAllElements(buttonsCSSSelector)
	if err != nil {
		return nil, errors.Wrap(err, "could not get buttons")
	}
	for _, button := range buttons {
		if isElementDisabled(button) {
			continue
		}

		hash := button.Hash()
		button.MD5Hash = hash

		if _, found := unique[hash]; found {
			continue
		}
		unique[hash] = struct{}{}
		navigations = append(navigations, &types.Action{
			Type:    types.ActionTypeLeftClick,
			Element: button,
		})
	}

	scopeValidator := b.launcher.ScopeValidator()
	links, err := b.GetAllElements(linksCSSSelector)
	if err != nil {
		return nil, errors.Wrap(err, "could not get links")
	}
	info, err := b.Info()
	if err != nil {
		return nil, errors.Wrap(err, "could not get page info")
	}
	for _, link := range links {
		href := link.Attributes["href"]
		if href == "" {
			continue
		}

		resolvedHref, err := resolveURL(info.URL, href)
		if err != nil {
			continue
		}

		u, err := url.Parse(resolvedHref)
		if err != nil {
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		if !scopeValidator(resolvedHref) {
			continue
		}

		hash := link.Hash()
		link.MD5Hash = hash

		if _, found := unique[hash]; found {
			continue
		}
		unique[hash] = struct{}{}
		navigations = append(navigations, &types.Action{
			Type:    types.ActionTypeLeftClick,
			Element: link,
		})
	}

	eventListeners, err := b.GetEventListeners()
	if err != nil {
		return nil, errors.Wrap(err, "could not get event listeners")
	}
	for _, listener := range eventListeners {
		if _, found := relevantEventListeners[listener.Type]; !found {
			continue
		}
		if listener.Element == nil {
			continue
		}
		hash := listener.Element.Hash()
		listener.Element.MD5Hash = hash
		if _, found := unique[hash]; found {
			continue
		}
		unique[hash] = struct{}{}
		navigations = append(navigations, types.ActionFromEventListener(listener))
	}

	return navigations, nil
}

func (b *BrowserPage) GetAllElements(selector string) ([]*types.HTMLElement, error) {
	objects, err := b.Eval(`() => window.getAllElements(` + strconv.Quote(selector) + `)`)
	if err != nil {
		return nil, err
	}

	elements := make([]*types.HTMLElement, 0)
	if err := objects.Value.Unmarshal(&elements); err != nil {
		return nil, err
	}
	return elements, nil
}

func (b *BrowserPage) GetElementFromXpath(xpath string) (*types.HTMLElement, error) {
	object, err := b.Eval(`() => window.getElementFromXPath(` + strconv.Quote(xpath) + `)`)
	if err != nil {
		return nil, err
	}

	element := &types.HTMLElement{}
	if err := object.Value.Unmarshal(element); err != nil {
		return nil, err
	}
	return element, nil
}

func (b *BrowserPage) GetAllForms() ([]*types.HTMLForm, error) {
	objects, err := b.Eval(`() => window.getAllForms()`)
	if err != nil {
		return nil, err
	}

	elements := make([]*types.HTMLForm, 0)
	if err := objects.Value.Unmarshal(&elements); err != nil {
		return nil, err
	}
	return elements, nil
}

// GetEventListeners returns all event listeners on the page
func (b *BrowserPage) GetEventListeners() ([]*types.EventListener, error) {
	listeners := make([]*types.EventListener, 0)

	eventlisteners, err := b.Eval(`() => window.__eventListeners`)
	if err == nil {
		_ = eventlisteners.Value.Unmarshal(&listeners)
	}

	// Also get inline event listeners
	var inlineEventListeners []struct {
		Element   *types.HTMLElement `json:"element"`
		Listeners []struct {
			Type     string `json:"type"`
			Listener string `json:"listener"`
		} `json:"listeners"`
	}
	inlineListeners, err := b.Eval(`() => window.getAllElementsWithEventListeners()`)
	if err != nil {
		return nil, err
	}
	if err := inlineListeners.Value.Unmarshal(&inlineEventListeners); err != nil {
		return nil, err
	}

	for _, inlineListener := range inlineEventListeners {
		for _, listener := range inlineListener.Listeners {
			listenerType := strings.TrimPrefix(listener.Type, "on")
			listeners = append(listeners, &types.EventListener{
				Type:     listenerType,
				Listener: listener.Listener,
				Element:  inlineListener.Element,
			})
		}
	}
	return listeners, nil
}

// NavigatedLink is a link navigated collected from one of the
// navigation hooks.
type NavigatedLink struct {
	URL    string `json:"url"`
	Source string `json:"source"`
}

// GetNavigatedLinks returns all navigated links on the page
func (b *BrowserPage) GetNavigatedLinks() ([]*NavigatedLink, error) {
	navigatedLinks, err := b.Eval(`() => window.__navigatedLinks`)
	if err != nil {
		return nil, err
	}

	listeners := make([]*NavigatedLink, 0)
	if err := navigatedLinks.Value.Unmarshal(&listeners); err != nil {
		return nil, err
	}
	return listeners, nil
}

// Define the map to hold event types
var relevantEventListeners = map[string]struct{}{
	// Focus and Blur events
	"focusin":  {},
	"focus":    {},
	"blur":     {},
	"focusout": {},

	// Click and Mouse events
	"click":       {},
	"auxclick":    {},
	"mousedown":   {},
	"mouseup":     {},
	"dblclick":    {},
	"mouseover":   {},
	"mouseenter":  {},
	"mouseleave":  {},
	"mouseout":    {},
	"wheel":       {},
	"contextmenu": {},

	// Key events
	"keydown":  {},
	"keypress": {},
	"keyup":    {},

	// Form events
	"submit": {},
	"input":  {},
	"change": {},
}

// resolveURL resolves a potentially relative URL against a base URL
func resolveURL(baseURLStr, href string) (string, error) {
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return "", err
	}

	resolvedURL, err := baseURL.Parse(href)
	if err != nil {
		return "", err
	}
	return resolvedURL.String(), nil
}
