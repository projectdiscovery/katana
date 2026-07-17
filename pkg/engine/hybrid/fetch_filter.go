package hybrid

import (
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/types"
)

// shouldInspectFetchResource reports whether a paused Fetch response deserves
// body retrieval and parsing. Non-interesting resources (images, fonts, media,
// etc.) should be continued immediately so they do not stall page load on the
// CDP critical path.
func shouldInspectFetchResource(resourceType proto.NetworkResourceType, opts *types.Options) bool {
	if opts == nil {
		return resourceType == proto.NetworkResourceTypeDocument
	}

	switch resourceType {
	case proto.NetworkResourceTypeDocument:
		return true
	case proto.NetworkResourceTypeScript:
		// Scripts are inspected when JS endpoint scraping or XHR capture is on.
		return opts.ScrapeJSResponses || opts.ScrapeJSLuiceResponses || opts.XhrExtraction
	case proto.NetworkResourceTypeStylesheet:
		// CSS relative-endpoint extraction shares the JS-crawl flag.
		return opts.ScrapeJSResponses
	case proto.NetworkResourceTypeXHR, proto.NetworkResourceTypeFetch:
		// Keep inspecting XHR/Fetch so JSON/API response bodies can still yield
		// endpoints; -xhr only controls whether the request itself is recorded.
		return true
	default:
		return false
	}
}
