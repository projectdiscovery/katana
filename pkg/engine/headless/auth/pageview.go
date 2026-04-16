package auth

import (
	"github.com/go-rod/rod"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	headlesstypes "github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// PageView is a multi-layer representation of a web page for AI agents.
// Each layer adds detail at increasing token cost. The accessibility tree (Layer 2)
// is the primary input for AI — it's compact, semantic, and what screen readers see.
type PageView struct {
	// Layer 1: Metadata (~100 tokens)
	URL   string `json:"url"`
	Title string `json:"title"`

	// Layer 2: Accessibility tree (~500-2000 tokens) — primary AI input
	AccessibilityTree string `json:"accessibility_tree,omitempty"`

	// Layer 3: Structured form data (~200-1000 tokens per form)
	Forms []FormView `json:"forms,omitempty"`

	// Layer 4: Interactive elements (~500-3000 tokens)
	InteractiveElements []ElementView `json:"interactive_elements,omitempty"`

	// Layer 5: Visible text excerpt (~200 tokens)
	VisibleText string `json:"visible_text,omitempty"`
}

// FormView is a compact representation of an HTML form.
type FormView struct {
	Selector string     `json:"selector"`
	Action   string     `json:"action,omitempty"`
	Method   string     `json:"method,omitempty"`
	Fields   []FieldView `json:"fields"`
}

// FieldView represents a single form field.
type FieldView struct {
	Selector    string `json:"selector"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Label       string `json:"label,omitempty"`
	Required    bool   `json:"required,omitempty"`
	AriaLabel   string `json:"aria_label,omitempty"`
	Value       string `json:"value,omitempty"`
}

// ElementView represents an interactive element on the page.
type ElementView struct {
	Selector string `json:"selector"`
	TagName  string `json:"tag"`
	Text     string `json:"text,omitempty"`
	Type     string `json:"type,omitempty"`
	Role     string `json:"role,omitempty"`
	Href     string `json:"href,omitempty"`
}

// BuildPageView constructs a multi-layer PageView from a BrowserPage.
// It reuses the existing JS injection functions for form and element discovery,
// and adds the accessibility tree via CDP.
func BuildPageView(page *browser.BrowserPage) (*PageView, error) {
	view := &PageView{}

	// Layer 1: URL and title
	info, err := page.Info()
	if err == nil && info != nil {
		view.URL = info.URL
		view.Title = info.Title
	}

	// Layer 2: Accessibility tree
	tree, err := page.GetAccessibilityTree(5)
	if err == nil {
		view.AccessibilityTree = tree
	}

	// Layer 3: Forms
	forms, err := page.GetAllForms()
	if err == nil {
		view.Forms = convertForms(forms, page.Page)
	}

	// Layer 4: Interactive elements
	elements, err := page.GetAllElements("a, button, [role='button'], input[type='submit'], [onclick]")
	if err == nil {
		view.InteractiveElements = convertElements(elements)
	}

	// Layer 5: Visible text
	textResult, err := page.Eval(`() => {
		try {
			const text = document.body ? document.body.innerText : '';
			return text.substring(0, 2000);
		} catch(e) {
			return '';
		}
	}`)
	if err == nil {
		view.VisibleText = textResult.Value.Str()
	}

	return view, nil
}

// convertForms converts headless HTMLForm types to compact FormView types,
// enriching with label associations via additional JS evaluation.
func convertForms(forms []*headlesstypes.HTMLForm, page *rod.Page) []FormView {
	views := make([]FormView, 0, len(forms))
	for _, f := range forms {
		fv := FormView{
			Selector: f.CSSSelector,
			Action:   f.Action,
			Method:   f.Method,
		}
		for _, el := range f.Elements {
			field := FieldView{
				Selector: el.CSSSelector,
				Name:     el.Attributes["name"],
				Type:     el.Type,
			}
			if el.Attributes != nil {
				field.Placeholder = el.Attributes["placeholder"]
				field.AriaLabel = el.Attributes["aria-label"]
				if _, ok := el.Attributes["required"]; ok {
					field.Required = true
				}
			}

			// Only expose value for hidden fields (CSRF tokens, not user data)
			if el.Type == "hidden" {
				field.Value = el.Value
			}

			// Try to get the label for this field
			if el.ID != "" && page != nil {
				label := getLabelText(page, el.ID)
				if label != "" {
					field.Label = label
				}
			}

			fv.Fields = append(fv.Fields, field)
		}
		views = append(views, fv)
	}
	return views
}

// getLabelText finds the <label> text associated with an input by its ID.
func getLabelText(page *rod.Page, inputID string) string {
	result, err := page.Eval(`(inputId) => {
		try {
			const label = document.querySelector('label[for="' + inputId + '"]');
			if (label) return label.textContent.trim();
			const input = document.getElementById(inputId);
			if (input) {
				const parentLabel = input.closest('label');
				if (parentLabel) return parentLabel.textContent.trim();
			}
			return '';
		} catch(e) {
			return '';
		}
	}`, inputID)
	if err != nil {
		return ""
	}
	return result.Value.Str()
}

// convertElements converts headless HTMLElement types to compact ElementView types.
func convertElements(elements []*headlesstypes.HTMLElement) []ElementView {
	views := make([]ElementView, 0, len(elements))
	for _, el := range elements {
		ev := ElementView{
			Selector: el.CSSSelector,
			TagName:  el.TagName,
			Type:     el.Type,
		}

		// Truncate text to 100 chars
		text := el.TextContent
		if len(text) > 100 {
			text = text[:97] + "..."
		}
		ev.Text = text

		if el.Attributes != nil {
			ev.Href = el.Attributes["href"]
			ev.Role = el.Attributes["role"]
		}

		views = append(views, ev)
	}
	return views
}
