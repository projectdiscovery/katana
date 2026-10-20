package js

import (
	_ "embed"

	"github.com/go-rod/rod"
	"github.com/pkg/errors"
)

var (
	//go:embed utils.js
	utilsJavascriptBundle string

	//go:embed page-init.js
	pageInitJavascriptBundle string
)

// InitJavascriptEnv injects the necessary javascript code into the browser
func InitJavascriptEnv(page *rod.Page) error {
	return InitJavascriptEnvWithHooks(page, false)
}

// InitJavascriptEnvWithHooks enables listener discovery only for the opt-in
// interactive crawler; instrumenting every listener slows ordinary crawls.
func InitJavascriptEnvWithHooks(page *rod.Page, hooks bool) error {
	if _, err := page.EvalOnNewDocument(utilsJavascriptBundle); err != nil {
		return errors.Wrap(err, "failed to inject utils.js")
	}
	if hooks {
		if _, err := page.EvalOnNewDocument(`window.__katanaHooksOptions = { hooked: true };`); err != nil {
			return errors.Wrap(err, "failed to enable interaction hooks")
		}
	}
	if _, err := page.EvalOnNewDocument(pageInitJavascriptBundle); err != nil {
		return errors.Wrap(err, "failed to inject page-init.js")
	}
	return nil
}
