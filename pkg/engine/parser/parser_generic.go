//go:build !386

package parser

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
)

func InitWithOptions(options *types.Options) {
	if options.AutomaticFormFill {
		responseParsers = append(responseParsers, responseParser{bodyParser, bodyFormTagParser})
	}
	if options.ScrapeJSLuiceResponses {
		responseParsers = append(responseParsers, responseParser{bodyParser, scriptContentJsluiceParser})
		responseParsers = append(responseParsers, responseParser{contentParser, scriptJSFileJsluiceParser})
	}
	if options.ScrapeJSResponses {
		responseParsers = append(responseParsers, responseParser{bodyParser, scriptContentRegexParser})
		responseParsers = append(responseParsers, responseParser{contentParser, scriptJSFileRegexParser})
		responseParsers = append(responseParsers, responseParser{contentParser, bodyScrapeEndpointsParser})
	}
	if !options.DisableRedirects {
		responseParsers = append(responseParsers, responseParser{headerParser, headerLocationParser})
	}
}

// scriptContentJsluiceParser parses script content endpoints using jsluice from response
func scriptContentJsluiceParser(resp *navigation.Response) (navigationRequests []*navigation.Request) {
	resp.Reader.Find("script").Each(func(i int, item *goquery.Selection) {
		text := item.Text()
		if text == "" {
			return
		}

		endpointItems := utils.ExtractJsluiceEndpoints(text)
		for _, item := range endpointItems {
			navigationRequests = append(navigationRequests, navigation.NewNavigationRequestURLFromResponse(item.Endpoint, resp.Resp.Request.URL.String(), "script", fmt.Sprintf("jsluice-%s", item.Type), resp))
		}
	})
	return
}

// scriptJSFileJsluiceParser parses endpoints using jsluice from js file pages
func scriptJSFileJsluiceParser(resp *navigation.Response) (navigationRequests []*navigation.Request) {
	// Only process javascript file based on path or content type
	// CSS, JS are supported for relative endpoint extraction.
	contentType := resp.Resp.Header.Get("Content-Type")
	if !(strings.HasSuffix(resp.Resp.Request.URL.Path, ".js") || strings.HasSuffix(resp.Resp.Request.URL.Path, ".css") || strings.Contains(contentType, "/javascript")) {
		return
	}
	// Skip common js libraries
	if utils.IsPathCommonJSLibraryFile(resp.Resp.Request.URL.Path) {
		return
	}

	endpointsItems := utils.ExtractJsluiceEndpoints(string(resp.Body))
	for _, item := range endpointsItems {
		navigationRequests = append(navigationRequests, navigation.NewNavigationRequestURLFromResponse(item.Endpoint, resp.Resp.Request.URL.String(), "js", fmt.Sprintf("jsluice-%s", item.Type), resp))
	}
	return
}
