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
	if _, err := page.EvalOnNewDocument(utilsJavascriptBundle); err != nil {
		return errors.Wrap(err, "failed to inject utils.js")
	}
	if _, err := page.EvalOnNewDocument(pageInitJavascriptBundle); err != nil {
		return errors.Wrap(err, "failed to inject page-init.js")
	}
	return nil
}
