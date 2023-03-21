package navigation

import (
	"net/http"
	"strings"
)

// Depth is the depth of a navigation
type Depth struct{}

// Request is a navigation request for the crawler
type Request struct {
	Method       string              `json:"method,omitempty"`
	URL          string              `json:"endpoint,omitempty"`
	Body         string              `json:"body,omitempty"`
	Depth        int                 `json:"-"`
	Headers      map[string]string   `json:"headers,omitempty"`
	Tag          string              `json:"tag,omitempty"`
	Attribute    string              `json:"attribute,omitempty"`
	RootHostname string              `json:"-"`
	Source       string              `json:"source,omitempty"`
	CustomFields map[string][]string `json:"-"`
	Raw          string              `json:"raw,omitempty"`
}

// RequestURL returns the request URL for the navigation
func (n *Request) RequestURL() string {
	switch n.Method {
	case "GET":
		return n.URL
	case "POST":
		builder := &strings.Builder{}
		builder.WriteString(n.URL)
		builder.WriteString(":")
		builder.WriteString(n.Body)
		builtURL := builder.String()
		return builtURL
	}
	return ""
}

// newNavigationRequestURL generates a navigation request from a relative URL
func NewNavigationRequestURLFromResponse(path, source, tag, attribute string, resp *Response) *Request {
	requestURL := resp.AbsoluteURL(path)
	request := &Request{
		Method:       http.MethodGet,
		URL:          requestURL,
		RootHostname: resp.RootHostname,
		Depth:        resp.Depth,
		Source:       source,
		Attribute:    attribute,
		Tag:          tag,
	}
	return request
}
