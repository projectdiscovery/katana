package hybrid

import "github.com/projectdiscovery/katana/pkg/types"

const maxHybridBrowserAgents = 8

func hybridBrowserAgents(opts *types.Options) int {
	if opts == nil || opts.ChromeWSUrl != "" || opts.ChromeDataDir != "" {
		return 1
	}
	if opts.BrowserAgents < 1 {
		return 1
	}
	if opts.BrowserAgents > maxHybridBrowserAgents {
		return maxHybridBrowserAgents
	}
	return opts.BrowserAgents
}
