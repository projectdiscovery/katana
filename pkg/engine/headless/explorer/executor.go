package explorer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// ExecutePlan runs a planned action list using the page snapshot for ref-based element targeting.
// Returns discovered navigation actions to feed back into the crawl queue.
func ExecutePlan(page *rod.Page, plan *ExplorationPlan, snapshot *PageSnapshot) []*types.Action {
	if plan == nil || len(plan.Actions) == 0 {
		return nil
	}

	// Sort by priority descending
	actions := make([]PlannedAction, len(plan.Actions))
	copy(actions, plan.Actions)
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority > actions[j].Priority
	})

	gologger.Info().Msgf("[executor] Executing %d planned actions", len(actions))

	var discovered []*types.Action
	executed := 0
	failed := 0

	for _, action := range actions {
		if action.Priority == 0 {
			continue
		}

		var err error
		switch action.Action {
		case "expand_menu":
			err = executeRefClick(page, snapshot, action, "EXPAND")
		case "fill_and_submit":
			err = executeRefFillAndSubmit(page, snapshot, action, plan.FormFillValues)
		case "click":
			err = executeRefClick(page, snapshot, action, "CLICK")
			if err == nil && action.FollowFlow {
				executeFollowFlow(page, action, plan.MultiStepFlows)
			}
		case "sample_pagination":
			gologger.Info().Msgf("[executor] [P%d] PAGINATION — sample pages %v (%s)", action.Priority, action.Pages, action.Reason)
		case "skip":
			continue
		default:
			gologger.Info().Msgf("[executor] [P%d] UNKNOWN action: %s", action.Priority, action.Action)
			continue
		}

		if err != nil {
			failed++
			gologger.Info().Msgf("[executor] [P%d] FAILED %s %s — %s", action.Priority, action.Action, trunc(action.Selector, 40), err)
		} else if action.Action != "sample_pagination" {
			executed++
		}
	}

	gologger.Info().Msgf("[executor] Done — %d executed, %d failed", executed, failed)
	return discovered
}

func executeRefClick(page *rod.Page, snapshot *PageSnapshot, action PlannedAction, label string) error {
	if snapshot != nil && action.Ref > 0 {
		if err := snapshot.ClickRef(action.Ref); err != nil {
			return fmt.Errorf("ref @%d: %w", action.Ref, err)
		}
		time.Sleep(1 * time.Second)
		url := ""
		if info, infoErr := page.Info(); infoErr == nil {
			url = info.URL
		}
		// Find the ref name for logging
		refName := fmt.Sprintf("@%d", action.Ref)
		for _, r := range snapshot.Refs {
			if r.Ref == action.Ref {
				refName = fmt.Sprintf("@%d [%s] \"%s\"", r.Ref, strings.ToLower(r.Tag), trunc(r.Name, 30))
				break
			}
		}
		gologger.Info().Msgf("[executor] [P%d] %s %s → %s — %s", action.Priority, label, refName, trunc(url, 50), action.Reason)
		return nil
	}
	// Fallback to CSS selector
	if action.Selector != "" {
		return executeClick(page, action)
	}
	return fmt.Errorf("no ref or selector")
}

func executeRefFillAndSubmit(page *rod.Page, snapshot *PageSnapshot, action PlannedAction, globalFillValues map[string]string) error {
	if snapshot == nil {
		return executeFillAndSubmit(page, action, globalFillValues)
	}

	filled := 0
	for refStr, fill := range action.Fields {
		var ref int
		fmt.Sscanf(refStr, "%d", &ref)
		if ref == 0 {
			continue
		}

		value := fill.Value
		if value == "" {
			// Try to find field name from snapshot for smart fill
			for _, form := range snapshot.Forms {
				for _, field := range form.Fields {
					if field.Ref == ref {
						value = smartFillValue(field.Name, field.Type, field.Placeholder)
						break
					}
				}
			}
		}
		if value == "" {
			value = "test"
		}

		switch fill.Type {
		case "select":
			if err := snapshot.SelectRef(ref, value); err != nil {
				gologger.Info().Msgf("[executor]   @%d select failed: %s", ref, err)
			} else {
				gologger.Info().Msgf("[executor]   @%d selected %q", ref, trunc(value, 20))
				filled++
			}
		default:
			if err := snapshot.TypeRef(ref, value); err != nil {
				gologger.Info().Msgf("[executor]   @%d type failed: %s", ref, err)
			} else {
				gologger.Info().Msgf("[executor]   @%d typed %q", ref, trunc(value, 20))
				filled++
			}
		}
	}

	if filled == 0 {
		return fmt.Errorf("no fields filled")
	}

	// Click submit — try form's submit button first
	submitted := false
	if action.FormRef > 0 {
		// Find submit button in the form's fields
		for _, form := range snapshot.Forms {
			if form.Ref == action.FormRef {
				for _, field := range form.Fields {
					if field.Type == "submit" || field.Tag == "BUTTON" {
						if err := snapshot.ClickRef(field.Ref); err == nil {
							submitted = true
							break
						}
					}
				}
			}
		}
	}
	if !submitted {
		clickSubmitButton(page)
	}

	time.Sleep(1 * time.Second)
	gologger.Info().Msgf("[executor] [P%d] FORM filled %d fields, submitted — %s", action.Priority, filled, action.Reason)
	return nil
}

func executeExpandMenu(page *rod.Page, action PlannedAction) error {
	el, err := page.Timeout(3 * time.Second).Element(action.Selector)
	if err != nil {
		return fmt.Errorf("element not found: %s", action.Selector)
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("click failed: %w", err)
	}
	time.Sleep(500 * time.Millisecond)
	gologger.Info().Msgf("[executor] [P%d] EXPAND %s — %s", action.Priority, trunc(action.Selector, 40), action.Reason)
	return nil
}

func executeFillAndSubmit(page *rod.Page, action PlannedAction, globalFillValues map[string]string) error {
	filled := 0
	for selector, fill := range action.Fields {
		value := fill.Value
		if value == "" {
			value = smartFillForSelector(selector, globalFillValues)
		}
		if value == "" {
			value = "test"
		}

		el, err := page.Timeout(3 * time.Second).Element(selector)
		if err != nil {
			gologger.Info().Msgf("[executor]   field not found: %s", selector)
			continue
		}

		switch fill.Type {
		case "select":
			if err := el.Select([]string{value}, true, rod.SelectorTypeText); err != nil {
				gologger.Info().Msgf("[executor]   select failed: %s — %s", selector, err)
			} else {
				gologger.Info().Msgf("[executor]   selected %q in %s", value, trunc(selector, 30))
				filled++
			}
		case "checkbox":
			if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
				gologger.Info().Msgf("[executor]   checkbox failed: %s — %s", selector, err)
			} else {
				filled++
			}
		default:
			_ = el.SelectAllText()
			if err := el.Input(value); err != nil {
				gologger.Info().Msgf("[executor]   input failed: %s — %s", selector, err)
			} else {
				gologger.Info().Msgf("[executor]   typed %q into %s", trunc(value, 20), trunc(selector, 30))
				filled++
			}
		}
	}

	if filled == 0 {
		return fmt.Errorf("no fields filled")
	}

	// Click submit
	submitted := false
	submitSelectors := []string{"button[type='submit']", "input[type='submit']", "button:not([type='reset']):not([type='button'])"}
	formPrefix := ""
	if action.FormSelector != "" {
		formPrefix = action.FormSelector + " "
	}
	for _, sel := range submitSelectors {
		el, err := page.Timeout(2 * time.Second).Element(formPrefix + sel)
		if err == nil {
			_ = el.Click(proto.InputMouseButtonLeft, 1)
			submitted = true
			break
		}
	}
	if !submitted {
		_ = page.Keyboard.Press(input.Enter)
	}

	time.Sleep(1 * time.Second)
	gologger.Info().Msgf("[executor] [P%d] FORM filled %d fields, submitted — %s", action.Priority, filled, action.Reason)
	return nil
}

func executeClick(page *rod.Page, action PlannedAction) error {
	el, err := page.Timeout(3 * time.Second).Element(action.Selector)
	if err != nil {
		return fmt.Errorf("element not found: %s", action.Selector)
	}
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("click failed: %w", err)
	}
	time.Sleep(1 * time.Second)

	// Get current URL after click
	url := ""
	if info, infoErr := page.Info(); infoErr == nil {
		url = info.URL
	}
	gologger.Info().Msgf("[executor] [P%d] CLICK %s → %s — %s", action.Priority, trunc(action.Selector, 40), trunc(url, 50), action.Reason)
	return nil
}

func executeFollowFlow(page *rod.Page, trigger PlannedAction, flows []FlowHint) {
	var hint *FlowHint
	for i, f := range flows {
		if f.Trigger == trigger.Selector {
			hint = &flows[i]
			break
		}
	}

	maxSteps := 10
	if hint != nil && hint.EstimatedSteps > 0 {
		maxSteps = hint.EstimatedSteps + 2
	}

	gologger.Info().Msgf("[executor] FLOW started from: %s", trunc(trigger.Selector, 50))

	for step := 0; step < maxSteps; step++ {
		time.Sleep(1 * time.Second)

		// Get current page state
		url := ""
		if info, err := page.Info(); err == nil {
			url = info.URL
		}

		pageType := detectFlowPageType(page)
		gologger.Info().Msgf("[executor] FLOW step %d: type=%s url=%s", step+1, pageType, trunc(url, 60))

		switch pageType {
		case "form":
			if hint != nil && step < len(hint.StepHints) {
				fillFlowStep(page, hint.StepHints[step])
				gologger.Info().Msgf("[executor] FLOW step %d: filled with hints — %s", step+1, hint.StepHints[step].Description)
			} else {
				fillFlowStepSmart(page)
				gologger.Info().Msgf("[executor] FLOW step %d: filled with smart defaults", step+1)
			}
			clickSubmitButton(page)

		case "confirmation":
			clickConfirmButton(page)
			gologger.Info().Msgf("[executor] FLOW step %d: confirmed", step+1)

		case "success":
			gologger.Info().Msgf("[executor] FLOW completed after %d steps (%s)", step+1, url)
			return

		case "error":
			gologger.Info().Msgf("[executor] FLOW error at step %d, aborting", step+1)
			return

		default:
			gologger.Info().Msgf("[executor] FLOW step %d: no form detected, ending flow", step+1)
			return
		}
	}
	gologger.Info().Msgf("[executor] FLOW max steps reached (%d)", maxSteps)
}

func detectFlowPageType(page *rod.Page) string {
	result, _ := page.Eval(`() => {
		const text = (document.body?.innerText || '').toLowerCase();
		if (text.includes('success') || text.includes('created') || text.includes('completed') || text.includes('done'))
			return 'success';
		if (text.includes('error') || text.includes('failed') || text.includes('invalid'))
			return 'error';
		if (text.includes('are you sure') || text.includes('confirm'))
			return 'confirmation';
		const inputs = document.querySelectorAll('input:not([type=hidden]):not([type=submit]):not([type=button]), textarea, select');
		if (inputs.length > 0)
			return 'form';
		return 'other';
	}`)
	if result != nil {
		return result.Value.Str()
	}
	return "other"
}

func fillFlowStep(page *rod.Page, hint StepHint) {
	for selector, value := range hint.Fill {
		el, err := page.Timeout(2 * time.Second).Element(selector)
		if err != nil {
			el, err = page.Timeout(1 * time.Second).Element(fmt.Sprintf("[name='%s']", selector))
			if err != nil {
				gologger.Info().Msgf("[executor]   flow field not found: %s", selector)
				continue
			}
		}
		_ = el.SelectAllText()
		_ = el.Input(value)
		gologger.Info().Msgf("[executor]   flow filled %s = %q", trunc(selector, 30), trunc(value, 20))
	}
}

func fillFlowStepSmart(page *rod.Page) {
	inputs, err := page.Elements("input:not([type='hidden']):not([type='submit']):not([type='button']), textarea, select")
	if err != nil {
		return
	}
	for _, el := range inputs {
		name, _ := el.Attribute("name")
		inputType, _ := el.Attribute("type")
		placeholder, _ := el.Attribute("placeholder")

		nameStr := attrStr(name)
		typeStr := attrStr(inputType)
		placeholderStr := attrStr(placeholder)

		tagResult, tagErr := el.Eval(`() => this.tagName`)
		if tagErr == nil && tagResult != nil && tagResult.Value.Str() == "SELECT" {
			_, _ = el.Eval(`() => { if (this.options.length > 1) this.selectedIndex = 1; }`)
			gologger.Info().Msgf("[executor]   flow selected option in %s", nameStr)
			continue
		}

		value := smartFillValue(nameStr, typeStr, placeholderStr)
		_ = el.SelectAllText()
		_ = el.Input(value)
		gologger.Info().Msgf("[executor]   flow filled %s = %q", nameStr, trunc(value, 20))
	}
}

func attrStr(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func clickSubmitButton(page *rod.Page) {
	for _, sel := range []string{"button[type='submit']", "input[type='submit']", "button"} {
		el, err := page.Timeout(2 * time.Second).Element(sel)
		if err == nil {
			_ = el.Click(proto.InputMouseButtonLeft, 1)
			return
		}
	}
	_ = page.Keyboard.Press(input.Enter)
}

func clickConfirmButton(page *rod.Page) {
	for _, sel := range []string{
		"button:has-text('Confirm')", "button:has-text('Yes')", "button:has-text('OK')",
		"button:has-text('Continue')", "button:has-text('Submit')", "button[type='submit']",
	} {
		el, err := page.Timeout(2 * time.Second).Element(sel)
		if err == nil {
			_ = el.Click(proto.InputMouseButtonLeft, 1)
			return
		}
	}
}

// Smart form fill patterns
var smartFillPatterns = []struct {
	pattern *regexp.Regexp
	value   string
}{
	{regexp.MustCompile(`(?i)(email|e-?mail)`), "test@example.com"},
	{regexp.MustCompile(`(?i)(name|full.?name|display.?name)`), "Test User"},
	{regexp.MustCompile(`(?i)(phone|mobile|tel)`), "+15550100"},
	{regexp.MustCompile(`(?i)(url|website|link|endpoint)`), "https://example.com"},
	{regexp.MustCompile(`(?i)(ip|ip.?addr|host$)`), "192.168.1.1"},
	{regexp.MustCompile(`(?i)(domain|hostname|fqdn)`), "example.com"},
	{regexp.MustCompile(`(?i)(port)`), "8080"},
	{regexp.MustCompile(`(?i)(target|scope|asset)`), "https://scanme.nmap.org"},
	{regexp.MustCompile(`(?i)(desc|description|note|comment|message)`), "Automated security test"},
	{regexp.MustCompile(`(?i)(company|org)`), "Test Corp"},
	{regexp.MustCompile(`(?i)(title|subject)`), "Test Entry"},
	{regexp.MustCompile(`(?i)(tag|label|category)`), "test"},
	{regexp.MustCompile(`(?i)(path|file|dir)`), "/tmp/test"},
	{regexp.MustCompile(`(?i)(cidr|range|subnet)`), "192.168.1.0/24"},
	{regexp.MustCompile(`(?i)(search|query|q|keyword)`), "test"},
	{regexp.MustCompile(`(?i)(count|limit|max|num)`), "10"},
}

func smartFillValue(name, fieldType, placeholder string) string {
	combined := strings.ToLower(name + " " + placeholder)
	for _, p := range smartFillPatterns {
		if p.pattern.MatchString(combined) {
			return p.value
		}
	}
	switch fieldType {
	case "email":
		return "test@example.com"
	case "number":
		return "10"
	case "url":
		return "https://example.com"
	case "tel":
		return "+15550100"
	}
	return "test"
}

func smartFillForSelector(selector string, globalValues map[string]string) string {
	sel := strings.ToLower(selector)
	for pattern, value := range globalValues {
		if strings.Contains(sel, strings.ToLower(pattern)) {
			return value
		}
	}
	return smartFillValue(selector, "", "")
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
