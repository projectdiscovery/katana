package navigation

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/types"
)

// NavigationResponse is a response generated from crawler navigation
type NavigationResponse struct {
	Resp   *http.Response
	Depth  int
	Reader *goquery.Document
	Body   []byte

	Options *types.CrawlerOptions
}

// AbsoluteURL parses the path returning a string.
//
// It returns a blank string if the item is invalid, not in-scope
// or can't be formatted.
func (n NavigationResponse) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "#") {
		return ""
	}
	if !n.Options.ExtensionsValidator.ValidatePath(path) {
		return ""
	}
	absURL, err := n.Resp.Request.URL.Parse(path)
	if err != nil {
		return ""
	}
	if validated, err := n.Options.ScopeManager.Validate(absURL); err != nil || !validated {
		return ""
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = n.Resp.Request.URL.Scheme
	}
	final := absURL.String()
	return final
}
