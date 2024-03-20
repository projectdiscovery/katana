//go:build !386

package utils

import (
	"regexp"

	"github.com/BishopFox/jsluice"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex         = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts jsluice endpoints from a given string.
//
// We use tomnomnom and bishopfox's jsluice to extract endpoints from javascript
// files.
//
// We apply several optimizations before running jsluice:
//   - We skip common js library files.
//   - We skip lines that are too long and contain a lot of characters.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	analyzer := jsluice.NewAnalyzer([]byte(data))

	// TODO: add new user url matchers
	// analyzer.AddURLMatcher(matcher)

	var endpoints []JSLuiceEndpoint
	foundURLs := analyzer.GetURLs()

	for _, url := range foundURLs {
		url := url
		endpoints = append(endpoints, JSLuiceEndpoint{
			Endpoint: url.URL,
			Type:     url.Type,
		})
	}
	return endpoints
}
