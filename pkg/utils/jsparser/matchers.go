package jsparser

import (
	"path"
	"regexp"
	"strings"
)

var (
	CommonJSLibraryFileRegex = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	schemeLikeRegex    = regexp.MustCompile(`(?i)^(?:https?|wss?|ftp)://`)
	domainLikeRegex    = regexp.MustCompile(`(?i)^(?:[a-z0-9-]+\.)+[a-z]{2,}(?::\d{1,5})?(?:/.*)?$`)
	pathLikeRegex      = regexp.MustCompile(`(?i)^(?:/?|\.{1,2}/)[a-z0-9_~%!$&'()*+,;=:@.-]+(?:/[a-z0-9_~%!$&'()*+,;=:@.-]*)*(?:\?[^\s#]*)?(?:#[^\s]*)?$`)
	barePathLikeRegex  = regexp.MustCompile(`(?i)^[a-z0-9_-]+(?:/[a-z0-9_~%!$&'()*+,;=:@.-]+)+(?:\?[^\s#]*)?(?:#[^\s]*)?$`)
	fileLikeRegex      = regexp.MustCompile(`(?i)^[a-z0-9_.-]+\.(?:json|js|php|asp|aspx|jsp|action|do|cgi|xml|txt)(?:\?[^\s#]*)?(?:#[^\s]*)?$`)
	apiKeywordRegex    = regexp.MustCompile(`(?i)(?:^|/)(?:api|rest|graphql|gql|v1|v2|v3|oauth|auth|login|logout|token|session|user|users|account|accounts|admin|search|query|gateway|internal)(?:$|/)`)
	invalidPrefixRegex = regexp.MustCompile(`(?i)^(?:data|mailto|javascript|vbscript|about|chrome|blob):`)
)

func IsPathCommonJSLibraryFile(value string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(strings.TrimSpace(value))
}

func IsLikelyEndpoint(value string) bool {
	value = NormalizeEndpoint(value)
	if value == "" {
		return false
	}

	lower := strings.ToLower(value)
	switch {
	case value == "#":
		return false
	case invalidPrefixRegex.MatchString(lower):
		return false
	case strings.ContainsAny(value, " \n\r\t"):
		return false
	case len(value) < 2:
		return false
	}

	switch {
	case schemeLikeRegex.MatchString(value):
		return true
	case strings.HasPrefix(value, "//"):
		return true
	case strings.HasPrefix(value, "/"), strings.HasPrefix(value, "./"), strings.HasPrefix(value, "../"):
		return true
	case domainLikeRegex.MatchString(value):
		return true
	case apiKeywordRegex.MatchString(value) && (pathLikeRegex.MatchString(value) || barePathLikeRegex.MatchString(value) || fileLikeRegex.MatchString(value)):
		return true
	case barePathLikeRegex.MatchString(value):
		return true
	case fileLikeRegex.MatchString(value):
		return true
	default:
		return false
	}
}

func ClassifyEndpoint(value string) string {
	value = NormalizeEndpoint(value)
	lower := strings.ToLower(value)

	switch {
	case strings.HasPrefix(lower, "ws://"), strings.HasPrefix(lower, "wss://"):
		return "websocket"
	case schemeLikeRegex.MatchString(lower), strings.HasPrefix(value, "//"), domainLikeRegex.MatchString(value):
		return "url"
	case strings.Contains(value, "?"), strings.Contains(value, "#"):
		return "path"
	case strings.HasPrefix(value, "/"), strings.HasPrefix(value, "./"), strings.HasPrefix(value, "../"):
		return "path"
	case strings.Contains(value, "/"), fileLikeRegex.MatchString(value), apiKeywordRegex.MatchString(value):
		return "path"
	default:
		return "candidate"
	}
}

func NormalizeEndpoint(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	value = strings.Trim(value, "`")
	value = strings.TrimSuffix(value, "\\n")
	value = strings.TrimSuffix(value, "\\r")
	value = strings.TrimSuffix(value, "\\t")

	for {
		trimmed := strings.TrimRight(value, ")}]>;,")
		if trimmed == value {
			break
		}
		value = trimmed
	}

	value = strings.TrimSpace(value)
	return value
}

func DedupeEndpoints(in []Endpoint) []Endpoint {
	if len(in) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(in))
	out := make([]Endpoint, 0, len(in))

	for _, item := range in {
		item.Endpoint = NormalizeEndpoint(item.Endpoint)
		if item.Endpoint == "" || !IsLikelyEndpoint(item.Endpoint) {
			continue
		}
		if item.Type == "" {
			item.Type = ClassifyEndpoint(item.Endpoint)
		}

		key := item.Endpoint
		if item.Type != "candidate" {
			key = item.Type + "\x00" + item.Endpoint
		}

		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}

	return out
}

func MergePaths(base, suffix string) string {
	base = NormalizeEndpoint(base)
	suffix = NormalizeEndpoint(suffix)

	switch {
	case base == "":
		return suffix
	case suffix == "":
		return base
	case strings.HasSuffix(base, "/") || strings.HasPrefix(suffix, "/"):
		return base + suffix
	default:
		return path.Clean(base + "/" + suffix)
	}
}

