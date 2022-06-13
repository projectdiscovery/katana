package standard

import (
	"net/http"
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

func (n navigationResponse) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "#") {
		return ""
	}
	if !n.options.ExtensionsValidator.ValidatePath(path) {
		return ""
	}
	absURL, err := n.Resp.Request.URL.Parse(path)
	if err != nil {
		return ""
	}
	if validated, err := n.options.ScopeManager.Validate(absURL); err != nil || !validated {
		return ""
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = n.Resp.Request.URL.Scheme
	}
	final := absURL.String()
	return final
}
