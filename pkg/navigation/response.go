package navigation

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	jsoniter "github.com/json-iterator/go"
)

type Headers map[string]string

type Form struct {
	Method     string   `json:"method,omitempty"`
	Action     string   `json:"action,omitempty"`
	Enctype    string   `json:"enctype,omitempty"`
	Parameters []string `json:"parameters,omitempty"`
}

func (h *Headers) MarshalJSON() ([]byte, error) {
	hCopy := make(Headers)
	for k, v := range *h {
		k := strings.ToLower(k)
		hCopy[k] = v
	}
	return jsoniter.Marshal(hCopy)
}

// Response is a response generated from crawler navigation
type Response struct {
	Resp               *http.Response    `json:"-"`
	Depth              int               `json:"-"`
	Reader             *goquery.Document `json:"-"`
	StatusCode         int               `json:"status_code,omitempty"`
	Headers            Headers           `json:"headers,omitempty"`
	Body               string            `json:"body,omitempty"`
	RootHostname       string            `json:"-"`
	Technologies       []string          `json:"technologies,omitempty"`
	Raw                string            `json:"raw,omitempty"`
	Forms              []Form            `json:"forms,omitempty"`
	XhrRequests        []Request         `json:"xhr_requests,omitempty"`
	StoredResponsePath string            `json:"stored_response_path,omitempty"`
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

func (n Response) IsRedirect() bool {
	return n.StatusCode >= 300 && n.StatusCode <= 399
}
