package standard

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/types"
)

// navigationResponse is a response generated from crawler navigation
type navigationResponse struct {
	Resp   *http.Response
	Depth  int
	Reader *goquery.Document
	Body   []byte

	options *types.CrawlerOptions
}

// AbsoluteURL parses the path returning a string.
//
// It returns a blank string if the item is invalid, not in-scope
// or can't be formatted.
func (n navigationResponse) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "#") {
		return ""
	}
	if !n.validatePath(path) {
		return ""
	}
	absURL, err := n.Resp.Request.URL.Parse(path)
	if err != nil {
		return ""
	}
	if validated, err := n.validateScope(absURL); err != nil || !validated {
		return ""
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = n.Resp.Request.URL.Scheme
	}
	final := absURL.String()
	return final
}

func (n navigationResponse) validatePath(path string) bool {
	if n.options != nil && n.options.ExtensionsValidator != nil {
		return n.options.ExtensionsValidator.ValidatePath(path)
	}
	return true
}

func (n navigationResponse) validateScope(absURL *url.URL) (bool, error) {
	if n.options != nil && n.options.ScopeManager != nil {
		return n.options.ScopeManager.Validate(absURL)
	}
	return true, nil
}
