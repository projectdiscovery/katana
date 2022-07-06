package navigation

import (
	"strings"

	"github.com/projectdiscovery/katana/ent"
)

// NavigationRequest is a navigation request for the crawler
type NavigationRequest struct {
	Method  string
	URL     string
	Body    string
	Depth   int
	Headers map[string]string

	Source   string // source is the source of the request
	FromNode *ent.Endpoint
}

// RequestURL returns the request URL for the navigation
func (n *NavigationRequest) RequestURL() string {
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

// NewNavigationRequestURL generates a navigation request from a relative URL
func NewNavigationRequestURL(path, source string, resp NavigationResponse) NavigationRequest {
	requestURL := resp.AbsoluteURL(path)
	return NavigationRequest{Method: "GET", URL: requestURL, Depth: resp.Depth, Source: source}
}

func (n *NavigationRequest) ToEphemeralEntity() *ent.Endpoint {
	return &ent.Endpoint{
		URL:     n.URL,
		Headers: n.Headers,
		Method:  n.Method,
		Body:    n.Body,
		Source:  n.Source,
	}
}
