package explorer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/projectdiscovery/gologger"
)

// ExecuteDirectActions runs button clicks, form fills, and menu expansions
// directly on the current page using the snapshot's element refs with JS .click().
// Link clicks are SKIPPED — FindNavigations() handles them.
func ExecuteDirectActions(plan *ExplorationPlan, snapshot *PageSnapshot) {
	if plan == nil || snapshot == nil {
		return
	}

	// Sort by priority descending
	actions := make([]PlannedAction, len(plan.Actions))
	copy(actions, plan.Actions)
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority > actions[j].Priority
	})

	// Filter to only direct actions (not links, not skip)
	var directActions []PlannedAction
	for _, a := range actions {
		if a.Priority == 0 {
			continue
		}
		if a.Ref > 0 {
			// Check if it's a link — skip those
			for _, r := range snapshot.Refs {
				if r.Ref == a.Ref && r.Tag == "A" && r.Href != "" {
					goto skip // FindNavigations() handles links
				}
			}
		}
		directActions = append(directActions, a)
	skip:
	}

	if len(directActions) == 0 {
		return
	}

	gologger.Info().Msgf("[explorer] Executing %d direct actions (buttons, forms, menus)", len(directActions))

	executed := 0
	failed := 0

	for _, action := range directActions {
		if action.Ref == 0 && action.FormRef == 0 {
			continue
		}

		switch action.Action {
		case "click", "expand_menu":
			if err := snapshot.ClickRef(action.Ref); err != nil {
				failed++
				gologger.Info().Msgf("[explorer] [P%d] FAILED %s %s — %s",
					action.Priority, action.Action, describeRef(snapshot, action.Ref), err)
			} else {
				executed++
				time.Sleep(500 * time.Millisecond)
				gologger.Info().Msgf("[explorer] [P%d] %s %s — %s",
					action.Priority, strings.ToUpper(action.Action), describeRef(snapshot, action.Ref), action.Reason)
			}

		case "fill_and_submit":
			filledCount := 0
			for refStr, fill := range action.Fields {
				var ref int
				_, _ = fmt.Sscanf(refStr, "%d", &ref)
				if ref == 0 {
					continue
				}

				value := fill.Value
				if value == "" {
					// Smart fill from field name
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

				if fill.Type == "select" {
					if err := snapshot.SelectRef(ref, value); err == nil {
						filledCount++
					}
				} else {
					if err := snapshot.TypeRef(ref, value); err == nil {
						filledCount++
						gologger.Info().Msgf("[explorer]   @%d typed %q", ref, trunc(value, 30))
					}
				}
			}

			if filledCount > 0 {
				// Try to submit — find a submit button in the form
				submitted := false
				if action.FormRef > 0 {
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
					// Try clicking any submit button on the page
					for _, r := range snapshot.Refs {
						if r.Type == "submit" || (r.Tag == "BUTTON" && strings.Contains(strings.ToLower(r.Name), "submit")) {
							if err := snapshot.ClickRef(r.Ref); err == nil {
								submitted = true
								break
							}
						}
					}
				}

				time.Sleep(1 * time.Second)
				executed++
				gologger.Info().Msgf("[explorer] [P%d] FORM filled %d fields — %s",
					action.Priority, filledCount, action.Reason)
			} else {
				failed++
				gologger.Info().Msgf("[explorer] [P%d] FORM failed — no fields filled — %s",
					action.Priority, action.Reason)
			}
		}
	}

	gologger.Info().Msgf("[explorer] Direct actions: %d executed, %d failed", executed, failed)
}
