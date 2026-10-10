//go:build !(386 || windows)

package headless

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/require"
)

func TestPerformAdditionalAnalysisUsesConfiguredJSLuiceParser(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			h := newTestHeadless(t, false, false)
			h.options.Parser = parser.NewResponseParser()
			h.options.Parser.InitWithOptions(&parser.Options{ScrapeJSLuiceResponses: enabled})
			response := makeResponse(t, "https://example.com/page",
				`<html><script>fetch("/api/from-jsluice");</script></html>`)
			results := h.performAdditionalAnalysis(&output.Result{Response: response})
			var found bool
			for _, result := range results {
				if result.Request.URL == "https://example.com/api/from-jsluice" {
					found = true
				}
			}
			require.Equal(t, enabled, found)
		})
	}
}
