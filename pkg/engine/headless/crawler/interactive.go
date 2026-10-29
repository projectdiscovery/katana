package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/projectdiscovery/katana/pkg/utils"
)

// urlSeedFrontier holds page URLs discovered via navigation that should be
// interactively explored iteratively. Sharing one frontier across explorers
// (root + popups) prevents URL-change recursion from growing the Go stack
// with InteractionMaxStates.
type urlSeedFrontier struct {
	pending []string
	queued  map[string]struct{}
}

func newURLSeedFrontier(rootURL string) *urlSeedFrontier {
	f := &urlSeedFrontier{queued: make(map[string]struct{})}
	if rootURL != "" {
		f.queued[rootURL] = struct{}{}
	}
	return f
}

func (f *urlSeedFrontier) push(rawURL string) bool {
	if f == nil || rawURL == "" {
		return false
	}
	if _, ok := f.queued[rawURL]; ok {
		return false
	}
	f.queued[rawURL] = struct{}{}
	f.pending = append(f.pending, rawURL)
	return true
}

func (f *urlSeedFrontier) pop() (string, bool) {
	if f == nil || len(f.pending) == 0 {
		return "", false
	}
	u := f.pending[0]
	f.pending = f.pending[1:]
	return u, true
}

type interactiveExplorer struct {
	crawler  *Crawler
	page     *browser.BrowserPage
	ctx      context.Context
	rootURL  string
	visited  map[string]struct{}
	attempts int
	success  int
	failures int
	seeds    *urlSeedFrontier
}

// reserveAction counts one click toward InteractionMaxActions.
// Returns false when the budget is already exhausted (attempts unchanged).
func (e *interactiveExplorer) reserveAction() bool {
	if limit := e.crawler.options.InteractionMaxActions; limit > 0 && e.attempts >= limit {
		return false
	}
	e.attempts++
	return true
}

func (c *Crawler) crawlInteractive(ctx context.Context, rootURL string) error {
	page, err := c.launcher.GetPageFromPool()
	if err != nil {
		return err
	}
	originalPage := page.Page
	defer func() {
		page.Page = originalPage
		c.launcher.PutBrowserToPool(page)
	}()
	page.Page = page.Context(ctx)
	if err := page.Timeout(c.options.PageMaxTimeout).Navigate(rootURL); err != nil {
		return err
	}
	if err := page.WaitPageLoadHeurisitics(); err != nil {
		return err
	}
	e := &interactiveExplorer{
		crawler: c, page: page, ctx: ctx, rootURL: rootURL,
		visited: make(map[string]struct{}),
		seeds:   newURLSeedFrontier(rootURL),
	}
	err = e.explore(0, nil)
	// Drain URL seeds iteratively. New in-scope page URLs discovered mid-click
	// are queued (not explored via recursive explore(0)), so deep navigation
	// chains cannot grow the call stack with InteractionMaxStates.
	for err == nil {
		seed, ok := e.seeds.pop()
		if !ok {
			break
		}
		if ctxErr := e.ctx.Err(); ctxErr != nil {
			err = ctxErr
			break
		}
		if limit := e.crawler.options.InteractionMaxStates; limit > 0 && len(e.visited) >= limit {
			break
		}
		if limit := e.crawler.options.InteractionMaxActions; limit > 0 && e.attempts >= limit {
			break
		}
		e.rootURL = seed
		if navErr := e.page.Timeout(e.crawler.options.PageMaxTimeout).Navigate(seed); navErr != nil {
			e.failures++
			c.logger.Debug("could not navigate interactive URL seed", slog.String("url", seed), slog.String("error", navErr.Error()))
			continue
		}
		if waitErr := e.page.WaitPageLoadHeurisitics(); waitErr != nil {
			e.failures++
			c.logger.Debug("interactive URL seed failed page-load wait", slog.String("url", seed), slog.String("error", waitErr.Error()))
			continue
		}
		if exploreErr := e.explore(0, nil); exploreErr != nil {
			if ctxErr := e.ctx.Err(); ctxErr != nil {
				err = ctxErr
				break
			}
			e.failures++
			c.logger.Debug("interactive URL seed explore failed", slog.String("url", seed), slog.String("error", exploreErr.Error()))
		}
	}
	c.logger.Info("interactive crawl finished",
		slog.Int("states", len(e.visited)),
		slog.Int("attempts", e.attempts),
		slog.Int("successful_clicks", e.success),
		slog.Int("failed_clicks", e.failures),
		slog.Int("url_seeds_queued", len(e.seeds.queued)),
	)
	return err
}

func (e *interactiveExplorer) explore(depth int, path []*types.HTMLElement) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	stateID, pageURL, err := interactiveState(e.page)
	if err != nil {
		return err
	}
	if _, ok := e.visited[stateID]; ok {
		return nil
	}
	if limit := e.crawler.options.InteractionMaxStates; limit > 0 && len(e.visited) >= limit {
		return nil
	}
	e.visited[stateID] = struct{}{}
	// Feed the current page and its scripts into the ordinary crawl queue
	// immediately so FindNavigations / output can consume them without waiting
	// for the interactive phase to finish.
	e.enqueueURL(pageURL, depth)
	e.harvestAssets(depth)
	if depth >= e.crawler.options.InteractionDepth {
		return nil
	}

	candidates, err := e.candidates(pageURL)
	if err != nil {
		return err
	}
	needsRestore := false
	for _, candidate := range candidates {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		// Recreate the parent only after one of our clicks changed it. Pages
		// with rotating ads or clocks can mutate between two observations.
		// Restore is cheap (back / direct Navigate); on failure we abandon the
		// remaining siblings instead of burning budget on root+full-path loops.
		if needsRestore {
			if err := e.restore(path, stateID, pageURL); err != nil {
				e.crawler.logger.Debug("could not restore interactive state; skipping remaining siblings", slog.String("error", err.Error()))
				e.failures++
				return nil
			}
			needsRestore = false
		}
		if !e.reserveAction() {
			return nil
		}
		beforeTabs, _ := e.page.Browser.Pages()
		if err := e.click(candidate); err != nil {
			e.failures++
			e.crawler.logger.Debug("interactive click failed", slog.String("xpath", candidate.XPath), slog.String("error", err.Error()))
			continue
		}
		e.success++
		e.settle(stateID)
		if err := e.explorePopup(beforeTabs, depth+1); err != nil {
			// One broken window.open must not abort sibling exploration.
			if ctxErr := e.ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			e.failures++
			e.crawler.logger.Debug("popup exploration failed", slog.String("error", err.Error()))
		}
		newID, newURL, err := interactiveState(e.page)
		if err != nil {
			e.failures++
			needsRestore = true
			continue
		}
		needsRestore = newID != stateID || newURL != pageURL
		if e.crawler.options.ScopeValidator != nil && !e.crawler.options.ScopeValidator(newURL) {
			continue
		}
		if newID == stateID {
			continue
		}
		if newURL != pageURL {
			// URL changed: harvest while performance entries are still present,
			// then queue an iterative URL seed (depth resets when drained).
			// Do not recurse explore(0) — that could nest up to MaxStates frames.
			e.enqueueURL(newURL, 0)
			e.harvestAssets(0)
			if e.seeds != nil {
				e.seeds.push(newURL)
			}
			continue
		}
		if err := e.explore(depth+1, appendPath(path, candidate)); err != nil {
			return err
		}
	}
	return nil
}

func (e *interactiveExplorer) explorePopup(before rod.Pages, depth int) error {
	seen := make(map[proto.TargetTargetID]struct{}, len(before))
	for _, page := range before {
		seen[page.TargetID] = struct{}{}
	}
	pages, err := e.page.Browser.Pages()
	if err != nil {
		if ctxErr := e.ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		e.failures++
		e.crawler.logger.Debug("could not list browser pages for popup exploration", slog.String("error", err.Error()))
		return nil
	}
	for _, page := range pages {
		if _, existed := seen[page.TargetID]; existed {
			continue
		}
		info, err := page.Info()
		if err != nil || info.OpenerID != e.page.TargetID {
			continue
		}
		if err := e.exploreOpenedPage(page, depth); err != nil {
			if ctxErr := e.ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			e.failures++
			e.crawler.logger.Debug("popup exploration failed", slog.String("error", err.Error()))
			continue
		}
	}
	return nil
}

func (e *interactiveExplorer) exploreOpenedPage(page *rod.Page, depth int) error {
	defer func() { _ = page.Close() }()
	// Install Fetch interception and page hooks immediately — before waiting
	// for a non-blank URL — so the popup's first navigation and early
	// subresources can be paused for scoped headers.
	// A reload after the URL is known covers the race where the document
	// request already left before we attached.
	popup, err := e.crawler.launcher.AdoptPopup(page.Context(e.ctx), e.page.Browser)
	if err != nil {
		return err
	}
	var popupURL string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && e.ctx.Err() == nil {
		info, err := popup.Info()
		if err != nil {
			return err
		}
		popupURL = info.URL
		if popupURL != "" && popupURL != "about:blank" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if popupURL == "" || popupURL == "about:blank" {
		e.crawler.logger.Debug("popup has no navigable URL")
		return nil
	}
	if validate := e.crawler.options.ScopeValidator; validate != nil && !validate(popupURL) {
		return nil
	}
	e.enqueueURL(popupURL, depth)
	if err := popup.Timeout(e.crawler.options.PageMaxTimeout).Navigate(popupURL); err != nil {
		return err
	}
	if err := popup.WaitPageLoadHeurisitics(); err != nil {
		return err
	}
	child := &interactiveExplorer{
		crawler: e.crawler, page: popup, ctx: e.ctx, rootURL: popupURL,
		visited: e.visited, attempts: e.attempts, success: e.success, failures: e.failures,
		seeds: e.seeds,
	}
	err = child.explore(depth, nil)
	// Sync even when explore fails so parent action budgets stay accurate.
	e.attempts, e.success, e.failures = child.attempts, child.success, child.failures
	return err
}

func appendPath(path []*types.HTMLElement, element *types.HTMLElement) []*types.HTMLElement {
	result := make([]*types.HTMLElement, len(path)+1)
	copy(result, path)
	result[len(path)] = element
	return result
}

// enqueueURL offers an in-scope URL (and its parent path prefixes) to the
// ordinary headless crawl queue so discoveries expand the crawl frontier in
// real time during interactive exploration.
func (e *interactiveExplorer) enqueueURL(rawURL string, depth int) {
	for _, target := range discoveryTargets(rawURL) {
		e.offerLoadURL(target, depth)
	}
}

// maxParentPathPrefixes caps how many parent directories we enqueue per discovery
// so deep paths cannot flood the ordinary crawl queue.
const maxParentPathPrefixes = 3

// maxHarvestedAssetsPerPage caps script/module URLs harvested from one DOM so a
// heavy page cannot enqueue unbounded chunk graphs during interaction.
const maxHarvestedAssetsPerPage = 50

func discoveryTargets(rawURL string) []string {
	if !isHTTPDiscoveryURL(rawURL) {
		return nil
	}
	targets := []string{rawURL}
	parents := utils.ExtractParentPaths(rawURL)
	// ExtractParentPaths is deepest-first; keep the shallowest N so directory
	// discovery favors /a and /a/b over near-leaf prefixes.
	if len(parents) > maxParentPathPrefixes {
		parents = parents[len(parents)-maxParentPathPrefixes:]
	}
	return append(targets, parents...)
}

func isHTTPDiscoveryURL(rawURL string) bool {
	if rawURL == "" || rawURL == "about:blank" {
		return false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// offerLoadURL enqueues rawURL when new and in scope. Returns true only when a
// brand-new action was accepted by the crawl queue (dedupe hits and Offer
// failures return false so harvest can cap on successful NEW offers).
func (e *interactiveExplorer) offerLoadURL(rawURL string, depth int) bool {
	if !isHTTPDiscoveryURL(rawURL) {
		return false
	}
	if validate := e.crawler.options.ScopeValidator; validate != nil && !validate(rawURL) {
		return false
	}
	if e.crawler.crawlQueue == nil {
		return false
	}
	action := &types.Action{
		Type:  types.ActionTypeLoadURL,
		Input: rawURL,
		Depth: clampEnqueueDepth(depth, e.crawler.options.MaxDepth),
	}
	// Unify with Action.Hash() for LoadURL with empty OriginID ("load_url|URL|")
	// so interactive discoveries and ordinary crawl dedupe share one keyspace.
	// Crawl() also registers this URL-only key alongside the seed OriginID hash.
	hash := action.Hash()
	if e.crawler.uniqueActions == nil {
		e.crawler.uniqueActions = make(map[string]struct{})
	}
	if _, ok := e.crawler.uniqueActions[hash]; ok {
		return false
	}
	e.crawler.uniqueActions[hash] = struct{}{}
	if err := e.crawler.crawlQueue.Offer(action); err != nil {
		delete(e.crawler.uniqueActions, hash)
		if e.crawler.logger != nil {
			e.crawler.logger.Debug("could not enqueue interactive discovery", slog.String("url", rawURL), slog.String("error", err.Error()))
		}
		return false
	}
	if e.crawler.logger != nil {
		e.crawler.logger.Debug("enqueued interactive discovery", slog.String("url", rawURL), slog.Int("depth", action.Depth))
	}
	return true
}

// clampEnqueueDepth keeps interactive discoveries within MaxDepth so the main
// crawl loop does not silently drop them (it skips action.Depth > MaxDepth).
func clampEnqueueDepth(depth, maxDepth int) int {
	if maxDepth > 0 && depth > maxDepth {
		return maxDepth
	}
	if depth < 0 {
		return 0
	}
	return depth
}

// harvestAssets collects script / modulepreload URLs from the current DOM and
// performance resource timeline, then enqueues them for ordinary crawl/output.
// Dynamic (performance) chunks are preferred; the per-page cap counts successful
// NEW offers only so already-seen static scripts do not crowd out fresh chunks.
func (e *interactiveExplorer) harvestAssets(depth int) {
	urls, err := e.pageAssetURLs()
	if err != nil {
		if e.crawler.logger != nil {
			e.crawler.logger.Debug("could not harvest page assets", slog.String("error", err.Error()))
		}
		return
	}
	offered := 0
	for _, assetURL := range urls {
		if offered >= maxHarvestedAssetsPerPage {
			break
		}
		if e.offerLoadURL(assetURL, depth) {
			offered++
		}
	}
}

// loadURLDedupeKey matches Action.Hash() for a LoadURL with empty OriginID.
func loadURLDedupeKey(rawURL string) string {
	return (&types.Action{Type: types.ActionTypeLoadURL, Input: rawURL}).Hash()
}

// pageAssetURLsScript builds the in-page collector. Truncation happens inside
// the browser (dynamic-first) before the result crosses CDP, so heavy pages
// cannot ship unbounded script URL arrays to Go.
func pageAssetURLsScript(max int) string {
	if max < 1 {
		max = maxHarvestedAssetsPerPage
	}
	return fmt.Sprintf(`() => {
		const max = %d;
		const dynamic = [];
		const staticScripts = [];
		const seen = new Set();
		const push = (u, bucket) => {
			if (!u || seen.has(u)) return;
			seen.add(u);
			bucket.push(u);
		};
		try {
			performance.getEntriesByType('resource').forEach(entry => {
				if (dynamic.length >= max) return;
				if (entry.initiatorType === 'script' || /\.m?js(\?|$)/i.test(entry.name)) push(entry.name, dynamic);
			});
		} catch (_) {}
		const room = Math.max(0, max - dynamic.length);
		document.querySelectorAll('script[src]').forEach(el => {
			if (staticScripts.length >= room) return;
			push(el.src, staticScripts);
		});
		document.querySelectorAll('link[rel="modulepreload"], link[rel="preload"][as="script"], link[rel="prefetch"][as="script"]').forEach(el => {
			if (staticScripts.length >= room) return;
			push(el.href, staticScripts);
		});
		return dynamic.concat(staticScripts).slice(0, max);
	}`, max)
}

func (e *interactiveExplorer) pageAssetURLs() ([]string, error) {
	// Dynamic/performance entries first so the NEW-offer cap prefers chunks
	// over already-present static <script src> tags when the page is heavy.
	value, err := e.page.Eval(pageAssetURLsScript(maxHarvestedAssetsPerPage))
	if err != nil {
		return nil, err
	}
	var urls []string
	if err := value.Value.Unmarshal(&urls); err != nil {
		return nil, err
	}
	if len(urls) > maxHarvestedAssetsPerPage {
		urls = urls[:maxHarvestedAssetsPerPage]
	}
	filtered := urls[:0]
	for _, u := range urls {
		if isHTTPDiscoveryURL(u) {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}

func (e *interactiveExplorer) candidates(pageURL string) ([]*types.HTMLElement, error) {
	items, err := e.page.GetInteractiveElements(e.crawler.options.InteractionSelector, e.crawler.options.InteractionExclude)
	if err != nil {
		return nil, err
	}
	// Event listeners on non-semantic nodes are additional click candidates.
	listeners, err := e.page.GetClickListenerElements()
	if err == nil {
		items = append(items, listeners...)
	}
	if e.crawler.options.InteractionExclude != "" {
		paths := make([]string, 0, len(items))
		for _, item := range items {
			if item != nil && item.XPath != "" {
				paths = append(paths, item.XPath)
			}
		}
		encoded, _ := json.Marshal(paths)
		value, err := e.page.Eval(`() => ` + string(encoded) + `.filter(path => {
			const el = document.evaluate(path, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
			return el && !el.closest(` + strconv.Quote(e.crawler.options.InteractionExclude) + `);
		})`)
		if err != nil {
			return nil, err
		}
		var allowed []string
		if err := value.Value.Unmarshal(&allowed); err != nil {
			return nil, err
		}
		keep := make(map[string]struct{}, len(allowed))
		for _, path := range allowed {
			keep[path] = struct{}{}
		}
		filtered := items[:0]
		for _, item := range items {
			if item != nil {
				if _, ok := keep[item.XPath]; ok {
					filtered = append(filtered, item)
				}
			}
		}
		items = filtered
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]*types.HTMLElement, 0, len(items))
	for _, item := range items {
		if item == nil || item.XPath == "" || isElementDisabledInteractive(item) || isLogoutPage(item) {
			continue
		}
		if item.TagName == "A" && item.Attributes["href"] != "" {
			href := item.Attributes["href"]
			if strings.HasPrefix(strings.ToLower(href), "javascript:") {
				continue
			}
			target, err := url.Parse(href)
			if err != nil || (target.Scheme != "" && target.Scheme != "http" && target.Scheme != "https") {
				continue
			}
			base, err := url.Parse(pageURL)
			if err != nil {
				continue
			}
			if validate := e.crawler.options.ScopeValidator; validate != nil && !validate(base.ResolveReference(target).String()) {
				continue
			}
		}
		if _, ok := seen[item.XPath]; ok {
			continue
		}
		seen[item.XPath] = struct{}{}
		result = append(result, item)
	}
	return prioritizeNavigationCandidates(result), nil
}

// prioritizeNavigationCandidates puts likely page-changing controls first so
// shallow budgets spend clicks on new URLs / chunks instead of dead-end UI.
func prioritizeNavigationCandidates(items []*types.HTMLElement) []*types.HTMLElement {
	if len(items) < 2 {
		return items
	}
	nav := make([]*types.HTMLElement, 0, len(items))
	other := make([]*types.HTMLElement, 0, len(items))
	for _, item := range items {
		if isNavigationCandidate(item) {
			nav = append(nav, item)
			continue
		}
		other = append(other, item)
	}
	return append(nav, other...)
}

func isNavigationCandidate(item *types.HTMLElement) bool {
	if item == nil {
		return false
	}
	if item.TagName == "A" {
		href := item.Attributes["href"]
		if href != "" && href != "#" && !strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return true
		}
	}
	role := strings.ToLower(item.Attributes["role"])
	if role == "link" || role == "tab" || role == "menuitem" {
		return true
	}
	for _, key := range []string{"href", "data-href", "data-url", "data-to", "to"} {
		v := item.Attributes[key]
		if v == "" || v == "#" || strings.HasPrefix(strings.ToLower(v), "javascript:") {
			continue
		}
		return true
	}
	return false
}

func isElementDisabledInteractive(item *types.HTMLElement) bool {
	if item.Attributes == nil {
		return false
	}
	if _, ok := item.Attributes["disabled"]; ok {
		return true
	}
	return item.Attributes["aria-disabled"] == "true"
}

func (e *interactiveExplorer) click(item *types.HTMLElement) error {
	if err := e.ctx.Err(); err != nil {
		return err
	}
	page := e.page.Page.Context(e.ctx).Timeout(e.crawler.options.PageMaxTimeout)
	value, err := page.Eval(`() => {
		const el = document.evaluate(` + strconv.Quote(item.XPath) + `, document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue;
		if (!el) return {error: 'element disappeared'};
		if (el.tagName !== ` + strconv.Quote(item.TagName) + ` || (` + strconv.Quote(item.ID) + ` && el.id !== ` + strconv.Quote(item.ID) + `)) return {error: 'element identity changed'};
		if (el.disabled || el.getAttribute('aria-disabled') === 'true') return {error: 'element is disabled'};
		el.scrollIntoView({block: 'center', inline: 'center', behavior: 'instant'});
		const style = getComputedStyle(el);
		const rect = el.getBoundingClientRect();
		if (style.display === 'none' || style.visibility === 'hidden' || style.pointerEvents === 'none' || !rect.width || !rect.height) return {error: 'element is not visible'};
		const x = rect.left + rect.width / 2, y = rect.top + rect.height / 2;
		const top = document.elementFromPoint(x, y);
		if (!top || !(top === el || el.contains(top))) return {error: 'element is covered'};
		return {x, y};
	}`)
	if err != nil {
		return err
	}
	var point struct {
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		Error string  `json:"error"`
	}
	if err := value.Value.Unmarshal(&point); err != nil {
		return err
	}
	if point.Error != "" {
		return fmt.Errorf("%s", point.Error)
	}
	if err := (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMouseMoved, X: point.X, Y: point.Y}).Call(page); err != nil {
		return err
	}
	if err := (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMousePressed, X: point.X, Y: point.Y, Button: proto.InputMouseButtonLeft, ClickCount: 1}).Call(page); err != nil {
		return err
	}
	return (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMouseReleased, X: point.X, Y: point.Y, Button: proto.InputMouseButtonLeft, ClickCount: 1}).Call(page)
}

func (e *interactiveExplorer) settle(previous string) {
	limit := e.crawler.options.InteractionSettle
	if limit <= 0 {
		return
	}
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) && e.ctx.Err() == nil {
		current, _, err := interactiveState(e.page)
		if err == nil && current != previous {
			// Give newly requested chunks a short chance to finish after DOM change.
			_ = e.page.Timeout(time.Until(deadline)).WaitIdle(200 * time.Millisecond)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// restore returns the page to the parent UI state.
//
// Success rule: state == expected always wins. A URL-only match is accepted
// only when path is empty (shallow-wide URL seeds). Non-empty paths must be
// click-replayed from rootURL; URL equality alone is never enough for those.
// Preference order: already there → NavigateBack (strict) → URL Navigate
// (path empty) → Navigate(rootURL)+path replay (path non-empty). On failure
// the caller abandons remaining siblings instead of looping.
func (e *interactiveExplorer) restore(path []*types.HTMLElement, expected, expectedURL string) error {
	if state, _, err := interactiveState(e.page); err == nil && state == expected {
		return nil
	}

	if _, currentURL, err := interactiveState(e.page); err == nil && currentURL != expectedURL {
		if err := e.page.Timeout(e.crawler.options.PageMaxTimeout).NavigateBack(); err == nil {
			_ = e.page.WaitPageLoadHeurisitics()
			if restored, url, err := interactiveState(e.page); err == nil {
				if restored == expected {
					return nil
				}
				if len(path) == 0 && url == expectedURL {
					return nil
				}
			}
		}
	}

	// Shallow-wide seeds: Navigate to the parent URL is enough.
	if len(path) == 0 {
		target := expectedURL
		if target == "" {
			target = e.rootURL
		}
		if err := e.page.Timeout(e.crawler.options.PageMaxTimeout).Navigate(target); err != nil {
			return err
		}
		if err := e.page.WaitPageLoadHeurisitics(); err != nil {
			return err
		}
		if state, url, err := interactiveState(e.page); err == nil {
			if state == expected || url == expectedURL {
				return nil
			}
		}
		return fmt.Errorf("could not restore interactive URL state")
	}

	// Same-URL UI stacks: replay the click path from root. Do not treat a bare
	// URL match as success — sibling controls live behind the path.
	if err := e.page.Timeout(e.crawler.options.PageMaxTimeout).Navigate(e.rootURL); err != nil {
		return err
	}
	if err := e.page.WaitPageLoadHeurisitics(); err != nil {
		return err
	}
	for _, item := range path {
		if !e.reserveAction() {
			return fmt.Errorf("interaction action budget exhausted during restore")
		}
		before, _, err := interactiveState(e.page)
		if err != nil {
			return err
		}
		if err := e.click(item); err != nil {
			return err
		}
		e.settle(before)
	}
	actual, _, err := interactiveState(e.page)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("restored a different UI state: %s != %s", actual, expected)
	}
	return nil
}

func interactiveState(page *browser.BrowserPage) (string, string, error) {
	// Hash in the browser to avoid repeatedly transferring a potentially huge
	// DOM across CDP while waiting for an interaction to settle.
	value, err := page.Eval(`async () => {
		const url = location.href;
		const dom = document.documentElement ? document.documentElement.outerHTML : '';
		const input = url + '\0' + dom;
		if (crypto.subtle) {
			const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(input));
			return url + '\0' + Array.from(new Uint8Array(digest), b => b.toString(16).padStart(2, '0')).join('');
		}
		let a = 2166136261, b = 1664525;
		for (let i = 0; i < input.length; i++) {
			a = Math.imul(a ^ input.charCodeAt(i), 16777619);
			b = Math.imul(b ^ input.charCodeAt(i), 2246822519);
		}
		return url + '\0' + input.length + ':' + (a >>> 0).toString(16) + ':' + (b >>> 0).toString(16);
	}`)
	if err != nil {
		return "", "", err
	}
	url, hash, ok := strings.Cut(value.Value.Str(), "\x00")
	if !ok {
		return "", "", fmt.Errorf("invalid browser state fingerprint")
	}
	return hash, url, nil
}
