package utils

import (
	"regexp"
	"strings"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex         = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

// JSLuiceEndpoint represents a URL endpoint extracted from JavaScript source.
type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// JS URL extraction patterns for common API call sites.
// \x60 represents the backtick character used in template literals.
var (
	jsFetchPattern           = regexp.MustCompile("\\bfetch\\s*\\(\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsXHROpenPattern         = regexp.MustCompile("\\.open\\s*\\(\\s*[\"'][A-Za-z]+[\"']\\s*,\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsLocationPattern        = regexp.MustCompile("(?:location(?:\\.href)?\\s*=|window\\.location\\s*=)\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsLocationReplacePattern = regexp.MustCompile("location\\.replace\\s*\\(\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsLocationAssignPattern  = regexp.MustCompile("location\\.assign\\s*\\(\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsWindowOpenPattern      = regexp.MustCompile("window\\.open\\s*\\(\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsJQueryGetPattern       = regexp.MustCompile("\\$\\s*\\.\\s*get\\s*\\(\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsJQueryPostPattern      = regexp.MustCompile("\\$\\s*\\.\\s*post\\s*\\(\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
	jsJQueryAjaxURLPattern   = regexp.MustCompile("\\$\\s*\\.\\s*ajax\\s*\\([^)]*[\"']url[\"']\\s*:\\s*[\"'\\x60]([^\"'\\x60\\s]+)[\"'\\x60]")
)

// jsURLPatterns maps compiled patterns to their URL type labels.
var jsURLPatterns = []struct {
	re  *regexp.Regexp
	typ string
}{
	{jsFetchPattern, "fetch"},
	{jsXHROpenPattern, "XMLHttpRequest.open"},
	{jsLocationPattern, "location"},
	{jsLocationReplacePattern, "location.replace"},
	{jsLocationAssignPattern, "location.assign"},
	{jsWindowOpenPattern, "window.open"},
	{jsJQueryGetPattern, "jQuery.get"},
	{jsJQueryPostPattern, "jQuery.post"},
	{jsJQueryAjaxURLPattern, "jQuery.ajax"},
}

// stripJSComments returns the source with JS line (//) and block (/* */)
// comments removed. String literals are copied verbatim so that the regex
// patterns applied afterwards can still see their call-site context.
//
// Limitation: JavaScript regex literals are not recognised, so a regex
// containing // or /* (e.g. /https?:\/\//) will be misinterpreted as a
// comment delimiter, potentially stripping code on the same line.
//
// Limitation: Template expression depth tracking (${...}) uses simple brace
// counting and does not account for braces inside nested string literals.
func stripJSComments(data string) string {
	var sb strings.Builder
	sb.Grow(len(data))
	i := 0
	n := len(data)

	for i < n {
		ch := data[i]

		switch {
		case ch == '/' && i+1 < n && data[i+1] == '/':
			// Line comment — skip to end of line
			for i < n && data[i] != '\n' {
				i++
			}

		case ch == '/' && i+1 < n && data[i+1] == '*':
			// Block comment — skip to */
			i += 2
			found := false
			for i+1 < n {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					found = true
					break
				}
				i++
			}
			// If unterminated, consume any remaining character so it is not emitted.
			if !found {
				i = n
			}

		case ch == '\'' || ch == '"' || ch == '`':
			// String literal — copy verbatim (including delimiters) so that
			// call-site regex patterns (e.g. fetch('/url')) still match.
			quote := ch
			sb.WriteByte(ch)
			i++
			for i < n {
				c := data[i]
				if c == '\\' && i+1 < n {
					// Escape sequence — copy both bytes
					sb.WriteByte(c)
					sb.WriteByte(data[i+1])
					i += 2
				} else if c == byte(quote) {
					// Closing quote
					sb.WriteByte(c)
					i++
					break
				} else if quote == '`' && c == '$' && i+1 < n && data[i+1] == '{' {
					// Template expression ${...} — copy as-is
					sb.WriteByte('$')
					sb.WriteByte('{')
					i += 2
					depth := 1
					for i < n && depth > 0 {
						if data[i] == '{' {
							depth++
						} else if data[i] == '}' {
							depth--
						}
						sb.WriteByte(data[i])
						i++
					}
				} else {
					sb.WriteByte(c)
					i++
				}
			}

		default:
			sb.WriteByte(ch)
			i++
		}
	}
	return sb.String()
}

// mimeTypePrefixes lists the standard IANA top-level media type prefixes.
// Used to reject MIME values (e.g. "application/json") that would otherwise
// pass the plain-relative-path heuristic in isLikelyURL.
var mimeTypePrefixes = []string{
	"application/",
	"text/",
	"image/",
	"audio/",
	"video/",
	"font/",
	"multipart/",
	"message/",
	"model/",
	"chemical/",
	"x-conference/",
}

// isMIMEType returns true if s looks like a MIME type value (e.g.
// "application/json", "text/html; charset=utf-8"). It checks for a known
// top-level type prefix and ensures the value contains exactly one slash
// before any parameters (semicolon-delimited).
func isMIMEType(s string) bool {
	// Work on the part before any parameters (e.g. "; charset=utf-8").
	base := s
	if idx := strings.IndexByte(s, ';'); idx >= 0 {
		base = s[:idx]
	}
	base = strings.TrimSpace(base)
	lower := strings.ToLower(base)

	// A MIME type has exactly one slash and no path-like separators.
	if strings.Count(lower, "/") != 1 {
		return false
	}
	// Must not contain dots, query params, or anchors — those indicate
	// file paths or URLs, not MIME types.
	if strings.ContainsAny(lower, ".?&=#") {
		return false
	}
	for _, prefix := range mimeTypePrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// isLikelyURL returns true if the string looks like a URL or path worth
// reporting as an endpoint.
func isLikelyURL(s string) bool {
	if len(s) < 2 || len(s) > 2048 {
		return false
	}
	// Skip w3.org schema/namespace strings (common noise in JS)
	if strings.Contains(s, "w3.org") {
		return false
	}
	// Skip data URIs
	if strings.HasPrefix(s, "data:") {
		return false
	}
	// Absolute URLs (http://, https://, ftp://, etc.)
	if strings.Contains(s, "://") {
		return true
	}
	// Protocol-relative URLs
	if strings.HasPrefix(s, "//") {
		return true
	}
	// Absolute paths — must have at least one char after the leading /
	if strings.HasPrefix(s, "/") && len(s) > 1 && s[1] != ' ' {
		return true
	}
	// Relative paths
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}
	// Plain relative paths (e.g. "api/v1/users") — must contain a slash,
	// no whitespace, and not start with a fragment or query marker.
	if strings.Contains(s, "/") &&
		!strings.ContainsAny(s, " \t\r\n") &&
		!strings.HasPrefix(s, "#") &&
		!strings.HasPrefix(s, "?") {
		// Reject MIME type values (e.g. "application/json", "text/plain").
		// These are common in JS headers/options and should not be treated
		// as URL endpoints.
		if isMIMEType(s) {
			return false
		}
		return true
	}
	return false
}

// extractJSStrings extracts string literal values from (comment-stripped)
// JavaScript source. It handles single-quoted, double-quoted, and template
// literal strings and replaces template expressions (${...}) with "EXPR".
func extractJSStrings(data string) []string {
	var results []string
	i := 0
	n := len(data)

	for i < n {
		ch := data[i]

		if ch != '\'' && ch != '"' && ch != '`' {
			i++
			continue
		}

		// String literal
		quote := ch
		i++
		var sb strings.Builder
		for i < n {
			c := data[i]
			if c == '\\' && i+1 < n {
				i++
				switch data[i] {
				case 'n':
					sb.WriteByte('\n')
				case 't':
					sb.WriteByte('\t')
				case 'r':
					sb.WriteByte('\r')
				case '\\':
					sb.WriteByte('\\')
				case '\'':
					sb.WriteByte('\'')
				case '"':
					sb.WriteByte('"')
				case '`':
					sb.WriteByte('`')
				case '/':
					sb.WriteByte('/')
				default:
					sb.WriteByte(data[i])
				}
				i++
			} else if c == byte(quote) {
				i++
				break
			} else if quote == '`' && c == '$' && i+1 < n && data[i+1] == '{' {
				// Template expression — replace with placeholder
				sb.WriteString("EXPR")
				i += 2
				depth := 1
				for i < n && depth > 0 {
					if data[i] == '{' {
						depth++
					} else if data[i] == '}' {
						depth--
					}
					i++
				}
			} else {
				sb.WriteByte(c)
				i++
			}
		}
		if s := sb.String(); s != "" {
			results = append(results, s)
		}
	}
	return results
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript content using
// pure-Go parsing. It first strips JS comments, then applies pattern matching
// for common API call sites (fetch, XHR, jQuery, location, window.open) and
// falls back to generic string literal extraction filtered by URL heuristics.
//
// Comment stripping ensures URLs inside commented-out code are not reported,
// matching the behavior of the previous AST-based implementation.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	// Strip comments first so that neither the regex patterns nor the string
	// extractor pick up URLs that appear only inside comments.
	cleaned := stripJSComments(data)

	var endpoints []JSLuiceEndpoint
	seen := make(map[string]bool)

	add := func(url, typ string) {
		if url == "" || seen[url] || !isLikelyURL(url) {
			return
		}
		seen[url] = true
		endpoints = append(endpoints, JSLuiceEndpoint{Endpoint: url, Type: typ})
	}

	// 1. Pattern-based extraction for known API call sites
	for _, p := range jsURLPatterns {
		for _, m := range p.re.FindAllStringSubmatch(cleaned, -1) {
			if len(m) > 1 {
				add(m[1], p.typ)
			}
		}
	}

	// 2. Generic string literal extraction — catches any remaining URL strings.
	// Skip strings containing "EXPR" since those are template literal artifacts
	// (e.g. "/api/usersEXPR") that were already captured with the original
	// ${...} syntax by the pattern pass above.
	for _, s := range extractJSStrings(cleaned) {
		if strings.Contains(s, "EXPR") {
			continue
		}
		add(s, "string")
	}

	return endpoints
}
