package standard

// navigationRequest is a navigation request for the crawler
type navigationRequest struct {
	Method  string
	URL     string
	Body    string
	Headers map[string]string
}

// newNavigationRequestURL generates a navigation request from a relative URL
func newNavigationRequestURL(path string, resp navigationResponse) navigationRequest {
	requestURL := resp.AbsoluteURL(path)
	return navigationRequest{Method: "GET", URL: requestURL}
}
