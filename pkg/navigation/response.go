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
func (r Response) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "#") {
		return ""
	}
	if !r.ValidatePath(path) {
		return ""
	}
	absURL, err := r.Resp.Request.URL.Parse(path)
	if err != nil {
		return ""
	}
	if validated, err := r.ValidateScope(absURL); err != nil || !validated {
		return ""
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = r.Resp.Request.URL.Scheme
	}
	final := absURL.String()
	return final
}

func (r Response) ValidatePath(path string) bool {
	if r.Options != nil && r.Options.ExtensionsValidator != nil {
		return r.Options.ExtensionsValidator.ValidatePath(path)
	}
	return true
}

func (r Response) ValidateScope(absURL *url.URL) (bool, error) {
	if r.Options != nil && r.Options.ScopeManager != nil {
		return r.Options.ScopeManager.Validate(absURL)
	}
	return true, nil
}
