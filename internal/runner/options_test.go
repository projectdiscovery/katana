package runner

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func newTestOptions() *types.Options {
	return &types.Options{
		MaxDepth: 2,
		URLs:     []string{"https://example.com"},
	}
}

func TestValidatePageLoadStrategy(t *testing.T) {
	t.Run("valid strategies are accepted", func(t *testing.T) {
		for _, strategy := range []string{"heuristic", "load", "domcontentloaded", "networkidle", "none"} {
			opts := newTestOptions()
			opts.PageLoadStrategy = strategy
			err := validateOptions(opts)
			require.NoError(t, err, "strategy %q should be valid", strategy)
			require.Equal(t, strategy, opts.PageLoadStrategy)
		}
	})

	t.Run("empty strategy defaults to heuristic", func(t *testing.T) {
		opts := newTestOptions()
		opts.PageLoadStrategy = ""
		err := validateOptions(opts)
		require.NoError(t, err)
		require.Equal(t, "heuristic", opts.PageLoadStrategy)
	})

	t.Run("invalid strategy is rejected", func(t *testing.T) {
		opts := newTestOptions()
		opts.PageLoadStrategy = "invalid"
		err := validateOptions(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid page-load-strategy")
	})
}

func TestValidateCrawlStrategy(t *testing.T) {
	t.Run("requires headless mode", func(t *testing.T) {
		opts := newTestOptions()
		opts.CrawlStrategy = "fast"
		err := validateOptions(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "-crawl-strategy requires")
	})

	t.Run("applies preset when headless", func(t *testing.T) {
		opts := newTestOptions()
		opts.Headless = true
		opts.CrawlStrategy = "fast"
		opts.MaxDepth = 99
		err := validateOptions(opts)
		require.NoError(t, err)
		require.Equal(t, 2, opts.MaxDepth)
		require.Equal(t, "domcontentloaded", opts.PageLoadStrategy)
		require.Equal(t, 0, opts.DOMWaitTime)
	})
}

func TestValidateHeadlessFlags(t *testing.T) {
	t.Run("headless and hybrid are mutually exclusive", func(t *testing.T) {
		opts := newTestOptions()
		opts.Headless = true
		opts.HeadlessHybrid = true
		err := validateOptions(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("no-sandbox without headless mode fails", func(t *testing.T) {
		opts := newTestOptions()
		opts.HeadlessNoSandbox = true
		err := validateOptions(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "headless")
	})

	t.Run("no-sandbox with headless mode succeeds", func(t *testing.T) {
		opts := newTestOptions()
		opts.Headless = true
		opts.HeadlessNoSandbox = true
		err := validateOptions(opts)
		require.NoError(t, err)
	})
}
