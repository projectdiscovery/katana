package hybrid

import (
	"context"

	"github.com/go-rod/rod"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/utils/errkit"
)

// Hooks bundles optional per-page lifecycle callbacks invoked by the hybrid
// crawler for every navigated page. All fields are optional; nil callbacks are
// skipped, so the zero value changes nothing.
//
// Both callbacks run synchronously on the goroutine that navigates the page and
// block it for their duration. If Crawl is invoked concurrently on the same
// crawler, the callbacks must themselves be safe for concurrent use.
//
// page is the crawler's own tab, bound to the crawl session's context (it is
// cancelled with the crawl). ctx is the per-request navigation context and
// carries the -timeout deadline; use page.Context(ctx) or page.Timeout(d) to
// bound any CDP call made from a callback. The page is closed by the crawler
// as soon as the callback returns: callbacks must not retain it, close it, or
// navigate it.
//
// A non-nil error from a callback aborts that request: it is reported exactly
// like any other navigation error (an output.Result carrying the error, plus
// the error writer), and the crawl continues with the next request. Return nil
// for failures that should not cost the page. Callbacks must not panic.
type Hooks struct {
	// BeforeNavigate is invoked after the tab is created and the configured
	// headers/user agent are applied, and before request interception starts
	// and the page navigates to req.URL. It is the place for per-page setup
	// such as page.SetCookies or page.EvalOnNewDocument.
	BeforeNavigate func(ctx context.Context, page *rod.Page, req *navigation.Request) error

	// AfterLoad is invoked once the page has loaded according to the
	// page-load strategy, its rendered HTML has been captured into resp, and
	// before the page is closed. It is only called when resp is non-nil and
	// the request has not already failed. The callback may read the live page
	// (evaluate scripts, take a screenshot) and may attach data to the result
	// via resp.Extra, which is delivered to OnResult and the output writers.
	AfterLoad func(ctx context.Context, page *rod.Page, req *navigation.Request, resp *navigation.Response) error
}

// SetHooks installs per-page lifecycle callbacks on the hybrid crawler. The
// supplied struct is copied, so mutating it after SetHooks returns has no
// effect; call SetHooks again to change the installed hooks. Passing nil clears
// any previously installed hooks. SetHooks is not safe to call concurrently
// with Crawl on the same crawler.
func (c *Crawler) SetHooks(hooks *Hooks) {
	if hooks == nil {
		c.hooks = Hooks{}
		return
	}
	c.hooks = *hooks
}

// runBeforeNavigate invokes hooks.BeforeNavigate when set, wrapping its error
// so the reported failure names the hook and the URL.
func runBeforeNavigate(ctx context.Context, hooks Hooks, page *rod.Page, req *navigation.Request) error {
	cb := hooks.BeforeNavigate
	if cb == nil {
		return nil
	}
	if err := cb(ctx, page, req); err != nil {
		return errkit.Wrap(err, "hybrid: BeforeNavigate hook failed for "+requestURL(req))
	}
	return nil
}

// runAfterLoad invokes hooks.AfterLoad when set, wrapping its error so the
// reported failure names the hook and the URL.
func runAfterLoad(ctx context.Context, hooks Hooks, page *rod.Page, req *navigation.Request, resp *navigation.Response) error {
	cb := hooks.AfterLoad
	if cb == nil {
		return nil
	}
	if err := cb(ctx, page, req, resp); err != nil {
		return errkit.Wrap(err, "hybrid: AfterLoad hook failed for "+requestURL(req))
	}
	return nil
}

func requestURL(req *navigation.Request) string {
	if req == nil {
		return "<nil request>"
	}
	return req.URL
}
