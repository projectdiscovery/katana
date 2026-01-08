package crawler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	graphlib "github.com/dominikbraun/graph"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/diagnostics"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler/normalizer/simhash"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

var emptyPageHash = sha256Hash("")

const simhashThreshold = 2 // Allow up to 2 bits difference

func (c *Crawler) isCorrectNavigation(page *browser.BrowserPage, action *types.Action) (string, *types.PageState, error) {
	currentPageHash, pageState, err := getPageHash(page)
	if err != nil {
		return "", nil, err
	}

	if currentPageHash == action.OriginID {
		return currentPageHash, pageState, nil
	}

	// Get the origin page state to compare SimHash
	originPageState, err := c.crawlGraph.GetPageState(action.OriginID)
	if err != nil {
		return "", pageState, fmt.Errorf("failed to get origin page state: %w", err)
	}

	if pageState != nil && originPageState != nil {
		distance := simhash.Distance(pageState.SimHash, originPageState.SimHash)
		if distance <= simhashThreshold {
			c.logger.Debug("Page is similar enough to origin, proceeding",
				slog.String("current_hash", currentPageHash),
				slog.String("origin_hash", action.OriginID),
				slog.Uint64("simhash_distance", uint64(distance)),
			)
			// Treat this page as the origin state to avoid creating a new vertex
			return originPageState.UniqueID, pageState, nil
		}
	}

	return "", pageState, fmt.Errorf("failed to navigate back to origin page: %s != %s", currentPageHash, action.OriginID)
}

func getPageHash(page *browser.BrowserPage) (string, *types.PageState, error) {
	pageState, err := newPageState(page, nil)
	if err == ErrEmptyPage {
		return emptyPageHash, nil, nil
	}
	if err != nil {
		return "", nil, errors.Wrap(err, "could not get page state")
	}
	return pageState.UniqueID, pageState, nil
}

var ErrEmptyPage = errors.New("page is empty")

func newPageState(page *browser.BrowserPage, action *types.Action) (*types.PageState, error) {
	pageInfo, err := page.Info()
	if err != nil {
		return nil, errors.Wrap(err, "could not get page info")
	}
	if pageInfo.URL == "" || pageInfo.URL == "about:blank" {
		return nil, ErrEmptyPage
	}

	outerHTML, err := page.HTML()
	if err != nil {
		return nil, errors.Wrap(err, "could not get html content")
	}

	state := &types.PageState{
		URL:              pageInfo.URL,
		DOM:              outerHTML,
		NavigationAction: action,
		Title:            pageInfo.Title,
	}
	if action != nil {
		state.Depth = action.Depth + 1
	}
	strippedDOM, err := getStrippedDOM(outerHTML)
	if err != nil {
		return nil, errors.Wrap(err, "could not get stripped dom")
	}
	state.StrippedDOM = strippedDOM

	// Get sha256 hash of the stripped dom
	state.UniqueID = sha256Hash(strippedDOM)
	state.SimHash = simhash.Fingerprint(strings.NewReader(strippedDOM), 3)

	return state, nil
}

func sha256Hash(item string) string {
	hasher := sha256.New()
	hasher.Write([]byte(item))
	hashItem := hex.EncodeToString(hasher.Sum(nil))
	return hashItem
}

func getStrippedDOM(contents string) (string, error) {
	normalized, err := domNormalizer.Apply(contents)
	if err != nil {
		return "", errors.Wrap(err, "could not normalize dom")
	}
	return normalized, nil
}

var ErrNoNavigationPossible = errors.New("no navigation possible")

// navigateBackToStateOrigin implements the logic to navigate back to the state origin
//
// It implements different logics as an optimization to decide
// how to navigate back.
//
//  1. If the action has an element, check if the element is visible on the current page
//     If the element is visible, directly use that to navigate.
//
//  2. If we have browser history, and the page is in the history which was the origin
//     of the action, then we can directly use the browser history to navigate back.
//
// 3. If all else fails, we have the shortest path navigation.
func (c *Crawler) navigateBackToStateOrigin(action *types.Action, page *browser.BrowserPage, currentPageHash string) (string, error) {
	c.logger.Debug("Found action with different origin id",
		slog.String("action_origin_id", action.OriginID),
		slog.String("current_page_hash", currentPageHash),
	)

	// Get vertex from the graph
	originPageState, err := c.crawlGraph.GetPageState(action.OriginID)
	if err != nil {
		c.logger.Debug("Failed to get origin page state", slog.String("error", err.Error()))
		return "", err
	}

	// First, check if the element we want to interact with exists on current page
	if action.Element != nil && currentPageHash != emptyPageHash {
		newPageHash, err := c.tryElementNavigation(page, action, currentPageHash)
		if err != nil {
			c.logger.Debug("Failed to navigate back to origin page using element", slog.String("error", err.Error()))
		}
		if newPageHash != "" {
			return newPageHash, nil
		}
	}

	// Try to see if we can move back using the browser history
	newPageHash, err := c.tryBrowserHistoryNavigation(page, originPageState, action)
	if err != nil {
		c.logger.Debug("Failed to navigate back using browser history", slog.String("error", err.Error()))
	}
	if newPageHash != "" {
		return newPageHash, nil
	}

	// Finally try Shortest path walking from root.
	newPageHash, err = c.tryShortestPathNavigation(action, page, currentPageHash)
	if err != nil {
		return "", err
	}
	if newPageHash == "" {
		return "", ErrNoNavigationPossible
	}
	return newPageHash, nil
}

func (c *Crawler) tryElementNavigation(page *browser.BrowserPage, action *types.Action, currentPageHash string) (string, error) {
	element, err := page.ElementX(action.Element.XPath)
	if err != nil {
		return "", err
	}
	visible, err := element.Visible()
	if err != nil {
		return "", err
	}
	if !visible {
		return "", nil
	}

	// Also ensure its interactable
	interactable, err := element.Interactable()
	if err != nil || interactable == nil {
		return "", nil
	}

	// Ensure its the same element
	htmlElement, err := page.GetElementFromXpath(action.Element.XPath)
	if err != nil {
		return "", err
	}
	// Ensure its the same element with stronger identity matching
	if isElementMatch(htmlElement, action.Element) {
		c.logger.Debug("Found target element on current page, proceeding without navigation")
		// FIXME: Return the origin element ID so that the graph shows
		// correctly the fastest way to reach the state.
		return action.OriginID, nil
	}
	return "", nil
}

// isElementMatch implements stronger identity matching logic to reduce false positives.
// It treats identical ID as definitive match, otherwise requires both Classes and TextContent
// to match, or enforces at least two matching non-empty attributes.
func isElementMatch(current, target *types.HTMLElement) bool {
	if current == nil || target == nil {
		return false
	}
	// Definitive match: identical non-empty IDs
	if current.ID != "" && target.ID != "" && current.ID == target.ID {
		return true
	}
	matchCount := 0

	if current.Classes != "" && target.Classes != "" && current.Classes == target.Classes {
		matchCount++
	}
	if current.TextContent != "" && target.TextContent != "" && current.TextContent == target.TextContent {
		matchCount++
	}
	if current.TagName != "" && target.TagName != "" && current.TagName == target.TagName {
		matchCount++
	}
	// Require at least two matching non-empty attributes for a positive match
	// This ensures stronger identity verification while still allowing reasonable fallbacks
	return matchCount >= 2
}

func (c *Crawler) tryBrowserHistoryNavigation(page *browser.BrowserPage, originPageState *types.PageState, action *types.Action) (string, error) {
	canNavigateBack, stepsBack, err := c.isBackNavigationPossible(page, originPageState)
	if err != nil {
		return "", err
	}
	if !canNavigateBack {
		return "", nil
	}

	c.logger.Debug("Navigating back using browser history", slog.Int("steps_back", stepsBack))

	var navigatedSuccessfully bool
	for i := 0; i < stepsBack; i++ {
		if err := page.NavigateBack(); err != nil {
			return "", err
		}
		navigatedSuccessfully = true
	}

	if !navigatedSuccessfully {
		return "", nil
	}

	if err := page.WaitPageLoadHeurisitics(); err != nil {
		c.logger.Debug("Failed to wait for page load after navigating back using browser history", slog.String("error", err.Error()))
	}
	newPageHash, pageState, err := c.isCorrectNavigation(page, action)
	if c.diagnostics != nil && pageState != nil {
		if err := c.diagnostics.LogPageState(pageState, diagnostics.PreActionPageState); err != nil {
			return "", err
		}
	}
	if err != nil {
		return "", err
	}
	return newPageHash, nil
}

func (c *Crawler) isBackNavigationPossible(page *browser.BrowserPage, originPage *types.PageState) (bool, int, error) {
	history, err := page.GetNavigationHistory()
	if err != nil {
		return false, 0, err
	}
	if len(history.Entries) == 0 {
		return false, 0, nil
	}

	currentIndex := history.CurrentIndex
	for i, entry := range history.Entries {
		if entry.URL == originPage.URL && originPage.Title == entry.Title {
			stepsBack := currentIndex - i
			return true, stepsBack, nil
		}
	}
	return false, 0, nil
}

func (c *Crawler) tryShortestPathNavigation(action *types.Action, page *browser.BrowserPage, currentPageHash string) (string, error) {
	c.logger.Debug("Trying Shortest path to navigate back to origin page", slog.String("action_origin_id", action.OriginID), slog.String("current_page_hash", currentPageHash))

	actions, err := c.crawlGraph.ShortestPath(currentPageHash, action.OriginID)
	if err != nil {
		if errors.Is(err, graphlib.ErrTargetNotReachable) {
			c.logger.Debug("Target not reachable, reaching from blank state",
				slog.String("action_origin_id", action.OriginID),
			)

			actions, err = c.crawlGraph.ShortestPath(emptyPageHash, action.OriginID)
			if err != nil {
				return "", errors.Wrap(err, "could not find path to origin page")
			}
		} else {
			return "", errors.Wrap(err, "failed to find shortest path")
		}
	}
	c.logger.Debug("Found actions to traverse",
		slog.Any("actions", actions),
	)
	for _, action := range actions {
		if err := c.executeCrawlStateAction(action, page); err != nil {
			return "", err
		}
	}
	newPageHash, pageState, err := c.isCorrectNavigation(page, action)
	if c.diagnostics != nil && pageState != nil {
		if err := c.diagnostics.LogPageState(pageState, diagnostics.PreActionPageState); err != nil {
			return "", err
		}
	}
	if err != nil {
		return "", err
	}
	return newPageHash, nil
}
