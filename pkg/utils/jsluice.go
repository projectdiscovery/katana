package utils

import (
	"regexp"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex         = `(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscor|tween|retina|selectivizr|cufon|underscore|angular|swf|sha1|freestyle|jquery|bootstrap|modernizr|d3|backbone|videojs|google_analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|angular|fusion|analytics|lib|libs|vendor|vendors|node_modules)([-._][\w\d]*)*\.js$`
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
	return extractJsluiceEndpoints(data)
}
