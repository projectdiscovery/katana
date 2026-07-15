package auth

import "strings"

// LoginStep is a single action in a multi-step headless login flow.
// In Value, the placeholders {{username}} and {{password}} are substituted with
// the configured credentials so secrets stay out of the step list / recording.
type LoginStep struct {
	// Action is one of: navigate, fill, click, waitvisible, wait, press, submit.
	Action string `json:"action" yaml:"action"`
	// Selector is a CSS selector (or xpath=… expression) the action targets.
	// Unused by navigate/wait.
	Selector string `json:"selector" yaml:"selector"`
	// Value is the action argument: a URL (navigate), text to type (fill), a key
	// name such as "enter"/"tab" (press), or a duration like "2s" (wait).
	Value string `json:"value" yaml:"value"`
}

// FirstNavigateURL returns the URL of the first navigate step, or "".
func FirstNavigateURL(steps []LoginStep) string {
	for _, s := range steps {
		if strings.EqualFold(s.Action, "navigate") && s.Value != "" {
			return s.Value
		}
	}
	return ""
}

// NeedsCredentials reports whether any step value still contains an unresolved
// credential placeholder that must be supplied via -auto-login.
func NeedsCredentials(steps []LoginStep) bool {
	for _, s := range steps {
		if strings.Contains(s.Value, "{{username}}") || strings.Contains(s.Value, "{{password}}") {
			return true
		}
	}
	return false
}

// ExpandCredentials substitutes {{username}}/{{password}} placeholders.
func ExpandCredentials(value, username, password string) string {
	value = strings.ReplaceAll(value, "{{username}}", username)
	value = strings.ReplaceAll(value, "{{password}}", password)
	return value
}
