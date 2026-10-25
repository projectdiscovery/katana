package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/utils/errkit"
)

// defaultSettle is how long to wait for the page to stabilize between steps.
const defaultSettle = 5 * time.Second

// RunLoginSteps replays an explicit multi-step login flow against a rod page.
// Cookies / storage set during replay remain on the browser context for the
// subsequent crawl. settle bounds waitvisible / inter-step WaitStable; zero
// uses a 5s default.
func RunLoginSteps(ctx context.Context, page *rod.Page, steps []LoginStep, username, password string, settle time.Duration) error {
	if page == nil {
		return errkit.New("recorded-flow: page is nil")
	}
	if err := ValidateSteps(steps); err != nil {
		return err
	}
	if settle <= 0 {
		settle = defaultSettle
	}

	for i, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Bind rod operations to ctx so cancellation is honored mid-step. The
		// settle budget bounds waits only: element interaction on a loaded CI
		// machine can take longer than a wait is allowed to.
		stepPage := page.Context(ctx)
		action := strings.ToLower(strings.TrimSpace(step.Action))
		switch action {
		case "navigate":
			if err := stepPage.Navigate(step.Value); err != nil {
				return errkit.Wrapf(err, "recorded-flow step %d (navigate): failed", i)
			}
			if err := stepPage.Timeout(settle).WaitLoad(); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return ctxErr
				}
				// A page that never fires load can still be interactable, so
				// the following steps decide whether the flow really broke.
			}
		case "fill", "input", "type":
			el := findVisible(stepPage, byName(step.Selector), step.Selector)
			if el == nil {
				return errkit.Newf("recorded-flow step %d (fill): element not found: %s", i, step.Selector)
			}
			if err := typeInto(el, ExpandCredentials(step.Value, username, password)); err != nil {
				return errkit.Wrapf(err, "recorded-flow step %d (fill): failed", i)
			}
		case "click", "doubleclick":
			el := findVisible(stepPage, step.Selector)
			if el == nil {
				return errkit.Newf("recorded-flow step %d (%s): element not found: %s", i, action, step.Selector)
			}
			_ = el.ScrollIntoView()
			clickCount := 1
			if action == "doubleclick" {
				clickCount = 2
			}
			if err := el.Click(proto.InputMouseButtonLeft, clickCount); err != nil {
				return errkit.Wrapf(err, "recorded-flow step %d (%s): failed", i, action)
			}
		case "waitvisible":
			if err := waitVisible(ctx, stepPage, step.Selector, settle); err != nil {
				return errkit.Wrapf(err, "recorded-flow step %d (waitvisible)", i)
			}
		case "wait":
			d := settle
			if step.Value != "" {
				if pd, perr := time.ParseDuration(step.Value); perr == nil {
					d = pd
				}
			}
			select {
			case <-time.After(d):
			case <-ctx.Done():
				return ctx.Err()
			}
		case "press":
			key, kerr := keyFromName(step.Value)
			if kerr != nil {
				return errkit.Wrapf(kerr, "recorded-flow step %d (press)", i)
			}
			if step.Selector != "" {
				el := findVisible(stepPage, byName(step.Selector), step.Selector)
				if el == nil {
					return errkit.Newf("recorded-flow step %d (press): element not found: %s", i, step.Selector)
				}
				if err := el.Type(key); err != nil {
					return errkit.Wrapf(err, "recorded-flow step %d (press): failed", i)
				}
			} else if err := stepPage.Keyboard.Type(key); err != nil {
				return errkit.Wrapf(err, "recorded-flow step %d (press): failed", i)
			}
		case "submit":
			fallback := findVisible(stepPage, `input[type="password"]`)
			if step.Selector != "" {
				if el := findVisible(stepPage, byName(step.Selector), step.Selector); el != nil {
					fallback = el
					_ = el.ScrollIntoView()
					if err := el.Click(proto.InputMouseButtonLeft, 1); err == nil {
						break
					}
				}
			}
			if err := submitForm(stepPage, fallback, settle); err != nil {
				return errkit.Wrapf(err, "recorded-flow step %d (submit)", i)
			}
		default:
			return errkit.Newf("recorded-flow step %d: unknown action %q", i, step.Action)
		}
		_ = rod.Try(func() { stepPage.Timeout(settle).MustWaitStable() })
	}
	return nil
}

func keyFromName(name string) (input.Key, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "enter", "return":
		return input.Enter, nil
	case "tab":
		return input.Tab, nil
	case "escape", "esc":
		return input.Escape, nil
	case "space":
		return input.Space, nil
	default:
		return 0, errkit.Newf("unsupported key %q (supported: enter, tab, escape, space)", name)
	}
}

func waitVisible(ctx context.Context, page *rod.Page, selector string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if findVisible(page, selector) != nil {
			return nil
		}
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errkit.Newf("timeout waiting for element to become visible: %s", selector)
}

func submitForm(page *rod.Page, fallbackField *rod.Element, settle time.Duration) error {
	clickTimeout := settle
	if clickTimeout <= 0 {
		clickTimeout = 5 * time.Second
	}
	if btn := findVisible(page, `button[type="submit"]`, `input[type="submit"]`, `button:not([type])`, `[role="button"]`); btn != nil {
		clickErr := rod.Try(func() {
			b := btn.Timeout(clickTimeout)
			b.MustWaitInteractable()
			_ = b.ScrollIntoView()
			b.MustClick()
		})
		if clickErr == nil {
			return nil
		}
		if fallbackField == nil {
			return errkit.Wrapf(clickErr, "recorded-flow: submit button never became interactable")
		}
	}
	if fallbackField != nil {
		if err := fallbackField.Type(input.Enter); err != nil {
			return errkit.Wrap(err, "recorded-flow: failed to submit form")
		}
		return nil
	}
	return errkit.New("recorded-flow: no submit control found and no field to submit")
}

func byName(name string) string {
	if name == "" || strings.HasPrefix(name, "xpath=") || strings.HasPrefix(name, "//") || strings.HasPrefix(name, "(//") {
		return ""
	}
	// Avoid wrapping an already-qualified CSS selector as input[name=...].
	if strings.ContainsAny(name, "#[.[:") {
		return ""
	}
	return fmt.Sprintf(`input[name=%q]`, name)
}

func findVisible(page *rod.Page, selectors ...string) *rod.Element {
	for _, sel := range selectors {
		if sel == "" {
			continue
		}
		els, err := queryElements(page, sel)
		if err != nil {
			continue
		}
		for _, el := range els {
			if vis, verr := el.Visible(); verr == nil && vis {
				return el
			}
		}
	}
	return nil
}

func queryElements(page *rod.Page, sel string) (rod.Elements, error) {
	switch {
	case strings.HasPrefix(sel, "xpath="):
		return page.ElementsX(strings.TrimPrefix(sel, "xpath="))
	case strings.HasPrefix(sel, "//"), strings.HasPrefix(sel, "(//"):
		return page.ElementsX(sel)
	default:
		return page.Elements(sel)
	}
}

func typeInto(el *rod.Element, text string) error {
	_ = el.ScrollIntoView()
	if err := el.Focus(); err != nil {
		return err
	}
	_ = el.SelectAllText()
	return el.Input(text)
}
