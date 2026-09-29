package hybrid

import (
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/types"
)

const maxHybridBrowserAgents = 8

// hybridBrowserAgents returns how many isolated Chrome instances to run.
// Concurrent tabs on one browser are unsafe under CDP; parallel agents each
// own a browser. Shared profile / remote debugger modes stay single-agent.
func hybridBrowserAgents(opts *types.Options) int {
	if opts == nil {
		return 1
	}
	if opts.ChromeWSUrl != "" || opts.ChromeDataDir != "" {
		return 1
	}
	n := opts.Concurrency
	if n < 1 {
		return 1
	}
	if n > maxHybridBrowserAgents {
		return maxHybridBrowserAgents
	}
	return n
}

// shouldInspectFetchResource reports whether a paused Fetch response deserves
// body retrieval and parsing. Non-interesting resources should be continued
// immediately so they do not stall page load on the CDP critical path.
func shouldInspectFetchResource(resourceType proto.NetworkResourceType, opts *types.Options) bool {
	if opts == nil {
		return resourceType == proto.NetworkResourceTypeDocument
	}

	switch resourceType {
	case proto.NetworkResourceTypeDocument:
		return true
	case proto.NetworkResourceTypeScript:
		return opts.ScrapeJSResponses || opts.ScrapeJSLuiceResponses || opts.XhrExtraction
	case proto.NetworkResourceTypeStylesheet:
		return opts.ScrapeJSResponses
	case proto.NetworkResourceTypeXHR, proto.NetworkResourceTypeFetch:
		// Only pause XHR/Fetch when explicitly requested; SPA polling otherwise
		// serializes every background call through the hijack handler.
		return opts.XhrExtraction
	default:
		return false
	}
}

// fetchRequestPatterns returns CDP Fetch patterns limited to resource types we
// actually inspect. Narrow patterns avoid pausing images/fonts/media at all.
func fetchRequestPatterns(opts *types.Options) []*proto.FetchRequestPattern {
	patterns := []*proto.FetchRequestPattern{
		{
			URLPattern:   "*",
			ResourceType: proto.NetworkResourceTypeDocument,
			RequestStage: proto.FetchRequestStageResponse,
		},
	}
	if opts == nil {
		return patterns
	}
	if opts.ScrapeJSResponses || opts.ScrapeJSLuiceResponses || opts.XhrExtraction {
		patterns = append(patterns, &proto.FetchRequestPattern{
			URLPattern:   "*",
			ResourceType: proto.NetworkResourceTypeScript,
			RequestStage: proto.FetchRequestStageResponse,
		})
	}
	if opts.ScrapeJSResponses {
		patterns = append(patterns, &proto.FetchRequestPattern{
			URLPattern:   "*",
			ResourceType: proto.NetworkResourceTypeStylesheet,
			RequestStage: proto.FetchRequestStageResponse,
		})
	}
	if opts.XhrExtraction {
		patterns = append(patterns,
			&proto.FetchRequestPattern{
				URLPattern:   "*",
				ResourceType: proto.NetworkResourceTypeXHR,
				RequestStage: proto.FetchRequestStageResponse,
			},
			&proto.FetchRequestPattern{
				URLPattern:   "*",
				ResourceType: proto.NetworkResourceTypeFetch,
				RequestStage: proto.FetchRequestStageResponse,
			},
		)
	}
	// Redirect blocking needs Fetch for every navigation response we care about;
	// Document is already covered. Keep patterns unchanged otherwise.
	_ = opts.DisableRedirects
	return patterns
}
