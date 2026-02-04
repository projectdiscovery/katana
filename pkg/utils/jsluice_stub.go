//go:build windows || 386 || nocgo

package utils

import (
	"regexp"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	// urlPathRegex matches URL-like patterns in strings
	urlPathRegex = regexp.MustCompile(`^(?:https?://[^\s"'` + "`" + `]+|/[a-zA-Z0-9_\-./]+(?:\?[^\s"'` + "`" + `]*)?|\.{1,2}/[a-zA-Z0-9_\-./]+)$`)

	// apiEndpointRegex matches common API endpoint patterns
	apiEndpointRegex = regexp.MustCompile(`(?i)^(?:/api/|/v[0-9]+/|/graphql|/rest/|/ajax/|/json/|/data/)`)
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript source code using
// pure-Go AST parsing (goja). This is a CGO-free alternative to the tree-sitter
// based jsluice library.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	extractor := newEndpointExtractor()
	return extractor.Extract(data)
}

// endpointExtractor walks JavaScript AST to find URLs and endpoints
type endpointExtractor struct {
	endpoints map[string]JSLuiceEndpoint // dedupe by endpoint
}

func newEndpointExtractor() *endpointExtractor {
	return &endpointExtractor{
		endpoints: make(map[string]JSLuiceEndpoint),
	}
}

func (e *endpointExtractor) Extract(source string) []JSLuiceEndpoint {
	program, err := parser.ParseFile(nil, "", source, 0)
	if err != nil {
		// If parsing fails, fall back to regex extraction
		return e.extractWithRegex(source)
	}

	e.walkProgram(program)

	result := make([]JSLuiceEndpoint, 0, len(e.endpoints))
	for _, ep := range e.endpoints {
		result = append(result, ep)
	}
	return result
}

func (e *endpointExtractor) walkProgram(program *ast.Program) {
	for _, stmt := range program.Body {
		e.walkStatement(stmt)
	}
}

func (e *endpointExtractor) walkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		e.walkExpression(s.Expression)
	case *ast.BlockStatement:
		for _, st := range s.List {
			e.walkStatement(st)
		}
	case *ast.IfStatement:
		e.walkExpression(s.Test)
		e.walkStatement(s.Consequent)
		e.walkStatement(s.Alternate)
	case *ast.ForStatement:
		e.walkStatement(s.Body)
	case *ast.ForInStatement:
		e.walkStatement(s.Body)
	case *ast.ForOfStatement:
		e.walkStatement(s.Body)
	case *ast.WhileStatement:
		e.walkStatement(s.Body)
	case *ast.DoWhileStatement:
		e.walkStatement(s.Body)
	case *ast.TryStatement:
		e.walkStatement(s.Body)
		if s.Catch != nil {
			e.walkStatement(s.Catch.Body)
		}
		e.walkStatement(s.Finally)
	case *ast.SwitchStatement:
		for _, c := range s.Body {
			for _, st := range c.Consequent {
				e.walkStatement(st)
			}
		}
	case *ast.FunctionDeclaration:
		if s.Function != nil {
			e.walkStatement(s.Function.Body)
		}
	case *ast.VariableStatement:
		for _, binding := range s.List {
			e.walkExpression(binding.Initializer)
		}
	case *ast.LexicalDeclaration:
		for _, binding := range s.List {
			e.walkExpression(binding.Initializer)
		}
	case *ast.ReturnStatement:
		e.walkExpression(s.Argument)
	case *ast.ThrowStatement:
		e.walkExpression(s.Argument)
	case *ast.ClassDeclaration:
		if s.Class != nil {
			for _, elem := range s.Class.Body {
				e.walkClassElement(elem)
			}
		}
	}
}

func (e *endpointExtractor) walkClassElement(elem ast.ClassElement) {
	switch el := elem.(type) {
	case *ast.MethodDefinition:
		if el.Body != nil {
			e.walkStatement(el.Body.Body)
		}
	case *ast.FieldDefinition:
		e.walkExpression(el.Initializer)
	}
}

func (e *endpointExtractor) walkExpression(expr ast.Expression) {
	if expr == nil {
		return
	}

	switch ex := expr.(type) {
	case *ast.StringLiteral:
		e.checkStringLiteral(ex.Value.String(), "stringLiteral")

	case *ast.TemplateLiteral:
		// Extract from template literal parts
		for _, elem := range ex.Elements {
			e.checkStringLiteral(elem.Parsed.String(), "templateLiteral")
		}

	case *ast.CallExpression:
		e.handleCallExpression(ex)

	case *ast.AssignExpression:
		e.handleAssignExpression(ex)
		e.walkExpression(ex.Right)

	case *ast.BinaryExpression:
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Right)

	case *ast.ObjectLiteral:
		for _, prop := range ex.Value {
			switch p := prop.(type) {
			case *ast.PropertyKeyed:
				e.walkExpression(p.Value)
			}
		}

	case *ast.ArrayLiteral:
		for _, elem := range ex.Value {
			e.walkExpression(elem)
		}

	case *ast.FunctionLiteral:
		if ex.Body != nil {
			e.walkStatement(ex.Body)
		}

	case *ast.ArrowFunctionLiteral:
		switch body := ex.Body.(type) {
		case *ast.BlockStatement:
			e.walkStatement(body)
		case *ast.ExpressionBody:
			e.walkExpression(body.Expression)
		}

	case *ast.ConditionalExpression:
		e.walkExpression(ex.Test)
		e.walkExpression(ex.Consequent)
		e.walkExpression(ex.Alternate)

	case *ast.UnaryExpression:
		e.walkExpression(ex.Operand)

	case *ast.NewExpression:
		e.walkExpression(ex.Callee)
		for _, arg := range ex.ArgumentList {
			e.walkExpression(arg)
		}

	case *ast.SequenceExpression:
		for _, seq := range ex.Sequence {
			e.walkExpression(seq)
		}

	case *ast.DotExpression:
		e.walkExpression(ex.Left)

	case *ast.BracketExpression:
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Member)
	}
}

func (e *endpointExtractor) handleCallExpression(call *ast.CallExpression) {
	// Check callee
	e.walkExpression(call.Callee)

	// Extract function name for type detection
	funcName := e.getFunctionName(call.Callee)

	// Determine endpoint type based on function name
	endpointType := e.getEndpointTypeForFunction(funcName)

	// Check arguments for URL-like strings
	for _, arg := range call.ArgumentList {
		if str, ok := arg.(*ast.StringLiteral); ok {
			e.checkStringLiteral(str.Value.String(), endpointType)
		} else if tpl, ok := arg.(*ast.TemplateLiteral); ok {
			for _, elem := range tpl.Elements {
				e.checkStringLiteral(elem.Parsed.String(), endpointType)
			}
		} else if obj, ok := arg.(*ast.ObjectLiteral); ok {
			// Check for url/URL property in object arguments (common in fetch/axios options)
			e.extractURLFromObject(obj, endpointType)
		} else {
			e.walkExpression(arg)
		}
	}
}

func (e *endpointExtractor) handleAssignExpression(assign *ast.AssignExpression) {
	// Check for location assignments: document.location = "...", window.location.href = "..."
	leftStr := e.expressionToString(assign.Left)
	if strings.Contains(leftStr, "location") || strings.Contains(leftStr, "href") {
		if str, ok := assign.Right.(*ast.StringLiteral); ok {
			e.addEndpoint(str.Value.String(), "locationAssignment")
		}
	}
}

func (e *endpointExtractor) getFunctionName(expr ast.Expression) string {
	switch ex := expr.(type) {
	case *ast.Identifier:
		return ex.Name.String()
	case *ast.DotExpression:
		leftName := e.expressionToString(ex.Left)
		return leftName + "." + ex.Identifier.Name.String()
	}
	return ""
}

func (e *endpointExtractor) expressionToString(expr ast.Expression) string {
	switch ex := expr.(type) {
	case *ast.Identifier:
		return ex.Name.String()
	case *ast.DotExpression:
		return e.expressionToString(ex.Left) + "." + ex.Identifier.Name.String()
	}
	return ""
}

func (e *endpointExtractor) getEndpointTypeForFunction(funcName string) string {
	funcName = strings.ToLower(funcName)

	switch {
	case strings.Contains(funcName, "fetch"):
		return "fetch"
	case strings.Contains(funcName, "open"):
		return "windowOpen"
	case strings.Contains(funcName, "ajax"):
		return "ajax"
	case strings.Contains(funcName, "get") && (strings.Contains(funcName, "$") || strings.Contains(funcName, "http") || strings.Contains(funcName, "axios")):
		return "httpGet"
	case strings.Contains(funcName, "post") && (strings.Contains(funcName, "$") || strings.Contains(funcName, "http") || strings.Contains(funcName, "axios")):
		return "httpPost"
	case strings.Contains(funcName, "request"):
		return "httpRequest"
	case strings.Contains(funcName, "xmlhttprequest"):
		return "xhr"
	case strings.Contains(funcName, "navigate") || strings.Contains(funcName, "redirect"):
		return "navigation"
	case strings.Contains(funcName, "import"):
		return "import"
	case strings.Contains(funcName, "require"):
		return "require"
	case strings.Contains(funcName, "src") || strings.Contains(funcName, "load"):
		return "resourceLoad"
	default:
		return "functionCall"
	}
}

func (e *endpointExtractor) extractURLFromObject(obj *ast.ObjectLiteral, defaultType string) {
	for _, prop := range obj.Value {
		if keyed, ok := prop.(*ast.PropertyKeyed); ok {
			keyName := ""
			if id, ok := keyed.Key.(*ast.Identifier); ok {
				keyName = strings.ToLower(id.Name.String())
			} else if str, ok := keyed.Key.(*ast.StringLiteral); ok {
				keyName = strings.ToLower(str.Value.String())
			}

			if keyName == "url" || keyName == "uri" || keyName == "href" || keyName == "src" || keyName == "endpoint" || keyName == "path" || keyName == "baseurl" {
				if str, ok := keyed.Value.(*ast.StringLiteral); ok {
					e.addEndpoint(str.Value.String(), defaultType)
				}
			}
		}
	}
}

func (e *endpointExtractor) checkStringLiteral(value, endpointType string) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) < 2 {
		return
	}

	// Check if it looks like a URL or path
	if e.isLikelyEndpoint(value) {
		e.addEndpoint(value, endpointType)
	}
}

func (e *endpointExtractor) isLikelyEndpoint(s string) bool {
	// Skip obvious non-URLs
	if strings.HasPrefix(s, "data:") || strings.HasPrefix(s, "javascript:") {
		return false
	}

	// Check for absolute URLs
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "//") {
		return true
	}

	// Check for path patterns
	if strings.HasPrefix(s, "/") && len(s) > 1 {
		// Skip paths that look like regex or unlikely to be endpoints
		if strings.ContainsAny(s, "*+?^${}[]|\\") {
			return false
		}
		return true
	}

	// Check for relative paths
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") {
		return true
	}

	// Check for API-like patterns
	if apiEndpointRegex.MatchString(s) {
		return true
	}

	// Check for file extensions commonly fetched
	for _, ext := range []string{".json", ".xml", ".html", ".js", ".css", ".php", ".asp", ".jsp"} {
		if strings.HasSuffix(strings.ToLower(s), ext) {
			return true
		}
	}

	return false
}

func (e *endpointExtractor) addEndpoint(endpoint, endpointType string) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" || endpoint == "/" {
		return
	}

	// Dedupe
	if _, exists := e.endpoints[endpoint]; !exists {
		e.endpoints[endpoint] = JSLuiceEndpoint{
			Endpoint: endpoint,
			Type:     endpointType,
		}
	}
}

// extractWithRegex is a fallback when AST parsing fails
func (e *endpointExtractor) extractWithRegex(source string) []JSLuiceEndpoint {
	// Common patterns for URLs in JavaScript
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`["'` + "`" + `](https?://[^"'` + "`" + `\s]+)["'` + "`" + `]`),
		regexp.MustCompile(`["'` + "`" + `](/[a-zA-Z0-9_\-./]+(?:\?[^"'` + "`" + `]*)?)["'` + "`" + `]`),
		regexp.MustCompile(`["'` + "`" + `](\.{1,2}/[a-zA-Z0-9_\-./]+)["'` + "`" + `]`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(source, -1)
		for _, match := range matches {
			if len(match) > 1 {
				e.addEndpoint(match[1], "regexMatch")
			}
		}
	}

	result := make([]JSLuiceEndpoint, 0, len(e.endpoints))
	for _, ep := range e.endpoints {
		result = append(result, ep)
	}
	return result
}
