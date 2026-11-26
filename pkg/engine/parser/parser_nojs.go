//go:build windows || 386

package parser

import "github.com/projectdiscovery/katana/pkg/decider"

type Options struct {
	AutomaticFormFill      bool
	ScrapeJSLuiceResponses bool
	ScrapeJSResponses      bool
	DisableRedirects       bool
	// FormDecider classifies form fields the rules cannot, nil disables it
	FormDecider decider.Decider
}

func (p *Parser) InitWithOptions(options *Options) {
	if options.AutomaticFormFill {
		*p = append(*p, responseParser{bodyParser, bodyFormTagParser(options.FormDecider)})
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
