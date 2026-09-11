package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const chromeRecording = `{
  "title": "login",
  "steps": [
    {"type": "setViewport", "width": 1280, "height": 720},
    {"type": "navigate", "url": "https://app.example.com/login"},
    {"type": "change", "value": "dave@example.com", "selectors": [["#email"], ["aria/Email"], ["xpath///*[@id=\"email\"]"]]},
    {"type": "click", "selectors": [["#next"], ["aria/Next"]]},
    {"type": "waitForElement", "selectors": [["#password"]]},
    {"type": "change", "value": "p@ss", "selectors": [["#password"]]},
    {"type": "keyDown", "key": "Enter"},
    {"type": "keyUp", "key": "Enter"},
    {"type": "click", "selectors": [["#submit"]]}
  ]
}`

func TestStepsFromRecording_MapsAndParameterizes(t *testing.T) {
	steps, err := StepsFromRecording([]byte(chromeRecording), "dave@example.com", "p@ss")
	require.NoError(t, err)

	require.Equal(t, []LoginStep{
		{Action: "navigate", Value: "https://app.example.com/login"},
		{Action: "fill", Selector: "#email", Value: "{{username}}"},
		{Action: "click", Selector: "#next"},
		{Action: "waitvisible", Selector: "#password"},
		{Action: "fill", Selector: "#password", Value: "{{password}}"},
		{Action: "press", Value: "enter"},
		{Action: "click", Selector: "#submit"},
	}, steps)
}

func TestStepsFromRecording_PasswordSelectorMaskedWithoutCredMatch(t *testing.T) {
	rec := `{"steps": [
		{"type": "change", "value": "literal-secret", "selectors": [["input[type=password]"]]}
	]}`
	steps, err := StepsFromRecording([]byte(rec), "", "")
	require.NoError(t, err)
	require.Len(t, steps, 1)
	require.Equal(t, "{{password}}", steps[0].Value)
}

func TestStepsFromRecording_UsernameSelectorUsesConfiguredCredential(t *testing.T) {
	rec := `{"steps": [
		{"type": "change", "value": "recorded-old-user", "selectors": [["input[name=email]"]]}
	]}`
	steps, err := StepsFromRecording([]byte(rec), "new-user@example.com", "new-password")
	require.NoError(t, err)
	require.Equal(t, "{{username}}", steps[0].Value)
	require.Equal(t, "new-user@example.com",
		ExpandCredentials(steps[0].Value, "new-user@example.com", "new-password"))
}

func TestStepsFromRecording_SelectorPriority(t *testing.T) {
	rec := `{"steps": [
		{"type": "click", "selectors": [["aria/Save"], ["xpath///button[1]"], ["#real"]]},
		{"type": "click", "selectors": [["pierce/#shadow"], ["xpath///div[2]"]]},
		{"type": "click", "selectors": [["aria/Submit"]]}
	]}`
	steps, err := StepsFromRecording([]byte(rec), "", "")
	require.NoError(t, err)
	require.Equal(t, "#real", steps[0].Selector)
	require.Equal(t, "xpath=//div[2]", steps[1].Selector)
	require.Equal(t, `[aria-label="Submit"]`, steps[2].Selector)
}

func TestStepsFromRecording_Errors(t *testing.T) {
	_, err := StepsFromRecording([]byte(`not json`), "", "")
	require.Error(t, err)

	_, err = StepsFromRecording([]byte(`{"steps": []}`), "", "")
	require.Error(t, err)

	_, err = StepsFromRecording([]byte(`{"steps": [{"type":"setViewport"}]}`), "", "")
	require.Error(t, err)

	_, err = StepsFromRecording([]byte(`{"steps": [{"type":"click","selectors":[]}]}`), "", "")
	require.ErrorContains(t, err, "no supported selector")

	_, err = StepsFromRecording([]byte(`{"steps": [{"type":"evaluate","expression":"alert(1)"}]}`), "", "")
	require.ErrorContains(t, err, "unsupported type")

	_, err = StepsFromData([]byte(`{"steps": [
		{"type":"navigate","url":"https://example.com"},
		{"action":"click","selector":"#submit"}
	]}`), "", "")
	require.ErrorContains(t, err, "unsupported type")
}

func TestStepsFromRecording_PreservesDoubleClick(t *testing.T) {
	steps, err := StepsFromRecording([]byte(`{"steps": [
		{"type":"doubleClick","selectors":[["#continue"]]}
	]}`), "", "")
	require.NoError(t, err)
	require.Equal(t, []LoginStep{{Action: "doubleclick", Selector: "#continue"}}, steps)
}

func TestFirstNavigateURL(t *testing.T) {
	steps := []LoginStep{
		{Action: "navigate", Value: "about:blank"},
		{Action: "fill", Selector: "#x"},
		{Action: "navigate", Value: "https://app.example.com/login"},
		{Action: "navigate", Value: "https://second"},
	}
	require.Equal(t, "https://app.example.com/login", FirstNavigateURL(steps))
	require.Equal(t, "", FirstNavigateURL([]LoginStep{{Action: "click"}}))

	remaining, navigateURL := StepsAfterFirstNavigateURL(steps)
	require.Equal(t, "https://app.example.com/login", navigateURL)
	require.Equal(t, []LoginStep{{Action: "navigate", Value: "https://second"}}, remaining)
}

func TestStepsFromData_Explicit(t *testing.T) {
	raw := `{
		"steps": [
			{"action": "navigate", "value": "https://app.example.com/login"},
			{"action": "fill", "selector": "#email", "value": "{{username}}"},
			{"action": "fill", "selector": "#password", "value": "{{password}}"},
			{"action": "click", "selector": "#submit"}
		]
	}`
	steps, err := StepsFromData([]byte(raw), "u", "p")
	require.NoError(t, err)
	require.Len(t, steps, 4)
	require.Equal(t, "fill", steps[1].Action)
	require.True(t, NeedsCredentials(steps))
}

func TestStepsFromData_Chrome(t *testing.T) {
	steps, err := StepsFromData([]byte(chromeRecording), "dave@example.com", "p@ss")
	require.NoError(t, err)
	require.NotEmpty(t, steps)
	require.Equal(t, "navigate", steps[0].Action)
}

func TestNeedsCredentials(t *testing.T) {
	require.False(t, NeedsCredentials([]LoginStep{{Action: "click", Selector: "#x"}}))
	require.True(t, NeedsCredentials([]LoginStep{{Action: "fill", Value: "{{password}}"}}))
}

func TestValidateSteps(t *testing.T) {
	valid := []LoginStep{
		{Action: "navigate", Value: "https://example.com/login"},
		{Action: "fill", Selector: "#email", Value: "{{username}}"},
		{Action: "press", Value: "enter"},
		{Action: "wait", Value: "250ms"},
		{Action: "submit"},
	}
	require.NoError(t, ValidateSteps(valid))

	tests := []struct {
		name string
		step LoginStep
		want string
	}{
		{name: "unknown action", step: LoginStep{Action: "eval"}, want: "unknown action"},
		{name: "missing selector", step: LoginStep{Action: "fill"}, want: "missing selector"},
		{name: "missing url", step: LoginStep{Action: "navigate"}, want: "missing URL"},
		{name: "invalid key", step: LoginStep{Action: "press", Value: "delete"}, want: "unsupported key"},
		{name: "invalid wait", step: LoginStep{Action: "wait", Value: "later"}, want: "invalid duration"},
		{name: "negative wait", step: LoginStep{Action: "wait", Value: "-1s"}, want: "invalid duration"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorContains(t, ValidateSteps([]LoginStep{test.step}), test.want)
		})
	}
	require.ErrorContains(t, ValidateSteps(nil), "no steps")
}

func TestHasTerminalVisibleAssertion(t *testing.T) {
	require.True(t, HasTerminalVisibleAssertion([]LoginStep{
		{Action: "click", Selector: "#submit"},
		{Action: "waitvisible", Selector: "#dashboard"},
	}))
	require.False(t, HasTerminalVisibleAssertion([]LoginStep{
		{Action: "waitvisible", Selector: "#password"},
		{Action: "click", Selector: "#submit"},
	}))
	require.False(t, HasTerminalVisibleAssertion(nil))
}
