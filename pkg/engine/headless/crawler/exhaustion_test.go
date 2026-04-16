package crawler

import (
	"testing"

	"github.com/adrianbrad/queue"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAffinityQueue_ExhaustedOriginDeprioritized verifies that a non-exhausted
// action wins over a higher-static-priority exhausted action during dequeue.
func TestAffinityQueue_ExhaustedOriginDeprioritized(t *testing.T) {
	exhaustedOrigins := map[string]bool{
		"origin-A": true,
	}
	exhaustionChecker := func(originID string) bool {
		return exhaustedOrigins[originID]
	}

	// Create actions: exhausted origin-A has higher priority (lower number = higher priority),
	// non-exhausted origin-B has lower priority.
	exhaustedAction := &types.Action{
		OriginID: "origin-A",
		Type:     types.ActionTypeLoadURL,
		Input:    "http://example.com/a",
		Depth:    0, // priority = 0 + 0*10 = 0 (highest)
	}
	freshAction := &types.Action{
		OriginID: "origin-B",
		Type:     types.ActionTypeLeftClick,
		Input:    "http://example.com/b",
		Depth:    0,
		Element: &types.HTMLElement{
			TagName: "button",
		},
		// priority = 4 + 0*10 = 4 (lower priority than exhaustedAction)
	}

	actions := []*types.Action{exhaustedAction, freshAction}
	pq := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})

	aq := NewAffinityQueue(pq, nil, exhaustionChecker)

	// GetPreferring with empty origin should use exhaustion-aware dequeue
	got, err := aq.GetPreferring("", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.OriginID != "origin-B" {
		t.Errorf("expected non-exhausted origin-B to win, got origin=%s", got.OriginID)
	}

	// The exhausted action should still be in the queue (not dropped)
	remaining, err := aq.Get()
	if err != nil {
		t.Fatalf("unexpected error getting remaining: %v", err)
	}
	if remaining.OriginID != "origin-A" {
		t.Errorf("expected exhausted origin-A to remain in queue, got origin=%s", remaining.OriginID)
	}
}

// TestAffinityQueue_ExhaustedOriginFallback verifies that when all actions are
// from exhausted origins, the best one is returned anyway (never dropped).
func TestAffinityQueue_ExhaustedOriginFallback(t *testing.T) {
	exhaustionChecker := func(originID string) bool {
		return true // all origins exhausted
	}

	action := &types.Action{
		OriginID: "origin-X",
		Type:     types.ActionTypeLoadURL,
		Input:    "http://example.com/x",
		Depth:    0,
	}

	actions := []*types.Action{action}
	pq := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})

	aq := NewAffinityQueue(pq, nil, exhaustionChecker)

	got, err := aq.GetPreferring("", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OriginID != "origin-X" {
		t.Errorf("expected fallback to return origin-X, got origin=%s", got.OriginID)
	}
}

// TestAffinityQueue_ExhaustedPreferredOriginSkipsAffinity verifies that when the
// preferred origin is exhausted, affinity is skipped and a non-exhausted action wins.
func TestAffinityQueue_ExhaustedPreferredOriginSkipsAffinity(t *testing.T) {
	exhaustedOrigins := map[string]bool{
		"origin-A": true,
	}
	exhaustionChecker := func(originID string) bool {
		return exhaustedOrigins[originID]
	}

	exhaustedAction := &types.Action{
		OriginID: "origin-A",
		Type:     types.ActionTypeLoadURL,
		Input:    "http://example.com/a",
		Depth:    0,
	}
	freshAction := &types.Action{
		OriginID: "origin-B",
		Type:     types.ActionTypeLeftClick,
		Input:    "http://example.com/b",
		Depth:    0,
		Element: &types.HTMLElement{
			TagName: "button",
		},
	}

	actions := []*types.Action{exhaustedAction, freshAction}
	pq := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})

	aq := NewAffinityQueue(pq, nil, exhaustionChecker)

	// Even though we prefer origin-A, it's exhausted so we should get origin-B
	got, err := aq.GetPreferring("origin-A", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OriginID != "origin-B" {
		t.Errorf("expected non-exhausted origin-B, got origin=%s", got.OriginID)
	}
}

// TestAffinityQueue_NilExhaustionChecker verifies that a nil exhaustion checker
// preserves existing behavior (no origin deprioritization).
func TestAffinityQueue_NilExhaustionChecker(t *testing.T) {
	action := &types.Action{
		OriginID: "origin-A",
		Type:     types.ActionTypeLoadURL,
		Input:    "http://example.com/a",
		Depth:    0,
	}

	actions := []*types.Action{action}
	pq := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})

	aq := NewAffinityQueue(pq, nil, nil)

	got, err := aq.GetPreferring("", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OriginID != "origin-A" {
		t.Errorf("expected origin-A, got origin=%s", got.OriginID)
	}
}

func TestOriginExhaustion_ThreeStrikesMarksExhausted(t *testing.T) {
	c := &Crawler{
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}

	origin := "page-abc123"
	assert.False(t, c.isOriginExhausted(origin))

	c.recordOriginYield(origin, 0, 0)
	assert.False(t, c.isOriginExhausted(origin))

	c.recordOriginYield(origin, 0, 0)
	assert.False(t, c.isOriginExhausted(origin))

	c.recordOriginYield(origin, 0, 0)
	assert.True(t, c.isOriginExhausted(origin), "should be exhausted after 3 zero-yield actions")
}

func TestOriginExhaustion_PositiveYieldResetsStrikes(t *testing.T) {
	c := &Crawler{
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}

	origin := "page-abc123"
	c.recordOriginYield(origin, 0, 0)
	c.recordOriginYield(origin, 0, 0)

	c.recordOriginYield(origin, 3, 0)
	assert.False(t, c.isOriginExhausted(origin))
	assert.Equal(t, 0, c.originStrikes[origin])
}

func TestOriginExhaustion_CoverageGainIgnoredForExhaustion(t *testing.T) {
	// Coverage gain should NOT affect exhaustion at all — only new navigations matter.
	// Opening search dialogs, toggling UI state, etc. triggers lots of JS coverage
	// without discovering new attack surface.
	c := &Crawler{
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}

	origin := "page-abc123"
	c.recordOriginYield(origin, 0, 0) // strike 1
	c.recordOriginYield(origin, 0, 0) // strike 2
	c.recordOriginYield(origin, 0, 0) // strike 3
	assert.True(t, c.isOriginExhausted(origin), "should be exhausted — coverage gain is irrelevant")
}

func TestOriginExhaustion_ResetAfterExhaustion(t *testing.T) {
	c := &Crawler{
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}

	origin := "page-abc123"
	c.recordOriginYield(origin, 0, 0)
	c.recordOriginYield(origin, 0, 0)
	c.recordOriginYield(origin, 0, 0)
	assert.True(t, c.isOriginExhausted(origin))

	c.recordOriginYield(origin, 1, 0)
	assert.False(t, c.isOriginExhausted(origin))
}

// TestAffinityQueue_ExhaustedOriginFallbackMultiAction verifies that when ALL
// origins are exhausted, multiple actions are still returned (not dropped),
// and the queue eventually empties.
func TestAffinityQueue_ExhaustedOriginFallbackMultiAction(t *testing.T) {
	exhaustedOrigin := "page-exhausted"

	actions := []*types.Action{
		{Type: types.ActionTypeLeftClick, OriginID: exhaustedOrigin, Depth: 1,
			Element: &types.HTMLElement{TagName: "BUTTON"}},
		{Type: types.ActionTypeLeftClick, OriginID: exhaustedOrigin, Depth: 1,
			Element: &types.HTMLElement{TagName: "BUTTON"}},
	}

	pq := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})

	isExhausted := func(originID string) bool { return true }
	aq := NewAffinityQueue(pq, nil, isExhausted)

	// Both actions should still be returned even though exhausted
	a1, err := aq.GetPreferring("some-origin", 20)
	require.NoError(t, err)
	assert.NotNil(t, a1)

	a2, err := aq.GetPreferring("some-origin", 20)
	require.NoError(t, err)
	assert.NotNil(t, a2)

	// Queue should now be empty
	_, err = aq.GetPreferring("some-origin", 20)
	assert.Error(t, err)
}

// TestAffinityQueue_AffinitySkipsExhaustedOrigin verifies that even if the
// preferred origin matches, it is skipped when exhausted and a non-exhausted
// action from a different origin is returned instead.
func TestAffinityQueue_AffinitySkipsExhaustedOrigin(t *testing.T) {
	exhaustedOrigin := "page-exhausted"
	freshOrigin := "page-fresh"

	actions := []*types.Action{
		{Type: types.ActionTypeLeftClick, OriginID: exhaustedOrigin, Depth: 1,
			Element: &types.HTMLElement{TagName: "A"}},
		{Type: types.ActionTypeLeftClick, OriginID: freshOrigin, Depth: 1,
			Element: &types.HTMLElement{TagName: "BUTTON"}},
	}

	pq := queue.NewPriority(actions, func(a, b *types.Action) bool {
		return actionPriority(a) < actionPriority(b)
	})

	isExhausted := func(originID string) bool { return originID == exhaustedOrigin }
	aq := NewAffinityQueue(pq, nil, isExhausted)

	// Request affinity for exhausted origin — should NOT match it, should get fresh action
	got, err := aq.GetPreferring(exhaustedOrigin, 20)
	require.NoError(t, err)
	assert.Equal(t, freshOrigin, got.OriginID, "should skip exhausted origin even when it matches affinity preference")
}

func TestOriginExhaustion_RequestYieldPreventsExhaustion(t *testing.T) {
	c := &Crawler{
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}

	origin := "page-abc123"
	c.recordOriginYield(origin, 0, 0) // strike 1
	c.recordOriginYield(origin, 0, 0) // strike 2

	// Action triggered API calls (newRequests > 0) but no DOM navigations
	c.recordOriginYield(origin, 0, 5)
	assert.False(t, c.isOriginExhausted(origin), "request yield should prevent exhaustion")
	assert.Equal(t, 0, c.originStrikes[origin], "request yield should reset strikes")
}

func TestOriginExhaustion_ZeroNavsZeroRequestsExhausts(t *testing.T) {
	c := &Crawler{
		originStrikes:    make(map[string]int),
		exhaustedOrigins: make(map[string]bool),
	}

	origin := "page-abc123"
	c.recordOriginYield(origin, 0, 0) // strike 1
	c.recordOriginYield(origin, 0, 0) // strike 2
	c.recordOriginYield(origin, 0, 0) // strike 3
	assert.True(t, c.isOriginExhausted(origin), "zero navs + zero requests should exhaust")
}

func TestClassifyAction_LinkNoBoost(t *testing.T) {
	// Links should NOT get a priority boost — boosting them causes them to
	// fire before SPA auth session settles, breaking navigation.
	// The penalty on dialog triggers/toggles effectively moves links up.
	a := &types.Action{
		Type:      types.ActionTypeLeftClick,
		OriginURL: "https://example.com/task",
		Element: &types.HTMLElement{
			TagName:    "A",
			Classes:    "peer/menu-button group/nav sidebar-link",
			Attributes: map[string]string{"href": "/agents"},
		},
	}
	adj := classifyAction(a)
	assert.Equal(t, 0, adj, "links should get no priority adjustment")
}

func TestClassifyAction_DialogTriggerButton(t *testing.T) {
	a := &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:     "BUTTON",
			TextContent: "Search⌘K",
			Attributes:  map[string]string{},
		},
	}
	adj := classifyAction(a)
	assert.Equal(t, 3, adj, "button with keyboard shortcut should get +3 penalty")
}

func TestClassifyAction_AriaHasPopupButton(t *testing.T) {
	a := &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:     "BUTTON",
			TextContent: "User Profile",
			Attributes:  map[string]string{"aria-haspopup": "true"},
		},
	}
	adj := classifyAction(a)
	assert.Equal(t, 3, adj, "button with aria-haspopup should get +3 penalty")
}

func TestClassifyAction_ExpandedToggle(t *testing.T) {
	a := &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:    "DIV",
			Attributes: map[string]string{"aria-expanded": "false"},
		},
	}
	adj := classifyAction(a)
	assert.Equal(t, 2, adj, "cursor-interactive with aria-expanded should get +2 penalty")
}

func TestClassifyAction_RegularButton(t *testing.T) {
	a := &types.Action{
		Type: types.ActionTypeLeftClick,
		Element: &types.HTMLElement{
			TagName:     "BUTTON",
			TextContent: "Submit Report",
			Attributes:  map[string]string{},
		},
	}
	adj := classifyAction(a)
	assert.Equal(t, 0, adj, "regular button without dialog/toggle signals should get no adjustment")
}
