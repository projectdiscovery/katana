package navigation

import (
	"net/http"
	"strings"

	urlutil "github.com/projectdiscovery/utils/url"
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

// NewNavigationRequestURLFromResponseForMaps generates a navigation requests for .map or .json files
// this files contains the file locations, to generate the url with greater chance of success
// also create urls with remove the path to create new paths
// eg: consider a path ./constants passed to NewNavigationRequestURLFromResponse will create https://YOUR_URL/SOME_PATH/SOME_MORE_PATH/constants
// but we also want https://YOUR_URL/constants
func NewNavigationRequestURLFromResponseForMaps(path, source, tag, attribute string, resp Response) *Request {
	requestURL := resp.AbsoluteURL(path)
	// contains url not path
	if requestURL == path || requestURL == "" {
		return nil
	}
	// copy the url and set the path to empty
	temp := urlutil.URL{URL: resp.Resp.Request.URL}
	tempURL := temp.Clone()
	tempURL.Path = ""
	// parse the  url with the path
	absURL, err := tempURL.Parse(path)
	if err != nil {
		return nil
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = resp.Resp.Request.URL.Scheme
	}
	return &Request{
		Method:       http.MethodGet,
		URL:          absURL.String(),
		RootHostname: resp.RootHostname,
		Depth:        resp.Depth,
		Source:       source,
		Attribute:    attribute,
		Tag:          tag,
	}
}
