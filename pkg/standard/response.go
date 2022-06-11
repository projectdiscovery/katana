package standard

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// navigationResponse is a response generated from crawler navigation
type navigationResponse struct {
	Resp   *http.Response
	Depth  int
	Reader *goquery.Document
}

func (n navigationResponse) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "#") {
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
