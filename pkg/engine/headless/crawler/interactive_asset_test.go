package crawler

import (
	"strconv"
	"testing"

	"github.com/adrianbrad/queue"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryTargetsIncludesParentPrefixes(t *testing.T) {
	targets := discoveryTargets("https://app.example.com/admin/users/edit")
	require.Contains(t, targets, "https://app.example.com/admin/users/edit")
	require.Contains(t, targets, "https://app.example.com/admin/users")
	require.Contains(t, targets, "https://app.example.com/admin")
	require.NotContains(t, targets, "https://app.example.com")
}

func TestDiscoveryTargetsRejectsNonHTTP(t *testing.T) {
	require.Nil(t, discoveryTargets("about:blank"))
	require.Nil(t, discoveryTargets("javascript:void(0)"))
	require.Nil(t, discoveryTargets(""))
}

func TestEnqueueURLOffersPageAndParents(t *testing.T) {
	q := queue.NewLinked([]*types.Action{})
	c := &Crawler{
		crawlQueue:    q,
		uniqueActions: make(map[string]struct{}),
		options: Options{
			ScopeValidator: func(u string) bool {
				return true
			},
		},
	}
	e := &interactiveExplorer{crawler: c}
	e.enqueueURL("https://app.example.com/settings/profile", 1)

	seen := map[string]bool{}
	for q.Size() > 0 {
		action, err := q.Get()
		require.NoError(t, err)
		require.Equal(t, types.ActionTypeLoadURL, action.Type)
		seen[action.Input] = true
	}
	require.True(t, seen["https://app.example.com/settings/profile"])
	require.True(t, seen["https://app.example.com/settings"])
}

func TestEnqueueURLDedupesAndHonorsScope(t *testing.T) {
	q := queue.NewLinked([]*types.Action{})
	c := &Crawler{
		crawlQueue:    q,
		uniqueActions: make(map[string]struct{}),
		options: Options{
			ScopeValidator: func(u string) bool {
				return u == "https://in.example.com/app" || u == "https://in.example.com/app/x"
			},
		},
	}
	e := &interactiveExplorer{crawler: c}
	e.enqueueURL("https://in.example.com/app/x", 2)
	e.enqueueURL("https://in.example.com/app/x", 2) // duplicate
	e.enqueueURL("https://out.example.com/app/x", 2)

	count := 0
	for q.Size() > 0 {
		action, err := q.Get()
		require.NoError(t, err)
		require.True(t, c.options.ScopeValidator(action.Input))
		count++
	}
	require.Equal(t, 2, count) // /app/x and parent /app
}

func TestPrioritizeNavigationCandidatesOrdersLinksFirst(t *testing.T) {
	items := []*types.HTMLElement{
		{TagName: "BUTTON", XPath: "/btn", Attributes: map[string]string{}},
		{TagName: "A", XPath: "/a", Attributes: map[string]string{"href": "/admin"}},
		{TagName: "DIV", XPath: "/div", Attributes: map[string]string{"role": "tab"}},
	}
	ordered := prioritizeNavigationCandidates(items)
	require.Equal(t, "/a", ordered[0].XPath)
	require.Equal(t, "/div", ordered[1].XPath)
	require.Equal(t, "/btn", ordered[2].XPath)
}

func TestIsNavigationCandidate(t *testing.T) {
	require.True(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "A", Attributes: map[string]string{"href": "/x"},
	}))
	require.False(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "A", Attributes: map[string]string{"href": "#"},
	}))
	require.True(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "DIV", Attributes: map[string]string{"role": "menuitem"},
	}))
	require.False(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "BUTTON", Attributes: map[string]string{},
	}))
}

func TestDiscoveryTargetsLimitsParentPrefixes(t *testing.T) {
	targets := discoveryTargets("https://app.example.com/a/b/c/d/e/f")
	require.Contains(t, targets, "https://app.example.com/a/b/c/d/e/f")
	// Keep the shallowest maxParentPathPrefixes parents (not deepest-first[:N]).
	parents := 0
	for _, target := range targets {
		if target != "https://app.example.com/a/b/c/d/e/f" {
			parents++
		}
	}
	require.Equal(t, maxParentPathPrefixes, parents)
	require.Contains(t, targets, "https://app.example.com/a")
	require.Contains(t, targets, "https://app.example.com/a/b")
	require.Contains(t, targets, "https://app.example.com/a/b/c")
	require.NotContains(t, targets, "https://app.example.com/a/b/c/d")
	require.NotContains(t, targets, "https://app.example.com/a/b/c/d/e")
}

func TestLoadURLDedupeKeyMatchesActionHash(t *testing.T) {
	url := "https://app.example.com/x"
	require.Equal(t, loadURLDedupeKey(url), (&types.Action{Type: types.ActionTypeLoadURL, Input: url}).Hash())
	require.Equal(t, "load_url|"+url+"|", loadURLDedupeKey(url))
}

func TestOfferLoadURLClampsDepthToMaxDepth(t *testing.T) {
	q := queue.NewLinked([]*types.Action{})
	c := &Crawler{
		crawlQueue:    q,
		uniqueActions: make(map[string]struct{}),
		options: Options{
			MaxDepth: 2,
			ScopeValidator: func(u string) bool {
				return true
			},
		},
	}
	e := &interactiveExplorer{crawler: c}
	e.offerLoadURL("https://app.example.com/deep", 9)
	action, err := q.Get()
	require.NoError(t, err)
	require.Equal(t, 2, action.Depth)
}

func TestClampEnqueueDepth(t *testing.T) {
	require.Equal(t, 2, clampEnqueueDepth(9, 2))
	require.Equal(t, 3, clampEnqueueDepth(3, 5))
	require.Equal(t, 7, clampEnqueueDepth(7, 0)) // MaxDepth 0 = unlimited
	require.Equal(t, 0, clampEnqueueDepth(-1, 5))
}

func TestIsNavigationCandidateRejectsJavascriptDataAttrs(t *testing.T) {
	require.False(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "DIV", Attributes: map[string]string{"data-href": "javascript:void(0)"},
	}))
	require.False(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "DIV", Attributes: map[string]string{"data-url": "javascript:alert(1)"},
	}))
	require.True(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "DIV", Attributes: map[string]string{"data-href": "/admin"},
	}))
}

func TestIsNavigationCandidateRejectsJavascriptHrefCaseInsensitive(t *testing.T) {
	require.False(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "A", Attributes: map[string]string{"href": "JavaScript:void(0)"},
	}))
	require.False(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "A", Attributes: map[string]string{"href": "JAVASCRIPT:alert(1)"},
	}))
	require.True(t, isNavigationCandidate(&types.HTMLElement{
		TagName: "A", Attributes: map[string]string{"href": "/admin"},
	}))
}

func TestOfferLoadURLReturnsWhetherNewlyEnqueued(t *testing.T) {
	q := queue.NewLinked([]*types.Action{})
	c := &Crawler{
		crawlQueue:    q,
		uniqueActions: make(map[string]struct{}),
		options: Options{
			ScopeValidator: func(u string) bool { return true },
		},
	}
	e := &interactiveExplorer{crawler: c}
	require.True(t, e.offerLoadURL("https://app.example.com/a.js", 1))
	require.False(t, e.offerLoadURL("https://app.example.com/a.js", 1))
	require.False(t, e.offerLoadURL("about:blank", 1))
}

func TestHarvestAssetsCapsSuccessfulNewOffersPreferringEarlierURLs(t *testing.T) {
	// Models harvestAssets: dynamic-first ordering + cap on successful NEW offers.
	// Pre-seen static scripts must not consume the cap and crowd out chunks.
	q := queue.NewLinked([]*types.Action{})
	c := &Crawler{
		crawlQueue:    q,
		uniqueActions: make(map[string]struct{}),
		options: Options{
			ScopeValidator: func(u string) bool { return true },
		},
	}
	e := &interactiveExplorer{crawler: c}
	for i := 0; i < 60; i++ {
		staticURL := "https://cdn.example.com/static/" + strconv.Itoa(i) + ".js"
		c.uniqueActions[loadURLDedupeKey(staticURL)] = struct{}{}
	}

	ordered := make([]string, 0, 120)
	for i := 0; i < 10; i++ {
		ordered = append(ordered, "https://cdn.example.com/chunk/"+strconv.Itoa(i)+".js")
	}
	for i := 0; i < 60; i++ {
		ordered = append(ordered, "https://cdn.example.com/static/"+strconv.Itoa(i)+".js")
	}
	for i := 10; i < 55; i++ {
		ordered = append(ordered, "https://cdn.example.com/chunk/"+strconv.Itoa(i)+".js")
	}

	offered := 0
	for _, assetURL := range ordered {
		if offered >= maxHarvestedAssetsPerPage {
			break
		}
		if e.offerLoadURL(assetURL, 1) {
			offered++
		}
	}
	require.Equal(t, maxHarvestedAssetsPerPage, offered)

	seen := map[string]bool{}
	for c.crawlQueue.Size() > 0 {
		action, err := c.crawlQueue.Get()
		require.NoError(t, err)
		seen[action.Input] = true
	}
	require.True(t, seen["https://cdn.example.com/chunk/0.js"])
	require.True(t, seen["https://cdn.example.com/chunk/49.js"])
	require.False(t, seen["https://cdn.example.com/chunk/50.js"])
	require.False(t, seen["https://cdn.example.com/static/0.js"])
}

func TestURLSeedFrontierDedupesRootAndDrainsFIFO(t *testing.T) {
	f := newURLSeedFrontier("https://app.example.com/")
	require.False(t, f.push("https://app.example.com/"), "root must already be marked queued")
	require.True(t, f.push("https://app.example.com/a"))
	require.True(t, f.push("https://app.example.com/b"))
	require.False(t, f.push("https://app.example.com/a"))
	first, ok := f.pop()
	require.True(t, ok)
	require.Equal(t, "https://app.example.com/a", first)
	second, ok := f.pop()
	require.True(t, ok)
	require.Equal(t, "https://app.example.com/b", second)
	_, ok = f.pop()
	require.False(t, ok)
}

func TestURLSeedFrontierNilSafe(t *testing.T) {
	var f *urlSeedFrontier
	require.False(t, f.push("https://app.example.com/x"))
	_, ok := f.pop()
	require.False(t, ok)
}

func TestPageAssetURLsScriptTruncatesInBrowser(t *testing.T) {
	script := pageAssetURLsScript(50)
	require.Contains(t, script, "const max = 50")
	require.Contains(t, script, "dynamic.length >= max")
	require.Contains(t, script, ".slice(0, max)")
	require.Contains(t, script, "max - dynamic.length")
	// Fallback when max < 1 uses the package constant.
	fallback := pageAssetURLsScript(0)
	require.Contains(t, fallback, "const max = "+strconv.Itoa(maxHarvestedAssetsPerPage))
}
