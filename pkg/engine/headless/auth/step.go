package auth

import (
	"net/url"
	"strings"
	"time"

	"github.com/projectdiscovery/utils/errkit"
)

// LoginStep is a single action in a multi-step headless login flow.
// In Value, the placeholders {{username}} and {{password}} are substituted with
// the configured credentials so secrets stay out of the step list / recording.
type LoginStep struct {
	// Action is one of: navigate, fill, click, doubleclick, waitvisible, wait,
	// press, submit.
	Action string `json:"action" yaml:"action"`
	// Selector is a CSS selector (or xpath=… expression) the action targets.
	// Unused by navigate/wait.
	Selector string `json:"selector" yaml:"selector"`
	// Value is the action argument: a URL (navigate), text to type (fill), a key
	// name such as "enter"/"tab" (press), or a duration like "2s" (wait).
	Value string `json:"value" yaml:"value"`
}

// FirstNavigateURL returns the first absolute HTTP(S) navigate URL, or "".
// Recorder exports can contain an initial about:blank navigation, which is not
// a useful anonymous-session baseline.
func FirstNavigateURL(steps []LoginStep) string {
	_, navigateURL := stepsAfterFirstNavigateURL(steps)
	return navigateURL
}

// StepsAfterFirstNavigateURL returns the replayable suffix after the first
// absolute HTTP(S) navigation. The caller can navigate once, capture the
// anonymous cookie baseline, and avoid loading the login page a second time.
func StepsAfterFirstNavigateURL(steps []LoginStep) ([]LoginStep, string) {
	return stepsAfterFirstNavigateURL(steps)
}

func stepsAfterFirstNavigateURL(steps []LoginStep) ([]LoginStep, string) {
	for i, s := range steps {
		if !strings.EqualFold(s.Action, "navigate") {
			continue
		}
		parsed, err := url.Parse(strings.TrimSpace(s.Value))
		if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return steps[i+1:], parsed.String()
		}
	}
	return nil, ""
}

// HasTerminalVisibleAssertion reports whether the flow ends by asserting an
// authenticated-only element. This is an explicit success signal for
// applications whose server-side session does not mutate browser state.
func HasTerminalVisibleAssertion(steps []LoginStep) bool {
	if len(steps) == 0 {
		return false
	}
	last := steps[len(steps)-1]
	return strings.EqualFold(strings.TrimSpace(last.Action), "waitvisible") &&
		strings.TrimSpace(last.Selector) != ""
}

// ValidateSteps rejects flows that cannot be replayed deterministically.
func ValidateSteps(steps []LoginStep) error {
	if len(steps) == 0 {
		return errkit.New("recorded-flow: no steps to replay")
	}
	for i, step := range steps {
		action := strings.ToLower(strings.TrimSpace(step.Action))
		switch action {
		case "navigate":
			if strings.TrimSpace(step.Value) == "" {
				return errkit.Newf("recorded-flow: step %d (navigate) missing URL", i)
			}
		case "fill", "input", "type", "click", "doubleclick", "waitvisible":
			if strings.TrimSpace(step.Selector) == "" {
				return errkit.Newf("recorded-flow: step %d (%s) missing selector", i, action)
			}
		case "press":
			if _, err := keyFromName(step.Value); err != nil {
				return errkit.Wrapf(err, "recorded-flow: step %d (press)", i)
			}
		case "wait":
			if step.Value != "" {
				duration, err := time.ParseDuration(step.Value)
				if err != nil || duration < 0 {
					return errkit.Newf("recorded-flow: step %d (wait) has invalid duration %q", i, step.Value)
				}
			}
		case "submit":
			// Selector is optional: replay falls back to a submit control or
			// Enter on the visible password field.
		default:
			return errkit.Newf("recorded-flow: step %d has unknown action %q", i, step.Action)
		}
	}
	return nil
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
