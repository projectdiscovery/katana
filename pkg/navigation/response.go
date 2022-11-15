package navigation

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/types"
)

// Response is a response generated from crawler navigation
type Response struct {
	State        *State
	Resp         *http.Response
	Depth        int
	Reader       *goquery.Document
	Body         []byte
	RootHostname string

	Options *types.CrawlerOptions
}

// AbsoluteURL parses the path returning a string.
//
// It returns a blank string if the item is invalid, not in-scope
// or can't be formatted.
func (n Response) AbsoluteURL(path string) string {
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
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = n.Resp.Request.URL.Scheme
	}
	final := absURL.String()
	return final
}

func (n Response) validatePath(path string) bool {
	if n.Options != nil && n.Options.ExtensionsValidator != nil {
		return n.Options.ExtensionsValidator.ValidatePath(path)
	}
	return true
}

// ValidateScope validates scope for an AbsURL
func (n Response) ValidateScope(absURL string) (bool, error) {
	parsed, err := url.Parse(absURL)
	if err != nil {
		return false, err
	}
	if n.Options != nil && n.Options.ScopeManager != nil {
		return n.Options.ScopeManager.Validate(parsed, n.RootHostname)
	}
	return true, nil
}
