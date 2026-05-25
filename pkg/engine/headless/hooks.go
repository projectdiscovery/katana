package headless

import "github.com/projectdiscovery/katana/pkg/engine/headless/crawler"

// Hooks re-exports crawler.Hooks so library users can configure headless
// lifecycle callbacks without importing the internal crawler sub-package.
// See crawler.Hooks for field-level documentation and semantics.
type Hooks = crawler.Hooks

// SetHooks installs lifecycle callbacks on the headless engine. The callbacks
// are picked up by subsequent calls to Crawl. Passing nil clears any previously
// installed hooks.
func (h *Headless) SetHooks(hooks *Hooks) {
	h.hooks = hooks
}
