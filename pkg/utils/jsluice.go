package utils

import (
	"regexp"
	"strings"

	"github.com/projectdiscovery/katana/pkg/utils/jsextract"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex         = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	// commonJSLibraryPaths contains directory patterns that indicate library code.
	commonJSLibraryPaths = []string{
		"node_modules/",
		"vendor/",
		"/vendor/",
		"/node_modules/",
	}
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	// Check for common library directories in the path
	for _, libPath := range commonJSLibraryPaths {
		if strings.Contains(path, libPath) {
			return true
		}
	}
	// Check filename against known library patterns
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

// JSLuiceEndpoint represents an extracted endpoint from JavaScript.
type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript code using AST parsing.
//
// This function uses goja to parse JavaScript and extract endpoints, providing
// cross-platform compatibility without CGO dependencies.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	endpoints := jsextract.ExtractURLs(data)

	result := make([]JSLuiceEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		result = append(result, JSLuiceEndpoint{
			Endpoint: ep.URL,
			Type:     ep.Type,
		})
	}
	return result
}
