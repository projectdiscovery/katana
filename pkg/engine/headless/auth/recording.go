package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/projectdiscovery/utils/errkit"
)

// recordingFile is the Chrome DevTools Recorder / @puppeteer/replay export
// schema. A user records their real login in Chrome (DevTools → Recorder) and
// exports it as JSON; we compile it down to LoginSteps so multi-step / SSO
// logins can be authored by recording instead of hand-writing selectors.
type recordingFile struct {
	Title string          `json:"title"`
	Steps []recordingStep `json:"steps"`
}

// recordingStep is a single recorded user action. Selectors is a list of
// alternative selector strategies; we pick the most engine-friendly candidate.
type recordingStep struct {
	Type       string     `json:"type"`
	URL        string     `json:"url"`
	Selectors  [][]string `json:"selectors"`
	Value      string     `json:"value"`
	Key        string     `json:"key"`
	Target     string     `json:"target"`
	Expression string     `json:"expression"`
}

// explicitFile is a hand-authored step list (same LoginStep shape as replay).
type explicitFile struct {
	Steps []LoginStep `json:"steps"`
}

// StepsFromFile loads either a Chrome DevTools Recorder export or an explicit
// { "steps": [ { "action": ... } ] } JSON file.
func StepsFromFile(path, username, password string) ([]LoginStep, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errkit.Wrap(err, "recorded-flow: failed to read file")
	}
	return StepsFromData(data, username, password)
}

// StepsFromData converts a recording / explicit-step JSON payload into LoginSteps.
func StepsFromData(data []byte, username, password string) ([]LoginStep, error) {
	kind, err := detectStepFileKind(data)
	if err != nil {
		return nil, err
	}
	switch kind {
	case stepFileChrome:
		return StepsFromRecording(data, username, password)
	case stepFileExplicit:
		return StepsFromExplicit(data)
	default:
		return nil, errkit.New("recorded-flow: unsupported flow file format")
	}
}

type stepFileKind int

const (
	stepFileUnknown stepFileKind = iota
	stepFileChrome
	stepFileExplicit
)

func detectStepFileKind(data []byte) (stepFileKind, error) {
	var probe struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return stepFileUnknown, errkit.Wrap(err, "recorded-flow: invalid json")
	}
	if len(probe.Steps) == 0 {
		return stepFileUnknown, errkit.New("recorded-flow: flow contains no steps")
	}
	var first map[string]json.RawMessage
	if err := json.Unmarshal(probe.Steps[0], &first); err != nil {
		return stepFileUnknown, errkit.Wrap(err, "recorded-flow: invalid step")
	}
	if _, ok := first["type"]; ok {
		return stepFileChrome, nil
	}
	if _, ok := first["action"]; ok {
		return stepFileExplicit, nil
	}
	return stepFileUnknown, errkit.New("recorded-flow: steps must be Chrome Recorder (type) or explicit (action)")
}

// StepsFromExplicit loads a hand-authored LoginStep list.
func StepsFromExplicit(data []byte) ([]LoginStep, error) {
	var file explicitFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, errkit.Wrap(err, "recorded-flow: invalid explicit steps json")
	}
	if len(file.Steps) == 0 {
		return nil, errkit.New("recorded-flow: explicit steps list is empty")
	}
	for i, s := range file.Steps {
		if strings.TrimSpace(s.Action) == "" {
			return nil, errkit.Newf("recorded-flow: step %d missing action", i)
		}
	}
	return file.Steps, nil
}

// StepsFromRecording converts a Chrome DevTools Recorder export into LoginSteps.
// username/password are used to parameterize captured credential literals into
// {{username}}/{{password}} placeholders.
func StepsFromRecording(data []byte, username, password string) ([]LoginStep, error) {
	var rec recordingFile
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, errkit.Wrap(err, "recorded-flow: invalid recording json")
	}
	if len(rec.Steps) == 0 {
		return nil, errkit.New("recorded-flow: recording contains no steps")
	}

	var steps []LoginStep
	for _, rs := range rec.Steps {
		switch strings.ToLower(strings.TrimSpace(rs.Type)) {
		case "navigate":
			if rs.URL != "" {
				steps = append(steps, LoginStep{Action: "navigate", Value: rs.URL})
			}
		case "click", "doubleclick":
			sel := pickSelector(rs.Selectors)
			if sel == "" {
				continue
			}
			steps = append(steps, LoginStep{Action: "click", Selector: sel})
		case "change":
			sel := pickSelector(rs.Selectors)
			if sel == "" {
				continue
			}
			steps = append(steps, LoginStep{
				Action:   "fill",
				Selector: sel,
				Value:    parameterizeValue(rs.Value, sel, username, password),
			})
		case "keydown":
			// Character keys are captured by `change`; ignore paired keyUp.
			if key := normalizeKey(rs.Key); key != "" {
				steps = append(steps, LoginStep{Action: "press", Value: key, Selector: pickSelector(rs.Selectors)})
			}
		case "waitforelement":
			sel := pickSelector(rs.Selectors)
			if sel == "" {
				continue
			}
			steps = append(steps, LoginStep{Action: "waitvisible", Selector: sel})
		case "waitforexpression":
			// Arbitrary expressions are not evaluated; settle instead.
			steps = append(steps, LoginStep{Action: "wait"})
		case "setviewport", "keyup", "scroll", "close", "emulatenetworkconditions", "hover", "":
			continue
		default:
			continue
		}
	}

	if len(steps) == 0 {
		return nil, errkit.New("recorded-flow: recording produced no replayable steps")
	}
	return steps, nil
}

func normalizeKey(key string) string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "enter", "return", "numpadenter":
		return "enter"
	case "tab":
		return "tab"
	case "escape", "esc":
		return "escape"
	case "space":
		return "space"
	default:
		return ""
	}
}

func parameterizeValue(value, selector, username, password string) string {
	if password != "" && value == password {
		return "{{password}}"
	}
	if username != "" && value == username {
		return "{{username}}"
	}
	if looksLikePasswordSelector(selector) {
		return "{{password}}"
	}
	return value
}

func looksLikePasswordSelector(selector string) bool {
	s := strings.ToLower(selector)
	for _, marker := range []string{
		"password",
		"passwd",
		"pwd",
		"type=\"password\"",
		"type=password",
		"current-password",
		"new-password",
	} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

// pickSelector prefers native CSS, then XPath, then aria-label CSS, then text XPath.
// Shadow-piercing (pierce/) selectors are skipped.
func pickSelector(groups [][]string) string {
	var css, xpath, aria, text string
	for _, group := range groups {
		for _, s := range group {
			s = strings.TrimSpace(s)
			switch {
			case s == "":
				continue
			case strings.HasPrefix(s, "xpath/"):
				if xpath == "" {
					xpath = "xpath=" + strings.TrimPrefix(s, "xpath/")
				}
			case strings.HasPrefix(s, "aria/"):
				if aria == "" {
					aria = ariaToCSS(strings.TrimPrefix(s, "aria/"))
				}
			case strings.HasPrefix(s, "text/"):
				if text == "" {
					text = textToXPath(strings.TrimPrefix(s, "text/"))
				}
			case strings.HasPrefix(s, "pierce/"):
				continue
			default:
				if css == "" {
					css = s
				}
			}
		}
	}
	for _, candidate := range []string{css, xpath, aria, text} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func ariaToCSS(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, `"`) {
		return ""
	}
	return fmt.Sprintf(`[aria-label="%s"]`, name)
}

func textToXPath(t string) string {
	t = strings.TrimSpace(t)
	if t == "" || strings.Contains(t, `"`) {
		return ""
	}
	return fmt.Sprintf(`xpath=//*[contains(normalize-space(.), "%s")]`, t)
}
