package utils

import (
	"regexp"
	"strings"

	"github.com/dop251/goja/parser"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	// URL/endpoint patterns commonly found in JavaScript code
	urlPatterns = []*regexp.Regexp{
		// Full URLs with protocol
		regexp.MustCompile(`(?i)(?:https?:)?//[^\s"'\x60<>()]+`),
		// Absolute paths
		regexp.MustCompile(`(?:["\x60']|=\s*)(/[a-zA-Z0-9_\-/.?=&]+)(?:["\x60']|\s|,|;)`),
		// API endpoints
		regexp.MustCompile(`(?i)(?:["\x60']|:\s*)(/api/[a-zA-Z0-9_\-/.?=&]+)(?:["\x60']|\s|,|;)`),
	}

	// stringPattern matches quoted strings (single quotes, double quotes, or backticks)
	stringPattern = regexp.MustCompile("[\"'`]([^\"'`]+)[\"'`]")
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript code using pure-Go parsing.
// This is a replacement for BishopFox's jsluice that doesn't require CGO.
//
// We use dop251/goja parser for JavaScript parsing and regex patterns for URL extraction.
//
// We apply several optimizations:
//   - We skip common js library files.
//   - We skip lines that are too long and contain a lot of characters.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	var endpoints []JSLuiceEndpoint
	seen := make(map[string]bool)

	// Try to parse as JavaScript for better results
	_, err := parser.ParseFile(nil, "", data, 0)
	if err == nil {
		// If valid JavaScript, extract string literals using regex
		endpoints = extractFromValidJS(data, seen)
	} else {
		// Fallback to simple regex extraction if not valid JS
		endpoints = extractFromRegex(data, seen)
	}

	return endpoints
}

// extractFromValidJS extracts endpoints from valid JavaScript code
func extractFromValidJS(data string, seen map[string]bool) []JSLuiceEndpoint {
	var endpoints []JSLuiceEndpoint

	// Extract quoted strings that look like URLs/paths
	matches := stringPattern.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		if len(match) > 1 {
			value := strings.TrimSpace(match[1])
			if isValidEndpoint(value) && !seen[value] {
				seen[value] = true
				endpoints = append(endpoints, JSLuiceEndpoint{
					Endpoint: value,
					Type:     classifyEndpoint(value),
				})
			}
		}
	}

	// Also apply regex patterns for better coverage
	for _, urlEndpoint := range extractFromRegex(data, seen) {
		if !seen[urlEndpoint.Endpoint] {
			seen[urlEndpoint.Endpoint] = true
			endpoints = append(endpoints, urlEndpoint)
		}
	}

	return endpoints
}

// extractFromRegex extracts endpoints using regex patterns
func extractFromRegex(data string, seen map[string]bool) []JSLuiceEndpoint {
	var endpoints []JSLuiceEndpoint

	for _, pattern := range urlPatterns {
		matches := pattern.FindAllStringSubmatch(data, -1)
		for _, match := range matches {
			if len(match) > 0 {
				value := strings.TrimSpace(match[0])
				// Clean up quotes if present
				value = strings.Trim(value, "\"'` ,;=")

				if isValidEndpoint(value) && !seen[value] {
					seen[value] = true
					endpoints = append(endpoints, JSLuiceEndpoint{
						Endpoint: value,
						Type:     classifyEndpoint(value),
					})
				}
			}
		}
	}

	return endpoints
}

// isValidEndpoint checks if a string is a valid endpoint
func isValidEndpoint(s string) bool {
	if len(s) < 2 {
		return false
	}

	// Must start with / or http
	if !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") && !strings.HasPrefix(s, "//") {
		return false
	}

	// Avoid common false positives
	if strings.Contains(s, " ") || strings.Contains(s, "\n") || strings.Contains(s, "\t") {
		return false
	}

	// Too short paths
	if strings.HasPrefix(s, "/") && len(s) < 2 {
		return false
	}

	return true
}

// classifyEndpoint classifies the type of endpoint
func classifyEndpoint(s string) string {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return "url"
	}
	if strings.HasPrefix(s, "//") {
		return "protocol-relative"
	}
	if strings.Contains(s, "/api/") {
		return "api"
	}
	if strings.HasPrefix(s, "/") {
		return "path"
	}
	return "unknown"
}
