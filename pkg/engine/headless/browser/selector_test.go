package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetCSSPathEscapesElementIDs(t *testing.T) {
	skipIfNoBrowser(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body>
			<div id="6a3cdb1e-55fd-4980-8dde-fa99b8cb6c93"></div>
			<div id="menu:item"></div>
			<div id="space id"></div>
		</body></html>`)
	}))
	defer server.Close()

	launcher, err := NewLauncher(LauncherOptions{MaxBrowsers: 1, NoSandbox: true})
	require.NoError(t, err)
	defer launcher.Close()

	page, err := launcher.GetPageFromPool()
	require.NoError(t, err)
	require.NoError(t, page.Navigate(server.URL))
	require.NoError(t, page.WaitLoad())

	value, err := page.Eval(`() => {
		const ids = [
			'6a3cdb1e-55fd-4980-8dde-fa99b8cb6c93',
			'menu:item',
			'space id',
		];
		const matches = (selector, target) => {
			try { return document.querySelector(selector) === target; }
			catch (_) { return false; }
		};
		return ids.map((id) => {
			const target = document.getElementById(id);
			const selector = window.getCssPath(target);
			const optimizedSelector = window.getCssPath(target, true);
			return {
				id,
				selector,
				optimizedSelector,
				matches: matches(selector, target),
				optimizedMatches: matches(optimizedSelector, target),
			};
		});
	}`)
	require.NoError(t, err)

	var results []struct {
		ID                string `json:"id"`
		Selector          string `json:"selector"`
		OptimizedSelector string `json:"optimizedSelector"`
		Matches           bool   `json:"matches"`
		OptimizedMatches  bool   `json:"optimizedMatches"`
	}
	require.NoError(t, value.Value.Unmarshal(&results))
	require.Len(t, results, 3)
	for _, result := range results {
		require.NotEmpty(t, result.Selector, result.ID)
		require.NotEmpty(t, result.OptimizedSelector, result.ID)
		require.True(t, result.Matches, result.ID)
		require.True(t, result.OptimizedMatches, result.ID)
	}

	launcher.PutBrowserToPool(page)
}
