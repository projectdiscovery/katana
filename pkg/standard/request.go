package standard

import (
	"strings"
)

// navigationRequest is a navigation request for the crawler
type navigationRequest struct {
	Method  string
	URL     string
	Body    string
	Depth   int
	Headers map[string]string

	Source string // source is the source of the request
}

// RequestURL returns the request URL for the navigation
func (n *navigationRequest) RequestURL() string {
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
func newNavigationRequestURL(path, source string, resp navigationResponse) navigationRequest {
	requestURL := resp.AbsoluteURL(path)
	return navigationRequest{Method: "GET", URL: requestURL, Depth: resp.Depth, Source: source}
}
