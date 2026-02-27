package parser

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	stringsutil "github.com/projectdiscovery/utils/strings"
)

// Options contains the configuration options for the parser
type Options struct {
	AutomaticFormFill      bool
	ScrapeJSLuiceResponses bool
	ScrapeJSResponses      bool
	DisableRedirects       bool
}

// InitWithOptions initializes the parser with the given options
func (p *Parser) InitWithOptions(options *Options) {
	if options.AutomaticFormFill {
		*p = append(*p, responseParser{bodyParser, nil})
	}
	// ScrapeJSLuiceResponses now uses our internal pure-Go regex parser
	if options.ScrapeJSLuiceResponses {
		*p = append(*p, responseParser{contentParser, scriptJSFileJsluiceParser})
		*p = append(*p, responseParser{contentParser, scriptContentJsluiceParser})
	}
	if options.ScrapeJSResponses {
		*p = append(*p, responseParser{bodyParser, nil})
		*p = append(*p, responseParser{contentParser, nil})
	}
	if !options.DisableRedirects {
		*p = append(*p, responseParser{headerParser, nil})
	}
}



// scriptContentJsluiceParser parses script content endpoints using internal regex parser from response
func scriptContentJsluiceParser(resp *navigation.Response) (navigationRequests []*navigation.Request) {
	resp.Reader.Find("script").Each(func(i int, item *goquery.Selection) {
		text := item.Text()
		if text == "" {
			return
		}

		endpointItems := utils.ExtractJsluiceEndpoints(text)
		for _, item := range endpointItems {
			navigationRequests = append(navigationRequests, navigation.NewNavigationRequestURLFromResponse(item.Endpoint, resp.Resp.Request.URL.String(), "script", fmt.Sprintf("regex-%s", item.Type), resp))
		}
	})
	return
}

// scriptJSFileJsluiceParser parses endpoints using internal regex parser from js file pages
func scriptJSFileJsluiceParser(resp *navigation.Response) (navigationRequests []*navigation.Request) {
	contentType := resp.Resp.Header.Get("Content-Type")
	if !stringsutil.HasSuffixAny(resp.Resp.Request.URL.Path, ".js", ".jsx", ".ts", ".tsx") && !strings.Contains(contentType, "javascript") {
		return
	}
	
	if utils.IsPathCommonJSLibraryFile(resp.Resp.Request.URL.Path) {
		return
	}

	endpointsItems := utils.ExtractJsluiceEndpoints(string(resp.Body))
	for _, item := range endpointsItems {
		navigationRequests = append(navigationRequests, navigation.NewNavigationRequestURLFromResponse(item.Endpoint, resp.Resp.Request.URL.String(), "js", fmt.Sprintf("regex-%s", item.Type), resp))
	}
	return
}

