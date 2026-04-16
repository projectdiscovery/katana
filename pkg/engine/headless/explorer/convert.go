package explorer

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// PlanToActions converts an AI exploration plan into standard crawl actions
// that the crawler processes independently (each on its own page/tab).
//
// - Click actions with href → ActionTypeLoadURL (navigate directly)
// - Click actions without href → ActionTypeLeftClick (click on origin page)
// - Fill+submit → ActionTypeFillForm (fill and submit on origin page)
// - Expand menu → ActionTypeLeftClick (click on origin page)
func PlanToActions(plan *ExplorationPlan, snapshot *PageSnapshot, originPageID string, currentDepth int) []*types.Action {
	if plan == nil {
		return nil
	}

	// Sort by priority
	actions := make([]PlannedAction, len(plan.Actions))
	copy(actions, plan.Actions)
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority > actions[j].Priority
	})

	var result []*types.Action

	for _, pa := range actions {
		if pa.Priority == 0 {
			continue // skip
		}

		switch pa.Action {
		case "click", "expand_menu":
			crawlAction := convertClickAction(pa, snapshot, originPageID, currentDepth)
			if crawlAction != nil {
				result = append(result, crawlAction)
				gologger.Info().Msgf("[explorer] → queued [P%d] %s: %s — %s",
					pa.Priority, pa.Action, describeRef(snapshot, pa.Ref), pa.Reason)
			}

		case "fill_and_submit":
			// Form submissions need to happen on the same page — convert to FillForm action
			crawlAction := convertFormAction(pa, snapshot, plan.FormFillValues, originPageID, currentDepth)
			if crawlAction != nil {
				result = append(result, crawlAction)
				gologger.Info().Msgf("[explorer] → queued [P%d] fill_form: %s — %s",
					pa.Priority, describeRef(snapshot, pa.FormRef), pa.Reason)
			}

		case "sample_pagination":
			gologger.Info().Msgf("[explorer] → pagination recommendation: sample pages %v — %s", pa.Pages, pa.Reason)
			// Pagination is handled naturally by the template dedup system

		case "skip":
			// Nothing

		default:
			gologger.Debug().Msgf("[explorer] → unknown action type: %s", pa.Action)
		}
	}

	return result
}

// convertClickAction converts a planned click into a crawl action.
// If the element has an href, it becomes a LoadURL action (independent navigation).
// If not, it becomes a LeftClick action on the origin page.
func convertClickAction(pa PlannedAction, snapshot *PageSnapshot, originPageID string, depth int) *types.Action {
	if snapshot == nil || pa.Ref == 0 {
		return nil
	}

	// Find the ref in the snapshot
	var ref *PageRef
	for i, r := range snapshot.Refs {
		if r.Ref == pa.Ref {
			ref = &snapshot.Refs[i]
			break
		}
	}
	if ref == nil {
		return nil
	}

	// If it has an href, navigate directly — this is independent and works across tabs
	if ref.Href != "" && !strings.HasPrefix(ref.Href, "javascript") && !strings.HasPrefix(ref.Href, "#") {
		href := ref.Href
		// Resolve relative URLs
		if !strings.HasPrefix(href, "http") && snapshot != nil && snapshot.URL != "" {
			if base, err := url.Parse(snapshot.URL); err == nil {
				if resolved, err := base.Parse(href); err == nil {
					href = resolved.String()
				}
			}
		}
		return &types.Action{
			Type:     types.ActionTypeLoadURL,
			Input:    href,
			OriginID: originPageID,
			Depth:    depth + 1,
		}
	}

	// No href — this is a button/menu click that needs the origin page context.
	// Create a LeftClick action with the element info from the snapshot.
	element := &types.HTMLElement{
		TagName:     ref.Tag,
		TextContent: ref.Name,
		Type:        ref.Type,
		Attributes:  make(map[string]string),
	}
	if ref.Role != "" {
		element.Attributes["role"] = ref.Role
	}

	// Build a best-effort CSS selector from available info
	selector := buildSelectorFromRef(ref)
	element.CSSSelector = selector
	element.XPath = "" // will need CSS-based lookup

	return &types.Action{
		Type:     types.ActionTypeLeftClick,
		Element:  element,
		OriginID: originPageID,
		Depth:    depth + 1,
	}
}

// convertFormAction converts a planned form fill into a FillForm crawl action.
func convertFormAction(pa PlannedAction, snapshot *PageSnapshot, globalFillValues map[string]string, originPageID string, depth int) *types.Action {
	if snapshot == nil {
		return nil
	}

	// Find the form in the snapshot
	var formRef *FormRef
	for i, f := range snapshot.Forms {
		if f.Ref == pa.FormRef {
			formRef = &snapshot.Forms[i]
			break
		}
	}
	if formRef == nil {
		return nil
	}

	// Build HTMLForm from the snapshot
	form := &types.HTMLForm{
		TagName: "FORM",
		Action:  formRef.Action,
		Method:  formRef.Method,
	}

	// Build form elements with fill values
	for _, field := range formRef.Fields {
		el := &types.HTMLElement{
			TagName:    field.Tag,
			Type:       field.Type,
			Attributes: map[string]string{},
		}
		if field.Name != "" {
			el.Attributes["name"] = field.Name
			el.ID = field.Name
		}
		if field.Placeholder != "" {
			el.Attributes["placeholder"] = field.Placeholder
		}

		// Check if the plan provided a fill value for this field
		refStr := fmt.Sprintf("%d", field.Ref)
		if fill, ok := pa.Fields[refStr]; ok {
			el.Value = fill.Value
		} else if field.Name != "" {
			// Smart fill from global values or patterns
			if gv, ok := globalFillValues[field.Name]; ok {
				el.Value = gv
			}
		}

		form.Elements = append(form.Elements, el)
	}

	return &types.Action{
		Type:     types.ActionTypeFillForm,
		Form:     form,
		OriginID: originPageID,
		Depth:    depth + 1,
	}
}

// buildSelectorFromRef creates a best-effort CSS selector from a PageRef.
func buildSelectorFromRef(ref *PageRef) string {
	tag := strings.ToLower(ref.Tag)

	// Try aria-label first (most reliable for buttons)
	if ref.Name != "" && (ref.Tag == "BUTTON" || ref.Role == "button") {
		// Escape single quotes in the name
		escaped := strings.ReplaceAll(ref.Name, "'", "\\'")
		if len(escaped) < 50 {
			return fmt.Sprintf("%s[aria-label='%s']", tag, escaped)
		}
	}

	// Try type attribute for inputs
	if ref.Type != "" && (ref.Tag == "INPUT" || ref.Tag == "TEXTAREA") {
		if ref.Name != "" {
			return fmt.Sprintf("%s[name='%s']", tag, ref.Name)
		}
		return fmt.Sprintf("%s[type='%s']", tag, ref.Type)
	}

	// Fallback to tag
	return tag
}

// describeRef returns a human-readable description of a ref for logging.
func describeRef(snapshot *PageSnapshot, ref int) string {
	if snapshot == nil || ref == 0 {
		return fmt.Sprintf("@%d", ref)
	}
	for _, r := range snapshot.Refs {
		if r.Ref == ref {
			name := r.Name
			if len(name) > 40 {
				name = name[:37] + "..."
			}
			extra := ""
			if r.Href != "" {
				extra = fmt.Sprintf(" → %s", r.Href)
			}
			return fmt.Sprintf("@%d [%s] \"%s\"%s", r.Ref, strings.ToLower(r.Tag), name, extra)
		}
	}
	for _, f := range snapshot.Forms {
		if f.Ref == ref {
			return fmt.Sprintf("@%d [form] action=%s", f.Ref, f.Action)
		}
	}
	return fmt.Sprintf("@%d", ref)
}
