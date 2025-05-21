//go:build windows || 386

package parser

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
	if options.ScrapeJSResponses {
		*p = append(*p, responseParser{bodyParser, scriptContentRegexParser})
		*p = append(*p, responseParser{contentParser, scriptJSFileRegexParser})
		*p = append(*p, responseParser{contentParser, bodyScrapeEndpointsParser})
	}
	if !options.DisableRedirects {
		*p = append(*p, responseParser{headerParser, headerLocationParser})
	}
}
