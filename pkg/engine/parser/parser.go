package parser

import (
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
)

// responseParserFunc is a function that parses the document returning
// new navigation items or requests for the crawler.
type ResponseParserFunc func(resp navigation.Response, callback func(navigation.Request))

// responseParsers is a list of response parsers for the standard engine
var responseParsers = []ResponseParserFunc{
	// Header based parsers
	headerContentLocationParser,
	headerLinkParser,
	headerLocationParser,
	headerRefreshParser,

	// Body based parsers
	bodyATagParser,
	bodyEmbedTagParser,
	bodyFrameTagParser,
	bodyIframeTagParser,
	bodyInputSrcTagParser,
	bodyIsindexActionTagParser,
	bodyScriptSrcTagParser,
	bodyFormTagParser,
	bodyMetaContentTagParser,

	// Optional JS relative endpoints parsers
	scriptContentRegexParser,
	scriptJSFileRegexParser,
}

// parseResponse runs the response parsers on the navigation response
func ParseResponse(resp navigation.Response, callback func(navigation.Request)) {
	for _, parser := range responseParsers {
		parser(resp, callback)
	}
}

// -------------------------------------------------------------------------
// Begin Header based parsers
// -------------------------------------------------------------------------

// headerContentLocationParser parsers Content-Location header from response
func headerContentLocationParser(resp navigation.Response, callback func(navigation.Request)) {
	header := resp.Resp.Header.Get("Content-Location")
	if header == "" {
		return
	}
	callback(navigation.NewNavigationRequestURLFromResponse(header, resp.Resp.Request.URL.String(), "header", "content-location", resp))
}

// headerLinkParser parsers Link header from response
func headerLinkParser(resp navigation.Response, callback func(navigation.Request)) {
	header := resp.Resp.Header.Get("Link")
	if header == "" {
		return
	}
	values := utils.ParseLinkTag(header)
	for _, value := range values {
		callback(navigation.NewNavigationRequestURLFromResponse(value, resp.Resp.Request.URL.String(), "header", "link", resp))
	}
}

// headerLocationParser parsers Location header from response
func headerLocationParser(resp navigation.Response, callback func(navigation.Request)) {
	header := resp.Resp.Header.Get("Location")
	if header == "" {
		return
	}
	callback(navigation.NewNavigationRequestURLFromResponse(header, resp.Resp.Request.URL.String(), "header", "location", resp))
}

// headerRefreshParser parsers Refresh header from response
func headerRefreshParser(resp navigation.Response, callback func(navigation.Request)) {
	header := resp.Resp.Header.Get("Refresh")
	if header == "" {
		return
	}
	values := utils.ParseRefreshTag(header)
	if values == "" {
		return
	}
	callback(navigation.NewNavigationRequestURLFromResponse(values, resp.Resp.Request.URL.String(), "header", "refresh", resp))
}

// -------------------------------------------------------------------------
// Begin Body based parsers
// -------------------------------------------------------------------------

// bodyATagParser parses A tag from response
func bodyATagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("a").Each(func(i int, item *goquery.Selection) {
		href, ok := item.Attr("href")
		if ok && href != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(href, resp.Resp.Request.URL.String(), "a", "href", resp))
		}
		ping, ok := item.Attr("ping")
		if ok && ping != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(ping, resp.Resp.Request.URL.String(), "a", "ping", resp))
		}
	})
}

// bodyEmbedTagParser parses Embed tag from response
func bodyEmbedTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("embed[src]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "embed", "src", resp))
		}
	})
}

// bodyFrameTagParser parses frame tag from response
func bodyFrameTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("frame[src]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "frame", "src", resp))
		}
	})
}

// bodyIframeTagParser parses iframe tag from response
func bodyIframeTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("iframe[src]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "iframe", "src", resp))
		}
	})
}

// bodyInputSrcTagParser parses input image src tag from response
func bodyInputSrcTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("input[type='image' i]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "input-image", "src", resp))
		}
	})
}

// bodyIsindexActionTagParser parses isindex action tag from response
func bodyIsindexActionTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("isindex[action]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("action")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "isindex", "action", resp))
		}
	})
}

// bodyScriptSrcTagParser parses script src tag from response
func bodyScriptSrcTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("script[src]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "script", "src", resp))
		}
	})
}

// bodyButtonFormactionTagParser parses button formaction tag from response
func bodyButtonFormactionTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("button[formaction]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("formaction")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "button", "formaction", resp))
		}
	})
}

// bodyFormTagParser parses forms from response
func bodyFormTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("form").Each(func(i int, item *goquery.Selection) {
		href, _ := item.Attr("action")
		encType, ok := item.Attr("enctype")
		if !ok || encType == "" {
			encType = "application/x-www-form-urlencoded"
		}

		method, _ := item.Attr("method")
		if method == "" {
			method = "GET"
		}
		method = strings.ToUpper(method)

		actionURL := resp.AbsoluteURL(href)
		if actionURL == "" {
			return
		}

		isMultipartForm := strings.HasPrefix(encType, "multipart/")

		queryValuesWriter := make(url.Values)
		var sb strings.Builder
		var multipartWriter *multipart.Writer

		if isMultipartForm {
			multipartWriter = multipart.NewWriter(&sb)
		}

		// Get the form field suggestions for all inputs
		formInputs := []utils.FormInput{}
		item.Find("input").Each(func(index int, item *goquery.Selection) {
			if len(item.Nodes) == 0 {
				return
			}
			formInputs = append(formInputs, utils.ConvertGoquerySelectionToFormInput(item))
		})

		dataMap := utils.FormInputFillSuggestions(formInputs)
		for key, value := range dataMap {
			if key == "" || value == "" {
				continue
			}
			if isMultipartForm {
				_ = multipartWriter.WriteField(key, value)
			} else {
				queryValuesWriter.Set(key, value)
			}
		}

		// Guess content-type
		var contentType string
		if multipartWriter != nil {
			multipartWriter.Close()
			contentType = multipartWriter.FormDataContentType()
		} else {
			contentType = encType
		}

		req := navigation.Request{
			Method:    method,
			URL:       actionURL,
			Depth:     resp.Depth,
			Tag:       "form",
			Attribute: "action",
			Source:    resp.Resp.Request.URL.String(),
		}
		switch method {
		case "GET":
			value := queryValuesWriter.Encode()
			sb.Reset()
			sb.WriteString(req.URL)
			sb.WriteString("?")
			sb.WriteString(value)
			req.URL = sb.String()
		case "POST":
			if multipartWriter != nil {
				req.Body = sb.String()
			} else {
				req.Body = queryValuesWriter.Encode()
			}
			req.Headers = make(map[string]string)
			req.Headers["Content-Type"] = contentType
		}
		callback(req)
	})
}

// bodyMetaContentTagParser parses meta content tag from response
func bodyMetaContentTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("meta[http-equiv='refresh' i]").Each(func(i int, item *goquery.Selection) {
		header, ok := item.Attr("content")
		if !ok {
			return
		}
		values := utils.ParseRefreshTag(header)
		if values == "" {
			return
		}
		callback(navigation.NewNavigationRequestURLFromResponse(values, resp.Resp.Request.URL.String(), "meta", "refresh", resp))
	})
}

// -------------------------------------------------------------------------
// Begin JS Regex based parsers
// -------------------------------------------------------------------------

// scriptContentRegexParser parses script content endpoints from response
func scriptContentRegexParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("script").Each(func(i int, item *goquery.Selection) {
		if !resp.Options.Options.ScrapeJSResponses { // do not process if disabled
			return
		}
		text := item.Text()
		if text == "" {
			return
		}
		endpoints := utils.ExtractRelativeEndpoints(text)
		for _, item := range endpoints {
			callback(navigation.NewNavigationRequestURLFromResponse(item, resp.Resp.Request.URL.String(), "script", "text", resp))
		}
	})
}

// scriptJSFileRegexParser parses relative endpoints from js file pages
func scriptJSFileRegexParser(resp navigation.Response, callback func(navigation.Request)) {
	if !resp.Options.Options.ScrapeJSResponses { // do not process if disabled
		return
	}

	// Only process javascript file based on path or content type
	contentType := resp.Resp.Header.Get("Content-Type")
	if !(strings.HasSuffix(resp.Resp.Request.URL.Path, ".js") || strings.Contains(contentType, "/javascript")) {
		return
	}

	endpoints := utils.ExtractRelativeEndpoints(string(resp.Body))
	for _, item := range endpoints {
		callback(navigation.NewNavigationRequestURLFromResponse(item, resp.Resp.Request.URL.String(), "js", "regex", resp))
	}
}