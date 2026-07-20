package cartography

import (
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// LinkFeatures are the stable-ish attributes used to recognize a door across revisits.
type LinkFeatures struct {
	Tag        string
	ID         string
	CSSPath    string
	Text       string
	PathOnly   string // URL path without query/fragment
	QueryKeys  []string
	ActionType types.ActionType
}

// LinkIdentity tracks which features remain stable for a logical door.
type LinkIdentity struct {
	mu       sync.Mutex
	core     LinkFeatures
	visits   int
	unstable map[string]int // feature name -> times it changed
}

// Observe updates core features: values that change are marked unstable and cleared from core.
func (l *LinkIdentity) Observe(f LinkFeatures) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.visits++
	if l.unstable == nil {
		l.unstable = make(map[string]int)
	}
	if l.visits == 1 {
		l.core = normalizeFeatures(f)
		return
	}
	cur := normalizeFeatures(f)
	l.dropIfChanged("tag", l.core.Tag, cur.Tag, func() { l.core.Tag = "" })
	l.dropIfChanged("id", l.core.ID, cur.ID, func() { l.core.ID = "" })
	l.dropIfChanged("css", l.core.CSSPath, cur.CSSPath, func() { l.core.CSSPath = "" })
	l.dropIfChanged("text", l.core.Text, cur.Text, func() { l.core.Text = "" })
	l.dropIfChanged("path", l.core.PathOnly, cur.PathOnly, func() { l.core.PathOnly = "" })
	if !sameStringSet(l.core.QueryKeys, cur.QueryKeys) {
		l.unstable["query_keys"]++
		l.core.QueryKeys = nil
	}
	if l.core.ActionType != "" && cur.ActionType != "" && l.core.ActionType != cur.ActionType {
		l.unstable["action_type"]++
		l.core.ActionType = ""
	}
}

func (l *LinkIdentity) dropIfChanged(name, old, neu string, clear func()) {
	if old == "" || neu == "" {
		return
	}
	if old != neu {
		l.unstable[name]++
		clear()
	}
}

// Core returns a snapshot of features still considered stable.
func (l *LinkIdentity) Core() LinkFeatures {
	if l == nil {
		return LinkFeatures{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.core
	if len(l.core.QueryKeys) > 0 {
		out.QueryKeys = append([]string(nil), l.core.QueryKeys...)
	}
	return out
}

// Visits returns how many observations were recorded.
func (l *LinkIdentity) Visits() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.visits
}

// Matches reports whether f agrees with remaining core features.
// Empty fields on f are ignored so partial observations can still match, but a
// match requires positive agreement on at least one discriminating feature:
// otherwise a fully churned (empty-core) identity would match every candidate
// and silently swallow the rest of the crawl.
func (l *LinkIdentity) Matches(f LinkFeatures) bool {
	core := l.Core()
	f = normalizeFeatures(f)

	agreed := false
	// strong compares a discriminating feature: a contradiction fails the match
	// and positive agreement satisfies the "at least one real signal" rule.
	strong := func(a, b string) bool {
		if a == "" || b == "" {
			return true
		}
		if a != b {
			return false
		}
		agreed = true
		return true
	}
	// weak features (tag, action type) are shared by whole classes of elements
	// (every <a>, every form). They can only reject a match, never confirm one.
	weak := func(a, b string) bool {
		return a == "" || b == "" || a == b
	}
	if !strong(core.ID, f.ID) ||
		!strong(core.CSSPath, f.CSSPath) ||
		!strong(core.Text, f.Text) ||
		!strong(core.PathOnly, f.PathOnly) {
		return false
	}
	if len(core.QueryKeys) > 0 && len(f.QueryKeys) > 0 {
		if !sameStringSet(core.QueryKeys, f.QueryKeys) {
			return false
		}
		agreed = true
	}
	if !weak(core.Tag, f.Tag) || !weak(string(core.ActionType), string(f.ActionType)) {
		return false
	}
	return agreed
}

// FeaturesFromAction extracts link features from a crawl action.
func FeaturesFromAction(a *types.Action) LinkFeatures {
	if a == nil {
		return LinkFeatures{}
	}
	f := LinkFeatures{ActionType: a.Type}
	if a.Element != nil {
		f.Tag = a.Element.TagName
		f.ID = a.Element.ID
		f.CSSPath = a.Element.CSSSelector
		f.Text = strings.TrimSpace(a.Element.TextContent)
		if href := a.Element.Attributes["href"]; href != "" {
			f.PathOnly, f.QueryKeys = splitURLFeatures(href)
		}
	}
	if a.Type == types.ActionTypeLoadURL && a.Input != "" {
		f.PathOnly, f.QueryKeys = splitURLFeatures(a.Input)
	}
	return normalizeFeatures(f)
}

func splitURLFeatures(raw string) (pathOnly string, queryKeys []string) {
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		// A malformed href has no meaningful path; returning the raw string
		// would plant a bogus PathOnly feature that never matches anything.
		return "", nil
	}
	pathOnly = u.Path
	for k := range u.Query() {
		queryKeys = append(queryKeys, k)
	}
	sort.Strings(queryKeys)
	return pathOnly, queryKeys
}

func normalizeFeatures(f LinkFeatures) LinkFeatures {
	f.Tag = strings.ToLower(strings.TrimSpace(f.Tag))
	f.ID = strings.TrimSpace(f.ID)
	f.CSSPath = strings.TrimSpace(f.CSSPath)
	f.Text = strings.Join(strings.Fields(f.Text), " ")
	f.PathOnly = strings.TrimSpace(f.PathOnly)
	if len(f.QueryKeys) > 0 {
		keys := append([]string(nil), f.QueryKeys...)
		sort.Strings(keys)
		f.QueryKeys = keys
	}
	return f
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
