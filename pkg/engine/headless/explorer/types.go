package explorer

// ExplorationPlan is the AI planner's output for a single page.
// It describes what actions to take, in what order, and why.
type ExplorationPlan struct {
	PageSummary    string            `json:"page_summary"`
	PageType       string            `json:"page_type"`
	Actions        []PlannedAction   `json:"actions"`
	MultiStepFlows []FlowHint        `json:"multi_step_flows,omitempty"`
	FormFillValues map[string]string `json:"form_fill_values,omitempty"`
}

// PlannedAction is a single action in the exploration plan.
type PlannedAction struct {
	// Action type: expand_menu, fill_and_submit, click, sample_pagination, skip
	Action string `json:"action"`

	// Ref is the numeric reference ID of the target element (from page snapshot).
	Ref int `json:"ref,omitempty"`

	// Selector is a CSS selector fallback (used when ref is unavailable).
	Selector string `json:"selector,omitempty"`

	// FormRef is the ref of the target form (for fill_and_submit).
	FormRef int `json:"form_ref,omitempty"`

	// FormSelector is a CSS selector fallback for the form.
	FormSelector string `json:"form_selector,omitempty"`

	// Fields maps field ref IDs (as strings) to fill values (for fill_and_submit).
	Fields map[string]FieldFill `json:"fields,omitempty"`

	// Pages is the list of page numbers to visit (for sample_pagination).
	Pages []int `json:"pages,omitempty"`

	// Priority from 0 (skip) to 10 (highest).
	Priority int `json:"priority"`

	// Reason explains why this action is valuable.
	Reason string `json:"reason"`

	// Expected describes what we expect to happen after this action.
	Expected string `json:"expected,omitempty"`

	// FollowFlow indicates the executor should follow multi-step flows after clicking.
	FollowFlow bool `json:"follow_flow,omitempty"`
}

// FieldFill describes how to fill a single form field.
type FieldFill struct {
	Value string `json:"value"`
	Type  string `json:"type,omitempty"` // text, select, checkbox, file
}

// FlowHint describes a multi-step flow the executor should follow.
type FlowHint struct {
	Trigger        string     `json:"trigger"`
	FlowType       string     `json:"type"`            // wizard, confirmation, modal_form
	EstimatedSteps int        `json:"estimated_steps"`
	Strategy       string     `json:"strategy"`
	StepHints      []StepHint `json:"step_hints,omitempty"`
}

// StepHint provides fill values for a specific step in a multi-step flow.
type StepHint struct {
	Step        int               `json:"step"`
	Description string            `json:"description"`
	Fill        map[string]string `json:"fill,omitempty"`
}

// GraphContext is passed to the planner so it knows the crawl state.
type GraphContext struct {
	CurrentURL       string   `json:"current_url"`
	PageTitle        string   `json:"page_title"`
	PagesVisited     int      `json:"pages_visited"`
	TemplatesFound   int      `json:"templates_found"`
	BudgetRemaining  int      `json:"budget_remaining"`
	VisitedPaths     []string `json:"visited_paths"`
	TemplatePatterns []string `json:"template_patterns,omitempty"`
}
