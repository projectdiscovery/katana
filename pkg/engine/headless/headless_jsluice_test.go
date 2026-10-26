//go:build !(386 || windows)

package headless

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/require"
)

func TestPerformAdditionalAnalysisHonorsJsluice(t *testing.T) {
	h := newTestHeadless(t, false, false)
	p := parser.NewResponseParser()
	p.InitWithOptions(&parser.Options{ScrapeJSLuiceResponses: true})
	h.options.Parser = p

	pageURL := "https://example.com/page"
	html := `<html><body><script>fetch("/api/jsluice_only");</script></body></html>`
	results := h.performAdditionalAnalysis(&output.Result{
		Request:  &navigation.Request{URL: pageURL},
		Response: makeResponse(t, pageURL, html),
	})

	found := false
	for _, r := range results {
		if r.Request != nil && r.Request.URL == "https://example.com/api/jsluice_only" {
			found = true
			break
		}
	}
	require.True(t, found, "-jsl inline script endpoint not extracted, got %v", results)
}
