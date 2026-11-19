package crawler

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// rewalkRepeats is how many clean-session replays are performed per sampled
// path so the classifier can distinguish a consistent app change from a
// non-deterministic (flaky) location.
const rewalkRepeats = 2

// rewalkTimeout bounds all rewalks of one crawl.
const rewalkTimeout = 2 * time.Minute

// RewalkResult is the outcome of the clean-session rewalks of one path.
type RewalkResult struct {
	TargetID     string              `json:"target_id"`
	TargetURL    string              `json:"target_url"`
	Outcome      cartography.Outcome `json:"outcome"`
	ObservedURLs []string            `json:"observed_urls,omitempty"`
	Errors       []string            `json:"errors,omitempty"`
}

// RewalkResults returns the outcomes of the post-crawl rewalks, if any ran.
func (c *Crawler) RewalkResults() []RewalkResult {
	if c == nil {
		return nil
	}
	return c.rewalkResults
}

// rewalkPath replays path on page and returns the location it reached.
func (c *Crawler) rewalkPath(ctx context.Context, page *browser.BrowserPage, path *types.Path) (cartography.LocationFingerprint, error) {
	if path == nil {
		return cartography.LocationFingerprint{}, nil
	}
	original := page.Page
	defer func() { page.Page = original }()
	page.Page = original.Context(ctx)

	for _, step := range path.Steps {
		if step == nil {
			continue
		}
		if err := c.executeCrawlStateAction(step, page); err != nil {
			return cartography.LocationFingerprint{}, err
		}
	}
	hash, state, err := getPageHash(page)
	if err != nil {
		return cartography.LocationFingerprint{}, err
	}
	// An empty/about:blank landing yields a nil state with a sentinel hash;
	// surface it as an empty-location fingerprint instead of dereferencing nil.
	if state == nil {
		return cartography.LocationFingerprint{UniqueID: hash}, nil
	}
	return cartography.LocationFingerprint{
		UniqueID: state.UniqueID,
		URL:      state.URL,
		SimHash:  state.SimHash,
	}, nil
}

// samplePaths returns n paths picked at random, or all of them when there are
// not more than n.
func samplePaths(paths []*types.Path, n int) []*types.Path {
	if n <= 0 || len(paths) == 0 {
		return nil
	}
	out := make([]*types.Path, len(paths))
	copy(out, paths)
	if n >= len(out) {
		return out
	}
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out[:n]
}

// crawledOrigins returns the origins of every crawled state, sorted. Only these
// are cleared between rewalks.
func (c *Crawler) crawledOrigins() []string {
	seen := map[string]struct{}{}
	for _, id := range c.crawlGraph.GetVertices() {
		state, err := c.crawlGraph.GetPageState(id)
		if err != nil {
			continue
		}
		u, err := url.Parse(state.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			continue
		}
		seen[u.Scheme+"://"+u.Host] = struct{}{}
	}
	origins := make([]string, 0, len(seen))
	for o := range seen {
		origins = append(origins, o)
	}
	sort.Strings(origins)
	return origins
}

// warnProfileClear warns that the rewalk deletes the cookies and storage of
// origins in a browser profile that outlives the crawl (-cdd, -cwu).
func (c *Crawler) warnProfileClear(origins []string) {
	if c.logger == nil || len(origins) == 0 || (c.options.UserDataDir == "" && c.options.ChromeWSUrl == "") {
		return
	}
	c.logger.Warn("rewalk clears cookies and storage of the crawled origins in the configured browser profile",
		slog.Any("origins", origins),
	)
}

// maybeRewalk clears the session and replays a sample of discovered paths to
// classify match / app_change / session_fork / non_deterministic. It runs on
// the crawl context, so -ct and Ctrl+C stop it too.
func (c *Crawler) maybeRewalk(ctx context.Context) {
	if c == nil || c.options.RewalkSample <= 0 || c.crawlGraph == nil || c.launcher == nil || ctx.Err() != nil {
		return
	}
	paths, err := c.Paths()
	if err != nil || len(paths) == 0 {
		return
	}
	sample := samplePaths(paths, c.options.RewalkSample)
	page, err := c.launcher.GetPageFromPool()
	if err != nil {
		c.logger.Warn("rewalk skipped: no page", slog.String("error", err.Error()))
		return
	}
	defer c.launcher.PutBrowserToPool(page)

	origins := c.crawledOrigins()
	c.warnProfileClear(origins)
	runner := &cartography.Runner{
		Clear: func(ctx context.Context) error {
			return browser.ClearSession(ctx, page.Page, origins)
		},
		Rewalk: func(ctx context.Context, path *types.Path) (cartography.LocationFingerprint, error) {
			return c.rewalkPath(ctx, page, path)
		},
		MaxHamming: cartography.DefaultMaxHamming,
	}

	rewalkCtx, cancel := context.WithTimeout(ctx, rewalkTimeout)
	defer cancel()

	for _, p := range sample {
		if rewalkCtx.Err() != nil {
			break
		}
		ps, err := c.crawlGraph.GetPageState(p.TargetID)
		if err != nil {
			continue
		}
		expected := cartography.LocationFingerprint{
			UniqueID: ps.UniqueID,
			URL:      ps.URL,
			SimHash:  ps.SimHash,
		}
		// Replay the same path more than once so the classifier sees multiple
		// observations; a single one can only ever yield match/app_change and
		// never surfaces a non-deterministic (flaky) location.
		observations := make([]*types.Path, rewalkRepeats)
		for i := range observations {
			observations[i] = p
		}
		outcome, reports, err := runner.Run(rewalkCtx, expected, observations)
		result := RewalkResult{TargetID: p.TargetID, TargetURL: p.TargetURL, Outcome: outcome}
		for _, rep := range reports {
			if rep.Err != nil {
				result.Errors = append(result.Errors, rep.Err.Error())
				continue
			}
			result.ObservedURLs = append(result.ObservedURLs, rep.Observed.URL)
		}
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
		c.rewalkResults = append(c.rewalkResults, result)
		c.logger.Info("rewalk complete",
			slog.String("target", p.TargetURL),
			slog.String("outcome", string(outcome)),
			slog.Any("observed", result.ObservedURLs),
			slog.Any("errors", result.Errors),
		)
	}
}

// writeRewalkExport writes the rewalk results to rewalk.json in the
// diagnostics directory.
func (c *Crawler) writeRewalkExport() error {
	if len(c.rewalkResults) == 0 || c.options.DiagnosticsDir == "" {
		return nil
	}
	data, err := json.MarshalIndent(c.rewalkResults, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.options.DiagnosticsDir, "rewalk.json"), data, 0644)
}
