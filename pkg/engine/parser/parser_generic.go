//go:build !(386 || windows)

package parser

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	stringsutil "github.com/projectdiscovery/utils/strings"
)

type Options struct {
	AutomaticFormFill      bool
	ScrapeJSLuiceResponses bool
	ScrapeJSResponses      bool
	DisableRedirects       bool
}

func (p *Parser) InitWithOptions(options *Options) {
	if options.AutomaticFormFill {
		*p = append(*p, responseParser{bodyParser, bodyFormTagParser})
	}
	if options.ScrapeJSLuiceResponses {
		*p = append(*p, responseParser{bodyParser, scriptContentJsluiceParser})
		*p = append(*p, responseParser{contentParser, scriptJSFileJsluiceParser})
	}
	if options.ScrapeJSResponses {
		*p = append(*p, responseParser{bodyParser, scriptContentRegexParser})
		*p = append(*p, responseParser{contentParser, scriptJSFileRegexParser})
		*p = append(*p, responseParser{contentParser, bodyScrapeEndpointsParser})
	}
	if !options.DisableRedirects {
		*p = append(*p, responseParser{headerParser, headerLocationParser})
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
	if !stringsutil.HasSuffixAny(resp.Resp.Request.URL.Path, ".js", ".css") && !strings.Contains(contentType, "/javascript") {
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
