package navigation

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	jsoniter "github.com/json-iterator/go"
)

type Headers map[string]string

func (h *Headers) MarshalJSON() ([]byte, error) {
	hCopy := make(Headers)
	for k, v := range *h {
		k := strings.ReplaceAll(strings.ToLower(k), "-", "_")
		hCopy[k] = v
	}
	return jsoniter.Marshal(hCopy)
}

// Response is a response generated from crawler navigation
type Response struct {
	Resp         *http.Response    `json:"-"`
	Depth        int               `json:"-"`
	Reader       *goquery.Document `json:"-"`
	StatusCode   int               `json:"status_code,omitempty"`
	Headers      Headers           `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	RootHostname string            `json:"-"`
	Technologies []string          `json:"technologies,omitempty"`
	Raw          string            `json:"raw,omitempty"`
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
