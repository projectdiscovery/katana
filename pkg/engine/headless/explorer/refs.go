package explorer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
)

// PageRef is an interactive element on the page with a numeric reference ID.
// The AI planner uses ref IDs instead of CSS selectors to avoid selector compatibility issues.
type PageRef struct {
	Ref       int    `json:"ref"`
	Tag       string `json:"tag"`
	Role      string `json:"role,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Href      string `json:"href,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
	Clickable bool   `json:"clickable,omitempty"` // cursor-interactive (cursor:pointer/onclick without ARIA role)
	Hints     string `json:"hints,omitempty"`     // e.g. "cursor:pointer, onclick"
	Ordinal   int    `json:"ordinal,omitempty"`   // 1-based index when multiple elements share same tag+name
	TotalDups int    `json:"total_dups,omitempty"` // total count of duplicates (0 if unique)
}

// FormRef is a form on the page with its fields as refs.
type FormRef struct {
	Ref    int        `json:"ref"`
	Action string     `json:"action,omitempty"`
	Method string     `json:"method,omitempty"`
	Fields []FieldRef `json:"fields"`
}

// FieldRef is a form field with a ref ID.
type FieldRef struct {
	Ref         int    `json:"ref"`
	Tag         string `json:"tag"`
	Name        string `json:"name,omitempty"`
	Type        string `json:"type,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Label       string `json:"label,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PageSnapshot holds all interactive elements on a page, indexed by ref.
type PageSnapshot struct {
	URL      string    `json:"url"`
	Title    string    `json:"title"`
	Refs     []PageRef `json:"elements"`
	Forms    []FormRef `json:"forms"`
	Text     string    `json:"visible_text,omitempty"`
	elements map[int]*rod.Element // internal: ref → live DOM element
}

// BuildPageSnapshot scans the page and assigns numeric refs to all interactive elements.
func BuildPageSnapshot(page *rod.Page) *PageSnapshot {
	snap := &PageSnapshot{
		elements: make(map[int]*rod.Element),
	}

	if info, err := page.Info(); err == nil {
		snap.URL = info.URL
		snap.Title = info.Title
	}

	// Get visible text
	textResult, _ := page.Eval(`() => {
		try { return (document.body ? document.body.innerText : '').substring(0, 500); }
		catch(e) { return ''; }
	}`)
	if textResult != nil {
		snap.Text = textResult.Value.Str()
	}

	ref := 1

	// Collect all interactive elements
	selectors := "a, button, [role='button'], [role='link'], [role='tab'], [role='menuitem'], " +
		"input:not([type='hidden']), textarea, select, " +
		"[aria-expanded], [aria-haspopup], details > summary"

	elements, err := page.Timeout(5 * time.Second).Elements(selectors)
	if err != nil {
		gologger.Debug().Msgf("[refs] Failed to get elements: %s", err)
		return snap
	}

	for _, el := range elements {
		// Skip invisible elements
		visible, _ := el.Visible()
		if !visible {
			continue
		}

		// Get element info
		tag := evalStr(el, `() => this.tagName`)
		role := evalStr(el, `() => this.getAttribute('role') || ''`)
		name := evalStr(el, `() => {
			return this.textContent?.trim()?.substring(0, 60) ||
				this.getAttribute('aria-label') ||
				this.getAttribute('title') ||
				this.getAttribute('placeholder') ||
				this.getAttribute('name') || '';
		}`)
		elemType := evalStr(el, `() => this.type || ''`)
		href := evalStr(el, `() => this.getAttribute('href') || ''`)
		disabled := evalStr(el, `() => this.disabled ? 'true' : ''`) == "true"

		if name == "" && tag == "INPUT" {
			name = evalStr(el, `() => this.getAttribute('name') || this.getAttribute('id') || ''`)
		}

		pageRef := PageRef{
			Ref:      ref,
			Tag:      tag,
			Role:     role,
			Name:     name,
			Type:     elemType,
			Href:     href,
			Disabled: disabled,
		}

		snap.Refs = append(snap.Refs, pageRef)
		snap.elements[ref] = el
		ref++
	}

	// Detect cursor-interactive elements (clickable divs/spans without ARIA roles).
	// These are common in modern SPAs that use styled divs as buttons.
	cursorElements := findCursorInteractiveElements(page)
	for _, ci := range cursorElements {
		pageRef := PageRef{
			Ref:       ref,
			Tag:       ci.tag,
			Name:      ci.name,
			Clickable: true,
			Hints:     ci.hints,
		}
		snap.Refs = append(snap.Refs, pageRef)
		snap.elements[ref] = ci.element
		ref++
	}

	// Scan iframe content for interactive elements
	iframes, _ := page.Timeout(3 * time.Second).Elements("iframe")
	for _, iframe := range iframes {
		framePage, err := iframe.Frame()
		if err != nil {
			continue
		}
		frameEls, err := framePage.Timeout(3 * time.Second).Elements(selectors)
		if err != nil {
			continue
		}
		for _, el := range frameEls {
			visible, _ := el.Visible()
			if !visible {
				continue
			}
			tag := evalStr(el, `() => this.tagName`)
			role := evalStr(el, `() => this.getAttribute('role') || ''`)
			name := evalStr(el, `() => {
				return this.textContent?.trim()?.substring(0, 60) ||
					this.getAttribute('aria-label') ||
					this.getAttribute('title') ||
					this.getAttribute('placeholder') ||
					this.getAttribute('name') || '';
			}`)
			elemType := evalStr(el, `() => this.type || ''`)
			href := evalStr(el, `() => this.getAttribute('href') || ''`)
			disabled := evalStr(el, `() => this.disabled ? 'true' : ''`) == "true"

			pageRef := PageRef{
				Ref:      ref,
				Tag:      tag,
				Role:     role,
				Name:     name,
				Type:     elemType,
				Href:     href,
				Disabled: disabled,
			}
			snap.Refs = append(snap.Refs, pageRef)
			snap.elements[ref] = el
			ref++
		}
	}

	// Disambiguate duplicate elements (same tag+name)
	disambiguateRefs(snap.Refs)

	// Collect forms
	formEls, _ := page.Timeout(3 * time.Second).Elements("form")
	for _, formEl := range formEls {
		formRef := FormRef{
			Ref:    ref,
			Action: evalStr(formEl, `() => this.action || ''`),
			Method: evalStr(formEl, `() => this.method || 'GET'`),
		}
		snap.elements[ref] = formEl
		ref++

		// Get form fields
		fields, _ := formEl.Elements("input:not([type='hidden']), textarea, select, button")
		for _, field := range fields {
			fieldRef := FieldRef{
				Ref:         ref,
				Tag:         evalStr(field, `() => this.tagName`),
				Name:        evalStr(field, `() => this.name || this.id || ''`),
				Type:        evalStr(field, `() => this.type || ''`),
				Placeholder: evalStr(field, `() => this.placeholder || ''`),
				Required:    evalStr(field, `() => this.required ? 'true' : ''`) == "true",
			}
			formRef.Fields = append(formRef.Fields, fieldRef)
			snap.elements[ref] = field
			ref++
		}

		snap.Forms = append(snap.Forms, formRef)
	}

	return snap
}

// ClickRef clicks an element by its ref ID using JavaScript dispatch
// (bypasses pointer-events:none issues).
func (s *PageSnapshot) ClickRef(ref int) error {
	el, ok := s.elements[ref]
	if !ok {
		return fmt.Errorf("ref @%d not found", ref)
	}
	// Use JS click to bypass pointer-events:none
	_, err := el.Eval(`() => this.click()`)
	if err != nil {
		// Fallback to native click
		return el.Click(proto.InputMouseButtonLeft, 1)
	}
	return nil
}

// TypeRef types text into a form field by its ref ID.
func (s *PageSnapshot) TypeRef(ref int, text string) error {
	el, ok := s.elements[ref]
	if !ok {
		return fmt.Errorf("ref @%d not found", ref)
	}
	// Focus the element first
	_, _ = el.Eval(`() => { this.focus(); this.value = ''; }`)
	return el.Input(text)
}

// SelectRef selects an option by text in a select element.
func (s *PageSnapshot) SelectRef(ref int, value string) error {
	el, ok := s.elements[ref]
	if !ok {
		return fmt.Errorf("ref @%d not found", ref)
	}
	return el.Select([]string{value}, true, rod.SelectorTypeText)
}

// GetElement returns the raw rod.Element for a ref.
func (s *PageSnapshot) GetElement(ref int) (*rod.Element, bool) {
	el, ok := s.elements[ref]
	return el, ok
}

// ToJSON returns the snapshot as pretty JSON for the AI planner.
func (s *PageSnapshot) ToJSON() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}

// FormatCompact returns a compact text representation for the AI planner.
func (s *PageSnapshot) FormatCompact() string {
	var b strings.Builder
	fmt.Fprintf(&b, "URL: %s\nTitle: %s\n\n", s.URL, s.Title)

	if len(s.Forms) > 0 {
		b.WriteString("FORMS:\n")
		for _, f := range s.Forms {
			fmt.Fprintf(&b, "  @%d form action=%s method=%s\n", f.Ref, f.Action, f.Method)
			for _, field := range f.Fields {
				label := field.Name
				if label == "" {
					label = field.Placeholder
				}
				req := ""
				if field.Required {
					req = " *required"
				}
				fmt.Fprintf(&b, "    @%d %s[%s] \"%s\"%s\n", field.Ref, strings.ToLower(field.Tag), field.Type, label, req)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("INTERACTIVE ELEMENTS:\n")
	for _, r := range s.Refs {
		tag := strings.ToLower(r.Tag)
		name := r.Name
		if len(name) > 50 {
			name = name[:47] + "..."
		}
		extra := ""
		if r.Href != "" && !strings.HasPrefix(r.Href, "javascript") {
			extra = fmt.Sprintf(" href=%s", r.Href)
		}
		if r.Disabled {
			extra += " [disabled]"
		}
		if r.Role != "" && r.Role != tag {
			extra += fmt.Sprintf(" role=%s", r.Role)
		}
		if r.Clickable {
			extra += fmt.Sprintf(" clickable [%s]", r.Hints)
		}
		ordinal := ""
		if r.Ordinal > 0 {
			ordinal = fmt.Sprintf(" (%s of %d)", ordinalStr(r.Ordinal), r.TotalDups)
		}
		fmt.Fprintf(&b, "  @%d [%s] \"%s\"%s%s\n", r.Ref, tag, name, extra, ordinal)
	}

	if s.Text != "" {
		fmt.Fprintf(&b, "\nVISIBLE TEXT:\n%s\n", s.Text)
	}

	return b.String()
}

// disambiguateRefs adds ordinal info to refs that share the same tag+name,
// so the AI planner can distinguish between e.g. three "Delete" buttons.
func disambiguateRefs(refs []PageRef) {
	counts := make(map[string]int)
	for _, r := range refs {
		if r.Name == "" {
			continue
		}
		key := r.Tag + ":" + r.Name
		counts[key]++
	}

	idx := make(map[string]int)
	for i := range refs {
		if refs[i].Name == "" {
			continue
		}
		key := refs[i].Tag + ":" + refs[i].Name
		total := counts[key]
		if total <= 1 {
			continue
		}
		n := idx[key]
		idx[key]++
		refs[i].Ordinal = n + 1
		refs[i].TotalDups = total
	}
}

type cursorInteractiveElement struct {
	tag     string
	name    string
	hints   string
	element *rod.Element
}

// findCursorInteractiveElements discovers elements with cursor:pointer or onclick
// that aren't standard interactive elements (a, button, input, select, textarea)
// and don't have ARIA roles. These are common in SPAs using styled divs as buttons.
func findCursorInteractiveElements(page *rod.Page) []cursorInteractiveElement {
	// Run JS to find all cursor-interactive elements, returning their indices
	// so we can resolve them back to rod.Element via querySelectorAll.
	result, err := page.Timeout(5 * time.Second).Eval(`() => {
		var results = [];
		if (!document.body) return results;

		var interactiveTags = {'a':1,'button':1,'input':1,'select':1,'textarea':1,'details':1,'summary':1};
		var interactiveRoles = {
			'button':1,'link':1,'textbox':1,'checkbox':1,'radio':1,'combobox':1,'listbox':1,
			'menuitem':1,'menuitemcheckbox':1,'menuitemradio':1,'option':1,'searchbox':1,
			'slider':1,'spinbutton':1,'switch':1,'tab':1,'treeitem':1
		};

		var allElements = document.body.querySelectorAll('*');
		for (var i = 0; i < allElements.length; i++) {
			var el = allElements[i];

			if (el.closest && el.closest('[hidden], [aria-hidden="true"]')) continue;

			var tagName = el.tagName.toLowerCase();
			if (interactiveTags[tagName]) continue;

			var role = el.getAttribute('role');
			if (role && interactiveRoles[role.toLowerCase()]) continue;

			// Also skip elements matched by aria-expanded/aria-haspopup (already scanned)
			if (el.hasAttribute('aria-expanded') || el.hasAttribute('aria-haspopup')) continue;

			var computedStyle = getComputedStyle(el);
			var hasCursorPointer = computedStyle.cursor === 'pointer';
			var hasOnClick = el.hasAttribute('onclick') || el.onclick !== null;
			var tabIndex = el.getAttribute('tabindex');
			var hasTabIndex = tabIndex !== null && tabIndex !== '-1';
			var ce = el.getAttribute('contenteditable');
			var isEditable = ce === '' || ce === 'true';

			if (!hasCursorPointer && !hasOnClick && !hasTabIndex && !isEditable) continue;

			// Skip inherited cursor:pointer from parent (not a direct interactive element)
			if (hasCursorPointer && !hasOnClick && !hasTabIndex && !isEditable) {
				var parent = el.parentElement;
				if (parent && getComputedStyle(parent).cursor === 'pointer') continue;
			}

			var text = (el.textContent || '').trim().substring(0, 60);
			if (!text) continue; // skip nameless elements

			var rect = el.getBoundingClientRect();
			if (rect.width === 0 || rect.height === 0) continue;

			var hints = [];
			if (hasCursorPointer) hints.push('cursor:pointer');
			if (hasOnClick) hints.push('onclick');
			if (hasTabIndex) hints.push('tabindex');
			if (isEditable) hints.push('contenteditable');

			// Tag element for resolution
			el.setAttribute('data-__katana-ci', String(results.length));
			results.push({
				tag: tagName.toUpperCase(),
				text: text,
				hints: hints.join(', ')
			});
		}
		return results;
	}`)
	if err != nil {
		return nil
	}

	var jsResults []struct {
		Tag   string `json:"tag"`
		Text  string `json:"text"`
		Hints string `json:"hints"`
	}
	if err := json.Unmarshal([]byte(result.Value.JSON("", "")), &jsResults); err != nil {
		return nil
	}

	if len(jsResults) == 0 {
		return nil
	}

	// Resolve tagged elements back to rod.Element
	taggedEls, err := page.Timeout(3 * time.Second).Elements("[data-__katana-ci]")
	if err != nil {
		return nil
	}

	// Build index map from data attribute to rod.Element
	elMap := make(map[string]*rod.Element)
	for _, el := range taggedEls {
		idx := evalStr(el, `() => this.getAttribute('data-__katana-ci') || ''`)
		if idx != "" {
			elMap[idx] = el
		}
	}

	// Clean up data attributes
	_, _ = page.Eval(`() => {
		var els = document.querySelectorAll('[data-__katana-ci]');
		for (var i = 0; i < els.length; i++) els[i].removeAttribute('data-__katana-ci');
	}`)

	var out []cursorInteractiveElement
	for i, r := range jsResults {
		el, ok := elMap[fmt.Sprintf("%d", i)]
		if !ok {
			continue
		}
		out = append(out, cursorInteractiveElement{
			tag:     r.Tag,
			name:    r.Text,
			hints:   r.Hints,
			element: el,
		})
	}
	return out
}

func ordinalStr(n int) string {
	switch {
	case n%100 == 11 || n%100 == 12 || n%100 == 13:
		return fmt.Sprintf("%dth", n)
	case n%10 == 1:
		return fmt.Sprintf("%dst", n)
	case n%10 == 2:
		return fmt.Sprintf("%dnd", n)
	case n%10 == 3:
		return fmt.Sprintf("%drd", n)
	default:
		return fmt.Sprintf("%dth", n)
	}
}

func evalStr(el *rod.Element, js string) string {
	result, err := el.Eval(js)
	if err != nil {
		return ""
	}
	return result.Value.Str()
}
