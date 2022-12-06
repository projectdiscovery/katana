package parser

import (
	"mime/multipart"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/utils"
	"golang.org/x/net/html"
)

// responseParserFunc is a function that parses the document returning
// new navigation items or requests for the crawler.
type ResponseParserFunc func(resp navigation.Response, callback func(navigation.Request))

type responseParserType int

const (
	headerParser responseParserType = iota + 1
	bodyParser
	contentParser
)

type responseParser struct {
	parserType responseParserType
	parserFunc ResponseParserFunc
}

// responseParsers is a list of response parsers for the standard engine
var responseParsers = []responseParser{
	// Header based parsers
	{headerParser, headerContentLocationParser},
	{headerParser, headerLinkParser},
	{headerParser, headerLocationParser},
	{headerParser, headerRefreshParser},

	// Body based parsers
	{bodyParser, bodyATagParser},
	{bodyParser, bodyLinkHrefTagParser},
	{bodyParser, bodyBackgroundTagParser},
	{bodyParser, bodyAudioTagParser},
	{bodyParser, bodyAppletTagParser},
	{bodyParser, bodyImgTagParser},
	{bodyParser, bodyObjectTagParser},
	{bodyParser, bodySvgTagParser},
	{bodyParser, bodyTableTagParser},
	{bodyParser, bodyVideoTagParser},
	{bodyParser, bodyButtonFormactionTagParser},
	{bodyParser, bodyBlockquoteCiteTagParser},
	{bodyParser, bodyFrameSrcTagParser},
	{bodyParser, bodyMapAreaPingTagParser},
	{bodyParser, bodyBaseHrefTagParser},
	{bodyParser, bodyImportImplementationTagParser},
	{bodyParser, bodyEmbedTagParser},
	{bodyParser, bodyFrameTagParser},
	{bodyParser, bodyIframeTagParser},
	{bodyParser, bodyInputSrcTagParser},
	{bodyParser, bodyIsindexActionTagParser},
	{bodyParser, bodyScriptSrcTagParser},
	{bodyParser, bodyFormTagParser},
	{bodyParser, bodyMetaContentTagParser},
	{bodyParser, scriptContentRegexParser},
	{bodyParser, bodyHtmlManifestTagParser},
	{bodyParser, bodyHtmlDoctypeTagParser},

	// Optional JS relative endpoints parsers
	{contentParser, scriptJSFileRegexParser},
	{contentParser, bodyScrapeEndpointsParser},

	// custom field regex parser
	{bodyParser, customFieldRegexParser},
}

// parseResponse runs the response parsers on the navigation response
func ParseResponse(resp navigation.Response, callback func(navigation.Request)) {
	for _, parser := range responseParsers {
		switch {
		case parser.parserType == headerParser && resp.Resp != nil:
			parser.parserFunc(resp, callback)
		case parser.parserType == bodyParser && resp.Reader != nil:
			parser.parserFunc(resp, callback)
		case parser.parserType == contentParser && len(resp.Body) > 0:
			parser.parserFunc(resp, callback)
		}
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

// bodyLinkHrefTagParser parses link tag from response
func bodyLinkHrefTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("link[href]").Each(func(i int, item *goquery.Selection) {
		href, ok := item.Attr("href")
		if ok && href != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(href, resp.Resp.Request.URL.String(), "link", "href", resp))
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
	resp.Reader.Find("iframe").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "iframe", "src", resp))
		}
		srcDoc, ok := item.Attr("srcdoc")
		if ok && srcDoc != "" {
			endpoints := utils.ExtractRelativeEndpoints(srcDoc)
			for _, item := range endpoints {
				callback(navigation.NewNavigationRequestURLFromResponse(item, resp.Resp.Request.URL.String(), "iframe", "srcdoc", resp))
			}
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

// bodyBackgroundTagParser parses body background tag from response
func bodyBackgroundTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("body[background]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("background")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "body", "background", resp))
		}
	})
}

// bodyAudioTagParser parses body audio tag from response
func bodyAudioTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("audio").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "audio", "src", resp))
		}
		item.Find("source").Each(func(i int, s *goquery.Selection) {
			src, ok := s.Attr("src")
			if ok && src != "" {
				callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "audio", "source", resp))
			}
			srcSet, ok := s.Attr("srcset")
			if ok && srcSet != "" {
				for _, value := range utils.ParseSRCSetTag(srcSet) {
					callback(navigation.NewNavigationRequestURLFromResponse(value, resp.Resp.Request.URL.String(), "audio", "sourcesrcset", resp))
				}
			}
		})
	})
}

// bodyAppletTagParser parses body applet tag from response
func bodyAppletTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("applet").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("archive")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "applet", "archive", resp))
		}
		srcCodebase, ok := item.Attr("codebase")
		if ok && srcCodebase != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcCodebase, resp.Resp.Request.URL.String(), "applet", "codebase", resp))
		}
	})
}

// bodyImgTagParser parses Img tag from response
func bodyImgTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("img").Each(func(i int, item *goquery.Selection) {
		srcDynsrc, ok := item.Attr("dynsrc")
		if ok && srcDynsrc != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcDynsrc, resp.Resp.Request.URL.String(), "img", "dynsrc", resp))
		}
		srcLongdesc, ok := item.Attr("longdesc")
		if ok && srcLongdesc != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcLongdesc, resp.Resp.Request.URL.String(), "img", "longdesc", resp))
		}
		srcLowsrc, ok := item.Attr("lowsrc")
		if ok && srcLowsrc != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcLowsrc, resp.Resp.Request.URL.String(), "img", "lowsrc", resp))
		}
		src, ok := item.Attr("src")
		if ok && src != "" && src != "#" {
			if strings.HasPrefix(src, "data:") {
				// TODO: Add data:uri/data:image parsing
				return
			}
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "img", "src", resp))
		}
		srcSet, ok := item.Attr("srcset")
		if ok && srcSet != "" {
			for _, value := range utils.ParseSRCSetTag(srcSet) {
				callback(navigation.NewNavigationRequestURLFromResponse(value, resp.Resp.Request.URL.String(), "img", "srcset", resp))
			}
		}
	})
}

// bodyObjectTagParser parses object tag from response
func bodyObjectTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("object").Each(func(i int, item *goquery.Selection) {
		srcData, ok := item.Attr("data")
		if ok && srcData != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcData, resp.Resp.Request.URL.String(), "src", "data", resp))
		}
		srcCodebase, ok := item.Attr("codebase")
		if ok && srcCodebase != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcCodebase, resp.Resp.Request.URL.String(), "src", "codebase", resp))
		}
		item.Find("param").Each(func(i int, s *goquery.Selection) {
			srcValue, ok := s.Attr("value")
			if ok && srcValue != "" {
				callback(navigation.NewNavigationRequestURLFromResponse(srcValue, resp.Resp.Request.URL.String(), "src", "value", resp))
			}
		})
	})
}

// bodySvgTagParser parses svg tag from response
func bodySvgTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("svg").Each(func(i int, item *goquery.Selection) {
		item.Find("image").Each(func(i int, s *goquery.Selection) {
			hrefData, ok := s.Attr("href")
			if ok && hrefData != "" {
				callback(navigation.NewNavigationRequestURLFromResponse(hrefData, resp.Resp.Request.URL.String(), "svg", "image-href", resp))
			}
		})
		item.Find("script").Each(func(i int, s *goquery.Selection) {
			hrefData, ok := s.Attr("href")
			if ok && hrefData != "" {
				callback(navigation.NewNavigationRequestURLFromResponse(hrefData, resp.Resp.Request.URL.String(), "svg", "script-href", resp))
			}
		})
	})
}

// bodyTableTagParser parses table tag from response
func bodyTableTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("table").Each(func(i int, item *goquery.Selection) {
		srcData, ok := item.Attr("background")
		if ok && srcData != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcData, resp.Resp.Request.URL.String(), "table", "background", resp))
		}
		item.Find("td").Each(func(i int, s *goquery.Selection) {
			srcValue, ok := s.Attr("background")
			if ok && srcValue != "" {
				callback(navigation.NewNavigationRequestURLFromResponse(srcValue, resp.Resp.Request.URL.String(), "table", "td-background", resp))
			}
		})
	})
}

// bodyVideoTagParser parses video tag from response
func bodyVideoTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("video").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "video", "src", resp))
		}
		srcData, ok := item.Attr("poster")
		if ok && srcData != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(srcData, resp.Resp.Request.URL.String(), "video", "poster", resp))
		}
		item.Find("track").Each(func(i int, s *goquery.Selection) {
			srcValue, ok := s.Attr("src")
			if ok && srcValue != "" {
				callback(navigation.NewNavigationRequestURLFromResponse(srcValue, resp.Resp.Request.URL.String(), "video", "track-src", resp))
			}
		})
	})
}

// bodyButtonFormactionTagParser parses blockquote cite tag from response
func bodyBlockquoteCiteTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("blockquote[cite]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("cite")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "blockquote", "cite", resp))
		}
	})
}

// bodyFrameSrcTagParser parses frame src tag from response
func bodyFrameSrcTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("frame[src]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("src")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "frame", "src", resp))
		}
	})
}

// bodyMapAreaPingTagParser parses map area ping tag from response
func bodyMapAreaPingTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("area[ping]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("ping")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "area", "ping", resp))
		}
	})
}

// bodyBaseHrefTagParser parses base href tag from response
func bodyBaseHrefTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("base[href]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("href")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "base", "href", resp))
		}
	})
}

// bodyImportImplementationTagParser parses import implementation tag from response
func bodyImportImplementationTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("import[implementation]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("implementation")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "import", "implementation", resp))
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

// bodyHtmlManifestTagParser parses body manifest tag from response
func bodyHtmlManifestTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("html[manifest]").Each(func(i int, item *goquery.Selection) {
		src, ok := item.Attr("manifest")
		if ok && src != "" {
			callback(navigation.NewNavigationRequestURLFromResponse(src, resp.Resp.Request.URL.String(), "html", "manifest", resp))
		}
	})
}

// bodyHtmlDoctypeTagParser parses body doctype tag from response
func bodyHtmlDoctypeTagParser(resp navigation.Response, callback func(navigation.Request)) {
	if len(resp.Reader.Nodes) < 1 || resp.Reader.Nodes[0].FirstChild == nil {
		return
	}
	docTypeNode := resp.Reader.Nodes[0].FirstChild
	if docTypeNode.Type != html.DoctypeNode {
		return
	}
	if len(docTypeNode.Attr) == 0 || strings.ToLower(docTypeNode.Attr[0].Key) != "system" {
		return
	}
	callback(navigation.NewNavigationRequestURLFromResponse(docTypeNode.Attr[0].Val, resp.Resp.Request.URL.String(), "html", "doctype", resp))
}

// bodyFormTagParser parses forms from response
func bodyFormTagParser(resp navigation.Response, callback func(navigation.Request)) {
	resp.Reader.Find("form").Each(func(i int, item *goquery.Selection) {
		if !resp.Options.Options.AutomaticFormFill {
			return
		}
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
			Method:       method,
			URL:          actionURL,
			Depth:        resp.Depth,
			RootHostname: resp.RootHostname,
			Tag:          "form",
			Attribute:    "action",
			Source:       resp.Resp.Request.URL.String(),
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
	resp.Reader.Find("meta").Each(func(i int, item *goquery.Selection) {
		header, ok := item.Attr("content")
		if !ok {
			return
		}
		extracted := utils.ExtractRelativeEndpoints(header)
		for _, item := range extracted {
			callback(navigation.NewNavigationRequestURLFromResponse(item, resp.Resp.Request.URL.String(), "meta", "refresh", resp))
		}
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
	// CSS, JS are supported for relative endpoint extraction.
	contentType := resp.Resp.Header.Get("Content-Type")
	if !(strings.HasSuffix(resp.Resp.Request.URL.Path, ".js") || strings.HasSuffix(resp.Resp.Request.URL.Path, ".css") || strings.Contains(contentType, "/javascript")) {
		return
	}

	endpoints := utils.ExtractRelativeEndpoints(string(resp.Body))
	for _, item := range endpoints {
		callback(navigation.NewNavigationRequestURLFromResponse(item, resp.Resp.Request.URL.String(), "js", "regex", resp))
	}
}

// bodyScrapeEndpointsParser parses scraped URLs from HTML body
func bodyScrapeEndpointsParser(resp navigation.Response, callback func(navigation.Request)) {
	if !resp.Options.Options.ScrapeJSResponses { // do not process if disabled
		return
	}

	endpoints := utils.ExtractBodyEndpoints(string(resp.Body))
	for _, item := range endpoints {
		callback(navigation.NewNavigationRequestURLFromResponse(item, resp.Resp.Request.URL.String(), "html", "regex", resp))
	}
}

// customFieldRegexParser parses custom regex from HTML body and header
func customFieldRegexParser(resp navigation.Response, callback func(navigation.Request)) {
	var customField = make(map[string][]string)
	for _, v := range output.CustomFieldsMap {
		for _, re := range v.CompileRegex {
			results := []string{}
			// read body
			matches := re.FindAllStringSubmatch(string(resp.Body), -1)

			// read header
			for _, v := range resp.Resp.Header {
				headerMatches := re.FindAllStringSubmatch(strings.Join(v, "\n"), -1)
				matches = append(matches, headerMatches...)
			}
			for _, match := range matches {
				if len(match) < (v.Group + 1) {
					continue
				}
				matchString := match[v.Group]
				results = append(results, matchString)
			}
			customField[v.GetName()] = results
		}
	}
	if len(customField) != 0 {
		callback(navigation.Request{
			Method:       "GET",
			URL:          resp.Resp.Request.URL.String(),
			Source:       resp.Resp.Request.URL.String(),
			Attribute:    "regex",
			Tag:          "regex",
			Depth:        resp.Depth,
			CustomFields: customField,
		})
	}
}
