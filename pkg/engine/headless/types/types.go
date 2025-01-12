package types

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	IsDiagnosticEnabled = false
)

// PageState represents a page in the state of the
// web application as determined by the crawler.
// It represents the vertex of the crawl graph
type PageState struct {
	UniqueID    string `json:"unique_id"`
	OriginID    string `json:"origin_id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	DOM         string `json:"dom"`
	StrippedDOM string `json:"stripped_dom"`
	Depth       int    `json:"depth"`
	IsRoot      bool   `json:"is_root"`

	// NavigationAction is actions taken to reach this state
	NavigationAction *Action `json:"navigation_actions"`
}

// Action is a action taken in the browser
type Action struct {
	OriginID string       `json:"origin_id"`
	Type     ActionType   `json:"type"`
	Input    string       `json:"input"`
	Element  *HTMLElement `json:"element"`
	Form     *HTMLForm    `json:"form"`
	Depth    int          `json:"depth"`
	ResultID string       `json:"result_id"`
}

func (a *Action) Hash() string {
	if a.Element != nil {
		return a.Element.Hash()
	}
	if a.Form != nil {
		return a.Form.Hash()
	}
	return ""
}

func (a *Action) String() string {
	var builder strings.Builder
	builder.WriteString(string(a.Type))
	if a.Type == ActionTypeLoadURL {
		builder.WriteString(fmt.Sprintf(" %s", a.Input))
	}
	if a.Element != nil {
		builder.WriteString(fmt.Sprintf(" on %s", a.Element))
	}
	value := builder.String()
	return value
}

type ActionType string

const (
	ActionTypeUnknown         ActionType = "unknown"
	ActionTypeLoadURL         ActionType = "load_url"
	ActionTypeExecuteJS       ActionType = "execute_js"
	ActionTypeLeftClick       ActionType = "left_click"
	ActionTypeLeftClickDown   ActionType = "left_click_down"
	ActionTypeLeftClickUp     ActionType = "left_click_up"
	ActionTypeRightClick      ActionType = "right_click"
	ActionTypeDoubleClick     ActionType = "double_click"
	ActionTypeScroll          ActionType = "scroll"
	ActionTypeSendKeys        ActionType = "send_keys"
	ActionTypeKeyUp           ActionType = "key_up"
	ActionTypeKeyDown         ActionType = "key_down"
	ActionTypeHover           ActionType = "hover"
	ActionTypeFocus           ActionType = "focus"
	ActionTypeBlur            ActionType = "blur"
	ActionTypeMouseOverAndOut ActionType = "mouse_over_and_out"
	ActionTypeMouseWheel      ActionType = "mouse_wheel"
	ActionTypeFillForm        ActionType = "fill_form"
	ActionTypeWait            ActionType = "wait"
	ActionTypeRedirect        ActionType = "redirect"
	ActionTypeSubRequest      ActionType = "sub_request"
)

func ActionFromEventListener(listener *EventListener) *Action {
	var actionType ActionType

	switch listener.Type {
	case "click":
		actionType = ActionTypeLeftClick
	case "mousedown":
		actionType = ActionTypeLeftClickDown
	case "mouseup":
		actionType = ActionTypeLeftClickUp
	case "keydown":
		actionType = ActionTypeKeyDown
	case "keyup":
		actionType = ActionTypeKeyUp
	case "focus":
		actionType = ActionTypeFocus
	case "blur":
		actionType = ActionTypeBlur
	case "scroll":
		actionType = ActionTypeScroll
	case "dblclick":
		actionType = ActionTypeDoubleClick
	case "contextmenu":
		actionType = ActionTypeRightClick
	case "mouseover", "mouseout":
		actionType = ActionTypeMouseOverAndOut
	case "wheel":
		actionType = ActionTypeMouseWheel
	}

	return &Action{
		Type:    actionType,
		Element: listener.Element,
	}
}

// HTMLElement represents a DOM element
type HTMLElement struct {
	TagName     string            `json:"tagName"`
	ID          string            `json:"id"`
	Classes     string            `json:"classes"`
	Attributes  map[string]string `json:"attributes"`
	Hidden      bool              `json:"hidden"`
	OuterHTML   string            `json:"outerHTML"`
	Type        string            `json:"type"`
	Value       string            `json:"value"`
	CSSSelector string            `json:"cssSelector"`
	XPath       string            `json:"xpath"`
	TextContent string            `json:"textContent"`
	MD5Hash     string            `json:"md5Hash"`
}

func (e *HTMLElement) String() string {
	var builder strings.Builder
	builder.WriteString(e.TagName)
	if e.ID != "" {
		builder.WriteString(fmt.Sprintf("#%s", e.ID))
	}
	if e.Classes != "" {
		builder.WriteString(fmt.Sprintf(".%s", e.Classes))
	}
	if e.TextContent != "" {
		e.TextContent = strings.Trim(e.TextContent, " \n\r\t")
		builder.WriteString(fmt.Sprintf(" (%s)", e.TextContent))
	}
	value := builder.String()
	return value
}

var (
	dynamicIDAndClassPattern = regexp.MustCompile(`^\d+$|^id_\d+|\bclass_\w{8}\b`)
)

func (e *HTMLElement) Hash() string {
	hasher := md5.New()

	var parts []string
	// Use stable identifiers
	parts = append(parts, e.TagName)

	if e.ID != "" {
		parts = append(parts, "id:"+e.ID)
	}
	if e.Classes != "" {
		parts = append(parts, "class:"+e.Classes)
	}

	// Add stable attributes
	stableAttrs := getStableAttributes(e.Attributes)
	for _, k := range stableAttrs {
		parts = append(parts, fmt.Sprintf("%s:%s", k, e.Attributes[k]))
	}

	hashInput := strings.Join(parts, "|")
	if IsDiagnosticEnabled {
		fmt.Printf("[diagnostic] Element hash input: %s\n", hashInput)
	}
	hasher.Write([]byte(hashInput))
	return hex.EncodeToString(hasher.Sum(nil))
}

// HTMLForm represents a form element
type HTMLForm struct {
	TagName     string            `json:"tagName"`
	ID          string            `json:"id"`
	Classes     string            `json:"classes"`
	Attributes  map[string]string `json:"attributes"`
	Hidden      bool              `json:"hidden"`
	OuterHTML   string            `json:"outerHTML"`
	Action      string            `json:"action"`
	Method      string            `json:"method"`
	Elements    []*HTMLElement    `json:"elements"`
	CSSSelector string            `json:"cssSelector"`
	XPath       string            `json:"xpath"`
}

func (f *HTMLForm) Hash() string {
	hasher := md5.New()

	var parts []string
	parts = append(parts, f.TagName)

	if f.ID != "" {
		parts = append(parts, "id:"+f.ID)
	}
	if f.Classes != "" {
		parts = append(parts, "class:"+f.Classes)
	}

	// Add stable attributes
	stableAttrs := getStableAttributes(f.Attributes)
	for _, k := range stableAttrs {
		parts = append(parts, fmt.Sprintf("%s:%s", k, f.Attributes[k]))
	}
	parts = append(parts, fmt.Sprintf("action:%s", f.Action), fmt.Sprintf("method:%s", f.Method))

	// Include hashes of form elements
	for _, element := range f.Elements {
		parts = append(parts, element.Hash())
	}

	hashInput := strings.Join(parts, "|")
	if IsDiagnosticEnabled {
		fmt.Printf("[diagnostic] Form hash input: %s\n", hashInput)
	}
	hasher.Write([]byte(hashInput))
	return hex.EncodeToString(hasher.Sum(nil))
}

// getStableAttributes returns a sorted slice of attribute keys that are considered stable.
func getStableAttributes(attrs map[string]string) []string {
	stableKeys := map[string]struct{}{
		"id":          {},
		"name":        {},
		"type":        {},
		"href":        {},
		"src":         {},
		"action":      {},
		"method":      {},
		"placeholder": {},
	}

	var stableAttrs []string
	for key := range attrs {
		if _, exists := stableKeys[key]; exists {
			stableAttrs = append(stableAttrs, key)
		}
	}

	// Sort the attributes to ensure consistent order for hashing.
	sort.Strings(stableAttrs)

	return stableAttrs
}

type EventListener struct {
	Element  *HTMLElement `json:"element"`
	Type     string       `json:"type"`
	Listener string       `json:"listener"`
}

// NavigationType represents the type of navigation
type NavigationType string

const (
	// NavigationTypeForm represents a form navigation
	NavigationTypeForm NavigationType = "form"
	// NavigationTypeButton represents a button navigation
	NavigationTypeButton NavigationType = "button"
	// NavigationTypeLink represents a link navigation
	NavigationTypeLink NavigationType = "link"
	// NavigationTypeEventListener represents an event listener navigation
	NavigationTypeEventListener NavigationType = "eventlistener"
)
