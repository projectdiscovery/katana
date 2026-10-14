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
)

type interactiveExplorer struct {
	crawler  *Crawler
	page     *browser.BrowserPage
	ctx      context.Context
	rootURL  string
	visited  map[string]struct{}
	attempts int
	success  int
	failures int
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
	e := &interactiveExplorer{crawler: c, page: page, ctx: ctx, rootURL: rootURL, visited: make(map[string]struct{})}
	err = e.explore(0, nil)
	c.logger.Info("interactive crawl finished", slog.Int("states", len(e.visited)), slog.Int("attempts", e.attempts), slog.Int("successful_clicks", e.success), slog.Int("failed_clicks", e.failures))
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
		if limit := e.crawler.options.InteractionMaxActions; limit > 0 && e.attempts >= limit {
			return nil
		}
		// Recreate the parent only after one of our clicks changed it. Pages
		// with rotating ads or clocks can mutate between two observations.
		if needsRestore {
			if err := e.restore(path, stateID, pageURL); err != nil {
				e.crawler.logger.Debug("could not restore interactive state", slog.String("error", err.Error()))
				e.failures++
				return nil
			}
			needsRestore = false
		}
		e.attempts++
		beforeTabs, _ := e.page.Browser.Pages()
		if err := e.click(candidate); err != nil {
			e.failures++
			e.crawler.logger.Debug("interactive click failed", slog.String("xpath", candidate.XPath), slog.String("error", err.Error()))
			continue
		}
		e.success++
		e.settle(stateID)
		if err := e.explorePopup(beforeTabs, depth+1); err != nil {
			return err
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
		if newID != stateID {
			if err := e.explore(depth+1, appendPath(path, candidate)); err != nil {
				return err
			}
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
		return err
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
			return err
		}
	}
	return nil
}

func (e *interactiveExplorer) exploreOpenedPage(page *rod.Page, depth int) error {
	defer page.Close()
	var popupURL string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && e.ctx.Err() == nil {
		info, err := page.Info()
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
	popup, err := e.crawler.launcher.AdoptPopup(page.Context(e.ctx), e.page.Browser)
	if err != nil {
		return err
	}
	// The target may have loaded its scripts before interception was attached.
	// Reload it once after installing hooks to capture the complete responses.
	if err := popup.Timeout(e.crawler.options.PageMaxTimeout).Navigate(popupURL); err != nil {
		return err
	}
	if err := popup.WaitPageLoadHeurisitics(); err != nil {
		return err
	}
	child := &interactiveExplorer{
		crawler: e.crawler, page: popup, ctx: e.ctx, rootURL: popupURL,
		visited: e.visited, attempts: e.attempts, success: e.success, failures: e.failures,
	}
	err = child.explore(depth, nil)
	e.attempts, e.success, e.failures = child.attempts, child.success, child.failures
	return err
}

func appendPath(path []*types.HTMLElement, element *types.HTMLElement) []*types.HTMLElement {
	result := make([]*types.HTMLElement, len(path)+1)
	copy(result, path)
	result[len(path)] = element
	return result
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
			target, err := url.Parse(item.Attributes["href"])
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
	return result, nil
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

func (e *interactiveExplorer) restore(path []*types.HTMLElement, expected, expectedURL string) error {
	if _, currentURL, err := interactiveState(e.page); err == nil && currentURL != expectedURL {
		if err := e.page.Timeout(e.crawler.options.PageMaxTimeout).NavigateBack(); err == nil {
			_ = e.page.WaitPageLoadHeurisitics()
			if state, _, err := interactiveState(e.page); err == nil && state == expected {
				return nil
			}
		}
	}
	if err := e.page.Timeout(e.crawler.options.PageMaxTimeout).Navigate(e.rootURL); err != nil {
		return err
	}
	if err := e.page.WaitPageLoadHeurisitics(); err != nil {
		return err
	}
	for _, item := range path {
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
