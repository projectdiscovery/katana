package explorer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/agent"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
)

const plannerSystemPrompt = `You are a web application exploration planner for a security crawler.

You see a page's interactive elements (with @ref numbers), an accessibility tree showing semantic hierarchy, and the crawl context (pages visited, templates discovered, budget remaining).

The accessibility tree shows the page structure as the browser understands it — roles like button, link, menuitem, tab reveal interactive intent regardless of HTML tag. Use it to understand hierarchy (button inside nav vs modal) and state (aria-expanded, disabled).

Produce a JSON exploration plan — a prioritized list of actions that maximally discovers the application's functionality and attack surface.

PRIORITIES (highest first):
10 - Auth-gated content just discovered
 9 - Collapsed menus/dropdowns hiding navigation (aria-expanded=false)
 8 - Data entry forms (create/edit/upload) — reveal business logic
 8 - Action buttons ("Create", "Add", "Import", "New") — trigger new flows
 7 - Search/query forms — discover content and test injection surface
 6 - Tabs/accordions showing different content views
 6 - Filter/sort forms — change what data is displayed
 5 - Modal triggers — may contain forms or actions
 4 - Navigation links to NEW path patterns not yet visited
 3 - Links matching already-discovered templates (sample only 1-2)
 2 - Pagination (sample first, middle, last page)
 1 - Context menus, row actions ("...", kebab menus)
 0 - SKIP: cosmetic buttons, logout, footer, marketing, static assets

FORM FILLING:
- Search forms: use descriptive terms like "test", "admin", "example.com"
- Data entry: use realistic values based on field names (email→test@example.com, domain→example.com, ip→192.168.1.1, target→https://scanme.nmap.org)
- Filters: create separate actions to try each dropdown option
- File uploads: note as high-priority but set value to "test.txt"
- Login/register forms: SKIP — handled by auth module separately

MULTI-STEP FLOWS:
- If a button likely leads to a wizard (e.g., "Create Scan", "New Project"), set follow_flow=true
- Provide step_hints with realistic fill values for each anticipated step
- The executor will follow the flow to completion using your hints

SKIP THESE:
- Logout/signout/delete/remove buttons (destructive)
- Copy/share/print/download buttons (cosmetic, no new pages)
- External links (different domain)
- Links to pages matching templates already sampled 3+ times
- Static assets (CSS, JS, images, fonts)
- Footer/legal/privacy links

ELEMENT REFERENCES:
- Every element on the page has a numeric ref: @1, @2, @3, etc.
- Use "ref" field in your actions to target elements: {"action": "click", "ref": 5}
- For forms: use "form_ref" for the form and field refs in "fields"
- Example: {"action": "fill_and_submit", "form_ref": 10, "fields": {"12": {"value": "test", "type": "text"}, "13": {"value": "example.com", "type": "text"}}}

ACTION TYPES — only use these:
- "click" — click a button/link by ref
- "fill_and_submit" — fill form fields by field refs and submit
- "expand_menu" — click a collapsed menu by ref
- "sample_pagination" — note which pages to sample
- "skip" — explicitly skip an element

Respond ONLY with valid JSON matching this schema:
{
  "page_summary": "string describing what this page is",
  "page_type": "string categorizing the page",
  "actions": [{"action": "click", "ref": N, "priority": 8, "reason": "string"}, ...],
  "multi_step_flows": [{"trigger_ref": N, "type": "wizard", "estimated_steps": 3, "step_hints": [...]}],
  "form_fill_values": {"field_name": "value"}
}`

// Planner uses an LLM to analyze each page and produce an exploration plan.
type Planner struct {
	apiKey       string
	model        string
	logger       *slog.Logger
	cache        map[string]*ExplorationPlan
	lastSnapshot *PageSnapshot // most recent page snapshot (for executor)
}

// LastSnapshot returns the most recent page snapshot (used by executor for ref resolution).
func (p *Planner) LastSnapshot() *PageSnapshot {
	return p.lastSnapshot
}

// NewPlanner creates an AI exploration planner.
func NewPlanner(logger *slog.Logger) *Planner {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	return &Planner{
		apiKey: apiKey,
		model:  agent.ModelHaiku4_5,
		logger: logger,
		cache:  make(map[string]*ExplorationPlan),
	}
}

// Plan produces an exploration plan for the current page.
// Returns a cached plan if available for this URL pattern.
func (p *Planner) Plan(ctx context.Context, page *browser.BrowserPage, graphCtx *GraphContext) (*ExplorationPlan, error) {
	// Check cache first
	cacheKey := urlPattern(graphCtx.CurrentURL)
	if cached, ok := p.cache[cacheKey]; ok {
		gologger.Debug().Msgf("[explorer] Using cached plan for %s", cacheKey)
		return cached, nil
	}

	if p.apiKey == "" {
		return nil, fmt.Errorf("no API key available for AI planner")
	}

	// Build page snapshot with numeric refs for all interactive elements
	snapshot := BuildPageSnapshot(page.Page)

	// Store snapshot so executor can use it
	p.lastSnapshot = snapshot

	// Fetch accessibility tree for structural context (hierarchy of semantic roles)
	axTree, axErr := page.GetAccessibilityTree(3)
	axSection := ""
	if axErr == nil && axTree != "" {
		axSection = fmt.Sprintf("\n\n## Accessibility Tree (semantic structure)\n```\n%s\n```", axTree)
	}

	ctxJSON, _ := json.MarshalIndent(graphCtx, "", "  ")

	userMsg := fmt.Sprintf("Analyze this page and produce an exploration plan.\n\nEach element has a ref number (@N). Use these refs in your plan.\n\n## Page Snapshot\n```\n%s\n```%s\n\n## Crawl Context\n```json\n%s\n```", snapshot.FormatCompact(), axSection, string(ctxJSON))

	// Create the planner agent — it only has one tool: submit_plan
	client := agent.NewAnthropicAgentClient(p.apiKey, agent.WithModel(p.model))

	resultTool := agent.NewResultTool(
		"submit_plan",
		"Submit the exploration plan for this page as JSON",
		"plan",
		"The complete exploration plan as a JSON object",
	)

	plannerAgent := agent.New(client,
		agent.WithSystemPrompt(plannerSystemPrompt),
		agent.WithTools(resultTool),
		agent.WithMaxTurns(2),
		agent.WithTokenBudget(30000),
		agent.WithMaxTokensPerCall(8192),
		agent.WithResultTool("submit_plan", "plan"),
		agent.WithOnTurn(func(ev agent.TurnEvent) {
			gologger.Debug().Msgf("[explorer] planner turn=%d stop=%s tokens=%d/%d",
				ev.Turn, ev.Response.StopReason, ev.TotalUsage.InputTokens, ev.TotalUsage.OutputTokens)
		}),
		agent.WithAgentLogger(p.logger),
	)

	gologger.Info().Msgf("[explorer] AI planning page: %s", graphCtx.CurrentURL)

	result, err := plannerAgent.Run(ctx, userMsg)
	if err != nil {
		return nil, fmt.Errorf("planner agent: %w", err)
	}

	// Parse the plan from the result
	plan := &ExplorationPlan{}
	if err := json.Unmarshal([]byte(result.Response), plan); err != nil {
		// Try to extract from the raw response text
		gologger.Debug().Msgf("[explorer] Failed to parse plan JSON: %s", truncate(result.Response, 200))
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	gologger.Info().Msgf("[explorer] Plan created: %s — %d actions, %d flows",
		plan.PageSummary, len(plan.Actions), len(plan.MultiStepFlows))

	// Log the plan actions
	for _, a := range plan.Actions {
		if a.Priority > 0 {
			gologger.Info().Msgf("[explorer]   [P%d] %s %s — %s", a.Priority, a.Action, truncate(a.Selector, 40), a.Reason)
		}
	}

	// Cache it
	p.cache[cacheKey] = plan

	return plan, nil
}

// HasAPIKey returns true if the planner has an API key available.
func (p *Planner) HasAPIKey() bool {
	return p.apiKey != ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// urlPattern extracts a cacheable pattern from a URL.
// Strips query params and normalizes numeric/UUID path segments.
func urlPattern(rawURL string) string {
	// Simple: just use the path without query params for now
	// The URL fingerprinting from urlfingerprint.go handles the rest
	return rawURL
}
