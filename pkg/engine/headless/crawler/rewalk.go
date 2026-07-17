package crawler

import (
	"context"
	"log/slog"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

type pageSession struct {
	bp *browser.BrowserPage
}

func (s *pageSession) Clear(ctx context.Context) error {
	if s == nil || s.bp == nil {
		return nil
	}
	return browser.ClearSession(ctx, s.bp.Page)
}

type pathObserver struct {
	c    *Crawler
	page *browser.BrowserPage
}

func (o *pathObserver) Rewalk(ctx context.Context, path *types.Path) (cartography.LocationFingerprint, error) {
	if path == nil {
		return cartography.LocationFingerprint{}, nil
	}
	o.page.Page = o.page.Context(ctx)
	for _, step := range path.Steps {
		if step == nil {
			continue
		}
		if err := o.c.executeCrawlStateAction(step, o.page); err != nil {
			return cartography.LocationFingerprint{}, err
		}
	}
	_, state, err := getPageHash(o.page)
	if err != nil {
		return cartography.LocationFingerprint{}, err
	}
	return cartography.LocationFingerprint{
		UniqueID: state.UniqueID,
		URL:      state.URL,
		SimHash:  state.SimHash,
	}, nil
}

func samplePaths(paths []*types.Path, n int) []*types.Path {
	if n <= 0 || len(paths) == 0 {
		return nil
	}
	if n >= len(paths) {
		out := make([]*types.Path, len(paths))
		copy(out, paths)
		return out
	}
	out := make([]*types.Path, n)
	copy(out, paths[:n])
	return out
}

// maybeRewalk clears the session and replays a sample of discovered paths to
// classify match / app_change / session_fork / non_deterministic.
func (c *Crawler) maybeRewalk(ctx context.Context) {
	if c == nil || c.options.RewalkSample <= 0 || c.crawlGraph == nil || c.launcher == nil {
		return
	}
	paths, err := c.Paths()
	if err != nil || len(paths) == 0 {
		return
	}
	sample := samplePaths(paths, c.options.RewalkSample)
	page, err := c.launcher.GetPageFromPool()
	if err != nil {
		c.logger.Debug("rewalk skipped: no page", slog.String("error", err.Error()))
		return
	}
	defer c.launcher.PutBrowserToPool(page)

	runner := &cartography.Runner{
		Session:    &pageSession{bp: page},
		Observer:   &pathObserver{c: c, page: page},
		MaxHamming: cartography.DefaultMaxHamming,
	}

	rewalkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	for _, p := range sample {
		ps, err := c.crawlGraph.GetPageState(p.TargetID)
		if err != nil {
			continue
		}
		expected := cartography.LocationFingerprint{
			UniqueID: ps.UniqueID,
			URL:      ps.URL,
			SimHash:  ps.SimHash,
		}
		outcome, _, err := runner.Run(rewalkCtx, expected, []*types.Path{p})
		if err != nil {
			c.logger.Debug("rewalk failed",
				slog.String("target", p.TargetURL),
				slog.String("error", err.Error()),
			)
			continue
		}
		c.logger.Info("rewalk complete",
			slog.String("target", p.TargetURL),
			slog.String("outcome", string(outcome)),
		)
	}
}
