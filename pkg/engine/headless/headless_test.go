package headless

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
	"github.com/stretchr/testify/require"
)

func newTestHeadless(t *testing.T, ignoreQueryParams, filterSimilar bool) *Headless {
	t.Helper()

	filter, err := filters.NewSimple()
	require.NoError(t, err)
	t.Cleanup(filter.Close)

	opts := &types.Options{
		IgnoreQueryParams: ignoreQueryParams,
		FilterSimilar:     filterSimilar,
	}
	crawlerOpts := &types.CrawlerOptions{
		Options:      opts,
		UniqueFilter: filter,
	}
	h := &Headless{options: crawlerOpts}
	if filterSimilar {
		h.pathTrie = utils.NewPathTrie(opts.FilterSimilarThreshold)
	}
	return h
}

func TestIsUniqueURL(t *testing.T) {
	t.Run("first_call_returns_true", func(t *testing.T) {
		h := newTestHeadless(t, false, false)
		require.True(t, h.isUniqueURL("https://example.com/a"))
	})

	t.Run("second_call_same_url_returns_false", func(t *testing.T) {
		h := newTestHeadless(t, false, false)
		require.True(t, h.isUniqueURL("https://example.com/a"))
		require.False(t, h.isUniqueURL("https://example.com/a"))
	})

	t.Run("different_urls_both_unique", func(t *testing.T) {
		h := newTestHeadless(t, false, false)
		require.True(t, h.isUniqueURL("https://example.com/a"))
		require.True(t, h.isUniqueURL("https://example.com/b"))
	})

	t.Run("ignore_query_params", func(t *testing.T) {
		h := newTestHeadless(t, true, false)
		require.True(t, h.isUniqueURL("https://example.com/page?id=1"))
		require.False(t, h.isUniqueURL("https://example.com/page?id=2"),
			"same path and key with different value should be duplicate when IgnoreQueryParams is set")
	})

	t.Run("query_params_matter_when_not_ignored", func(t *testing.T) {
		h := newTestHeadless(t, false, false)
		require.True(t, h.isUniqueURL("https://example.com/page?id=1"))
		require.True(t, h.isUniqueURL("https://example.com/page?id=2"),
			"different query param values should be unique when IgnoreQueryParams is off")
	})
}

func TestRealRequestTakesPriority(t *testing.T) {
	h := newTestHeadless(t, false, false)

	targetURL := "https://example.com/target"

	// Simulate the reordered RequestCallback: real request registers first.
	isUnique := h.isUniqueURL(targetURL)
	require.True(t, isUnique, "real request should register as unique")

	// A later performAdditionalAnalysis call that discovers the same URL
	// should see it as already registered.
	require.False(t, h.isUniqueURL(targetURL),
		"synthetic discovery of same URL should be filtered after real request registered it")
}

func makeResponse(t *testing.T, pageURL, html string) *navigation.Response {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	parsed, err := url.Parse(pageURL)
	require.NoError(t, err)
	return &navigation.Response{
		Body:   html,
		Reader: doc,
		Resp:   &http.Response{Request: &http.Request{URL: parsed}},
	}
}

func TestPerformAdditionalAnalysisDedups(t *testing.T) {
	h := newTestHeadless(t, false, false)

	html := `<html><body>
		<a href="https://example.com/link1">Link 1</a>
		<a href="https://example.com/link2">Link 2</a>
		<a href="https://example.com/link1">Link 1 again</a>
		<a href="https://example.com/page">Self</a>
	</body></html>`

	pageURL := "https://example.com/page"
	rr := &output.Result{
		Request:  &navigation.Request{URL: pageURL},
		Response: makeResponse(t, pageURL, html),
	}

	// Register the real request URL first (matching the reordered callback flow).
	h.isUniqueURL(rr.Request.URL)

	results := h.performAdditionalAnalysis(rr)

	// Count occurrences to verify dedup actually works (a map would hide duplicates).
	counts := make(map[string]int)
	for _, r := range results {
		counts[r.Request.URL]++
	}
	require.Equal(t, 1, counts["https://example.com/link1"], "link1 should appear exactly once")
	require.Equal(t, 1, counts["https://example.com/link2"], "link2 should appear exactly once")
	require.Equal(t, 0, counts["https://example.com/page"],
		"self-referencing page URL should be filtered since the real request already registered it")
	require.Len(t, results, 2, "only the two unique non-page links should be returned")
}

func TestAdditionalAnalysisStillRunsForDuplicateRequests(t *testing.T) {
	h := newTestHeadless(t, false, false)

	// Simulate page A discovering URL B via additional analysis.
	h.isUniqueURL("https://example.com/a")
	h.isUniqueURL("https://example.com/b") // as if discovered from A's analysis

	// Now the browser visits B. B is a duplicate, but its response body
	// contains a new URL C that we should still discover.
	html := `<html><body>
		<a href="https://example.com/c">New link</a>
	</body></html>`

	bURL := "https://example.com/b"
	rr := &output.Result{
		Request:  &navigation.Request{URL: bURL},
		Response: makeResponse(t, bURL, html),
	}

	// In the reordered callback, B is checked first (returns false = duplicate),
	// but additional analysis still runs.
	isUnique := h.isUniqueURL(rr.Request.URL)
	require.False(t, isUnique, "B should be a duplicate")

	results := h.performAdditionalAnalysis(rr)

	var foundC bool
	for _, r := range results {
		if r.Request.URL == "https://example.com/c" {
			foundC = true
		}
	}
	require.True(t, foundC, "new URL C should be discovered even though B was a duplicate request")
}
