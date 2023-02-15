package navigation

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Response is a response generated from crawler navigation
type Response struct {
	Resp         *http.Response
	Depth        int
	Reader       *goquery.Document
	Body         []byte
	RootHostname string
	Technologies []string
}

func (n Response) AbsoluteURL(path string) string {
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
