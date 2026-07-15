package runner

import (
	"os"
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

func TestValidateRecordedFlow(t *testing.T) {
	writeFlow := func(t *testing.T, contents string) string {
		t.Helper()
		path := t.TempDir() + "/flow.json"
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
		return path
	}

	t.Run("missing file is rejected", func(t *testing.T) {
		opts := newTestOptions()
		opts.RecordedFlow = "/no/such/recorded-flow.json"
		err := validateOptions(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "does not exist")
	})

	t.Run("forces headless and accepts chrome recording with credentials", func(t *testing.T) {
		flow := `{
		  "title": "login",
		  "steps": [
		    {"type": "navigate", "url": "https://example.com/login"},
		    {"type": "change", "value": "dave", "selectors": [["#email"]]},
		    {"type": "change", "value": "secret", "selectors": [["#password"]]},
		    {"type": "click", "selectors": [["#submit"]]}
		  ]
		}`
		opts := newTestOptions()
		opts.RecordedFlow = writeFlow(t, flow)
		opts.AuthCredentials = "dave:secret"
		err := validateOptions(opts)
		require.NoError(t, err)
		require.True(t, opts.Headless, "recorded flow should enable headless")
	})

	t.Run("requires credentials when placeholders remain", func(t *testing.T) {
		flow := `{
		  "steps": [
		    {"type": "change", "value": "x", "selectors": [["#password"]]}
		  ]
		}`
		opts := newTestOptions()
		opts.RecordedFlow = writeFlow(t, flow)
		err := validateOptions(opts)
		require.Error(t, err)
		require.Contains(t, err.Error(), "auto-login")
	})

	t.Run("disables hybrid when recorded flow is set", func(t *testing.T) {
		flow := `{"steps":[{"action":"navigate","value":"https://example.com"}]}`
		opts := newTestOptions()
		opts.HeadlessHybrid = true
		opts.RecordedFlow = writeFlow(t, flow)
		err := validateOptions(opts)
		require.NoError(t, err)
		require.False(t, opts.HeadlessHybrid)
		require.True(t, opts.Headless)
	})
}
