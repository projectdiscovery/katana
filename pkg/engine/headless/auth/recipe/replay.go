package recipe

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// ValidateRecipe checks if the recipe's selectors still exist on the current page.
// Returns false if any required selector (with no fallback) is missing.
func ValidateRecipe(page *rod.Page, r *Recipe) bool {
	for _, step := range r.Steps {
		if step.Selector == "" {
			continue // navigate/wait/press_enter steps have no selector
		}
		el, err := page.Element(step.Selector)
		if err != nil || el == nil {
			if step.Fallback != "" {
				el, err = page.Element(step.Fallback)
				if err != nil || el == nil {
					return false
				}
			} else {
				return false
			}
		}
	}
	return true
}

// ReplayRecipe executes a cached recipe on a browser page.
// Substitutes {{username}} and {{password}} placeholders at execution time.
// Returns error if any step fails (caller should invalidate and re-learn).
func ReplayRecipe(page *rod.Page, r *Recipe, username, password string) error {
	for i, step := range r.Steps {
		value := SubstituteCredentials(step.Value, username, password)

		switch step.Action {
		case "navigate":
			if err := page.Navigate(value); err != nil {
				return fmt.Errorf("step %d (navigate): %w", i, err)
			}
			waitForLoad(page, 5*time.Second)

		case "clear":
			el, err := findElement(page, step.Selector, step.Fallback)
			if err != nil {
				return fmt.Errorf("step %d (clear): %w", i, err)
			}
			_ = el.SelectAllText()
			_ = el.Input("")

		case "type":
			el, err := findElement(page, step.Selector, step.Fallback)
			if err != nil {
				return fmt.Errorf("step %d (type): %w", i, err)
			}
			if err := el.Input(value); err != nil {
				return fmt.Errorf("step %d (type input): %w", i, err)
			}

		case "click":
			el, err := findElement(page, step.Selector, step.Fallback)
			if err != nil {
				return fmt.Errorf("step %d (click): %w", i, err)
			}
			if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
				return fmt.Errorf("step %d (click exec): %w", i, err)
			}

		case "select":
			el, err := findElement(page, step.Selector, step.Fallback)
			if err != nil {
				return fmt.Errorf("step %d (select): %w", i, err)
			}
			if err := el.Select([]string{value}, true, rod.SelectorTypeText); err != nil {
				return fmt.Errorf("step %d (select exec): %w", i, err)
			}

		case "press_enter":
			if err := page.Keyboard.Press(input.Enter); err != nil {
				return fmt.Errorf("step %d (press_enter): %w", i, err)
			}

		case "wait":
			if value == "navigation" {
				waitForLoad(page, 5*time.Second)
			} else {
				dur, err := time.ParseDuration(value)
				if err != nil {
					dur = 1 * time.Second
				}
				time.Sleep(dur)
			}

		default:
			return fmt.Errorf("step %d: unknown action %q", i, step.Action)
		}
	}
	return nil
}

// findElement tries the primary CSS selector, then the fallback.
func findElement(page *rod.Page, selector, fallback string) (*rod.Element, error) {
	el, err := page.Element(selector)
	if err == nil && el != nil {
		return el, nil
	}
	if fallback != "" {
		el, err = page.Element(fallback)
		if err == nil && el != nil {
			return el, nil
		}
	}
	return nil, fmt.Errorf("element not found: %s (fallback: %s)", selector, fallback)
}

// waitForLoad waits for a page to finish loading after navigation.
// Uses WaitLoad with a timeout instead of WaitStable which blocks
// indefinitely on pages with animations or live content.
func waitForLoad(page *rod.Page, timeout time.Duration) {
	chained := page.Timeout(timeout)
	_ = chained.WaitLoad()
	// Brief extra settle time for JS frameworks to initialize
	time.Sleep(500 * time.Millisecond)
}
