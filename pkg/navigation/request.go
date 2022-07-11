package navigation

import (
	"strings"
)

// Request is a navigation request for the crawler
type Request struct {
	Method  string
	URL     string
	Body    string
	Depth   int
	Headers map[string]string

	Source string // source is the source of the request
}

// RequestURL returns the request URL for the navigation
func (r *Request) RequestURL() string {
	switch r.Method {
	case "GET":
		return r.URL
	case "POST":
		builder := &strings.Builder{}
		builder.WriteString(r.URL)
		builder.WriteString(":")
		builder.WriteString(r.Body)
		builtURL := builder.String()
		return builtURL
	}
	return ""
}

// newNavigationRequestURL generates a navigation request from a relative URL
func NewRequestURL(path, source string, resp Response) Request {
	requestURL := resp.AbsoluteURL(path)
	return Request{Method: "GET", URL: requestURL, Depth: resp.Depth, Source: source}
}
