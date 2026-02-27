package utils

import (
	"regexp"
	"strings"

	"github.com/dop251/goja/ast"
	jsparser "github.com/dop251/goja/parser"
)

var (
	// urlLikeStringRegex matches URL-like strings including absolute URLs and paths.
	urlLikeStringRegex = regexp.MustCompile(`^(?:https?://[^\s'"` + "`" + `]+|//[^\s'"` + "`" + `]+|\.{0,2}/[a-zA-Z0-9_\-./]+(?:\?[^\s'"` + "`" + `]*)?)$`)

	// fallbackURLPatterns are compiled regexes used for regex-based fallback extraction
	// when AST parsing fails.
	fallbackURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:"|'|` + "`" + `)(https?://[^\s'"` + "`" + `]{5,})(?:"|'|` + "`" + `)`),
		regexp.MustCompile(`(?:"|'|` + "`" + `)(//[^\s'"` + "`" + `]{5,})(?:"|'|` + "`" + `)`),
		regexp.MustCompile(`(?:"|'|` + "`" + `)((?:\.{0,2}/)[a-zA-Z0-9_\-./]{3,}(?:\?[^\s'"` + "`" + `]*)?)(?:"|'|` + "`" + `)`),
		regexp.MustCompile(`(?:"|'|` + "`" + `)(/[a-zA-Z0-9_\-]+(?:/[a-zA-Z0-9_\-./]+)+)(?:"|'|` + "`" + `)`),
	}

	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	// es6ImportExportRegex matches ES6 import/export statements for preprocessing.
	es6ImportExportRegex = regexp.MustCompile(`(?m)^\s*(?:import\s+.*?(?:from\s+)?['"].*?['"];?\s*$|export\s+(?:default\s+)?(?:function|class|const|let|var|async)\s|export\s*\{[^}]*\}\s*(?:from\s*['"][^'"]*['"])?\s*;?\s*$|export\s*\*\s*from\s*['"][^'"]*['"];?\s*$)`)
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

// JSLuiceEndpoint represents an extracted endpoint from JavaScript code.
type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// endpointItem is used internally during AST extraction.
type endpointItem struct {
	value   string
	context string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript code using
// goja's pure-Go AST parser with a regex fallback for malformed JS.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	// Try AST-based extraction first
	endpoints := extractWithAST(data)
	if endpoints != nil {
		return endpoints
	}
	// If AST parsing fails, try with ES6 preprocessing
	preprocessed := preprocessES6(data)
	if preprocessed != data {
		endpoints = extractWithAST(preprocessed)
		if endpoints != nil {
			return endpoints
		}
	}
	// Fall back to regex extraction
	return regexFallbackExtract(data)
}

// preprocessES6 strips ES6 import/export statements that goja's ES5.1
// parser cannot handle.
func preprocessES6(data string) string {
	return es6ImportExportRegex.ReplaceAllString(data, "")
}

// extractWithAST parses JavaScript with goja and walks the AST to find endpoints.
func extractWithAST(data string) []JSLuiceEndpoint {
	program, err := jsparser.ParseFile(nil, "", data, 0)
	if err != nil {
		return nil
	}

	var items []endpointItem
	emit := func(item endpointItem) {
		items = append(items, item)
	}

	for _, stmt := range program.Body {
		walkStatement(stmt, emit)
	}

	return deduplicateEndpoints(items)
}

// walkStatement traverses AST statement nodes.
func walkStatement(stmt ast.Statement, emit func(endpointItem)) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.BlockStatement:
		for _, item := range s.List {
			walkStatement(item, emit)
		}
	case *ast.ExpressionStatement:
		walkExpression(s.Expression, emit)
	case *ast.VariableStatement:
		for _, binding := range s.List {
			walkExpression(binding.Initializer, emit)
			walkBindingTarget(binding.Target, emit)
		}
	case *ast.LexicalDeclaration:
		for _, binding := range s.List {
			walkExpression(binding.Initializer, emit)
			walkBindingTarget(binding.Target, emit)
		}
	case *ast.ReturnStatement:
		walkExpression(s.Argument, emit)
	case *ast.IfStatement:
		walkExpression(s.Test, emit)
		walkStatement(s.Consequent, emit)
		walkStatement(s.Alternate, emit)
	case *ast.ForStatement:
		walkForInitializer(s.Initializer, emit)
		walkExpression(s.Test, emit)
		walkExpression(s.Update, emit)
		walkStatement(s.Body, emit)
	case *ast.ForInStatement:
		walkExpression(s.Source, emit)
		walkStatement(s.Body, emit)
	case *ast.ForOfStatement:
		walkExpression(s.Source, emit)
		walkStatement(s.Body, emit)
	case *ast.SwitchStatement:
		walkExpression(s.Discriminant, emit)
		for _, c := range s.Body {
			walkExpression(c.Test, emit)
			for _, cs := range c.Consequent {
				walkStatement(cs, emit)
			}
		}
	case *ast.TryStatement:
		walkStatement(s.Body, emit)
		if s.Catch != nil {
			walkStatement(s.Catch.Body, emit)
		}
		walkStatement(s.Finally, emit)
	case *ast.WhileStatement:
		walkExpression(s.Test, emit)
		walkStatement(s.Body, emit)
	case *ast.DoWhileStatement:
		walkExpression(s.Test, emit)
		walkStatement(s.Body, emit)
	case *ast.ThrowStatement:
		walkExpression(s.Argument, emit)
	case *ast.FunctionDeclaration:
		if s.Function != nil {
			walkFunctionLiteral(s.Function, emit)
		}
	case *ast.ClassDeclaration:
		if s.Class != nil {
			walkClassLiteral(s.Class, emit)
		}
	case *ast.LabelledStatement:
		walkStatement(s.Statement, emit)
	case *ast.WithStatement:
		walkExpression(s.Object, emit)
		walkStatement(s.Body, emit)
	}
}

// walkForInitializer traverses the for-loop initializer which is its own interface type.
func walkForInitializer(init ast.ForLoopInitializer, emit func(endpointItem)) {
	if init == nil {
		return
	}
	switch fi := init.(type) {
	case *ast.ForLoopInitializerExpression:
		walkExpression(fi.Expression, emit)
	case *ast.ForLoopInitializerVarDeclList:
		for _, binding := range fi.List {
			walkExpression(binding.Initializer, emit)
		}
	case *ast.ForLoopInitializerLexicalDecl:
		for _, binding := range fi.LexicalDeclaration.List {
			walkExpression(binding.Initializer, emit)
		}
	}
}

// walkBindingTarget traverses binding targets (destructuring patterns).
func walkBindingTarget(target ast.BindingTarget, emit func(endpointItem)) {
	if target == nil {
		return
	}
	switch t := target.(type) {
	case *ast.ObjectPattern:
		for _, prop := range t.Properties {
			walkProperty(prop, emit)
		}
	case *ast.ArrayPattern:
		for _, elem := range t.Elements {
			walkExpression(elem, emit)
		}
	}
}

// walkProperty traverses a Property interface value.
func walkProperty(prop ast.Property, emit func(endpointItem)) {
	if prop == nil {
		return
	}
	switch p := prop.(type) {
	case *ast.PropertyKeyed:
		walkExpression(p.Key, emit)
		walkExpression(p.Value, emit)
	case *ast.PropertyShort:
		walkExpression(p.Initializer, emit)
	case *ast.SpreadElement:
		walkExpression(p.Expression, emit)
	}
}

// walkExpression traverses AST expression nodes, emitting URL-like strings.
func walkExpression(expr ast.Expression, emit func(endpointItem)) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.StringLiteral:
		checkAndEmitURL(e.Value.String(), "string", emit)
	case *ast.TemplateLiteral:
		// Walk the dynamic expressions
		for _, texpr := range e.Expressions {
			walkExpression(texpr, emit)
		}
		// Check the static template parts
		for _, elem := range e.Elements {
			if elem.Parsed.String() != "" {
				checkAndEmitURL(elem.Parsed.String(), "template", emit)
			}
		}
	case *ast.BinaryExpression:
		walkExpression(e.Left, emit)
		walkExpression(e.Right, emit)
	case *ast.AssignExpression:
		walkExpression(e.Left, emit)
		walkExpression(e.Right, emit)
		// Detect assignments like location.href = "..."
		if dot, ok := e.Left.(*ast.DotExpression); ok {
			classifyAssignment(dot, e.Right, emit)
		}
	case *ast.CallExpression:
		walkCallExpression(e, emit)
	case *ast.NewExpression:
		walkNewExpression(e, emit)
	case *ast.ObjectLiteral:
		for _, prop := range e.Value {
			walkProperty(prop, emit)
		}
	case *ast.ArrayLiteral:
		for _, item := range e.Value {
			walkExpression(item, emit)
		}
	case *ast.ConditionalExpression:
		walkExpression(e.Test, emit)
		walkExpression(e.Consequent, emit)
		walkExpression(e.Alternate, emit)
	case *ast.DotExpression:
		walkExpression(e.Left, emit)
	case *ast.BracketExpression:
		walkExpression(e.Left, emit)
		walkExpression(e.Member, emit)
	case *ast.UnaryExpression:
		walkExpression(e.Operand, emit)
	case *ast.SequenceExpression:
		for _, item := range e.Sequence {
			walkExpression(item, emit)
		}
	case *ast.ArrowFunctionLiteral:
		walkConciseBody(e.Body, emit)
		if e.ParameterList != nil {
			for _, p := range e.ParameterList.List {
				walkExpression(p.Initializer, emit)
			}
		}
	case *ast.FunctionLiteral:
		walkFunctionLiteral(e, emit)
	case *ast.ClassLiteral:
		walkClassLiteral(e, emit)
	case *ast.SpreadElement:
		walkExpression(e.Expression, emit)
	case *ast.YieldExpression:
		walkExpression(e.Argument, emit)
	case *ast.AwaitExpression:
		walkExpression(e.Argument, emit)
	case *ast.OptionalChain:
		walkExpression(e.Expression, emit)
	case *ast.Optional:
		walkExpression(e.Expression, emit)
	case *ast.Binding:
		walkExpression(e.Initializer, emit)
		walkBindingTarget(e.Target, emit)
	}
}

// walkConciseBody traverses an arrow function's body which can be a
// block statement or a single expression.
func walkConciseBody(body ast.ConciseBody, emit func(endpointItem)) {
	if body == nil {
		return
	}
	switch b := body.(type) {
	case *ast.BlockStatement:
		walkStatement(b, emit)
	case *ast.ExpressionBody:
		walkExpression(b.Expression, emit)
	}
}

// walkFunctionLiteral traverses a function literal node.
func walkFunctionLiteral(fn *ast.FunctionLiteral, emit func(endpointItem)) {
	if fn == nil {
		return
	}
	walkStatement(fn.Body, emit)
	if fn.ParameterList != nil {
		for _, p := range fn.ParameterList.List {
			walkExpression(p.Initializer, emit)
		}
	}
}

// walkClassLiteral traverses a class literal node.
func walkClassLiteral(cls *ast.ClassLiteral, emit func(endpointItem)) {
	if cls == nil {
		return
	}
	for _, element := range cls.Body {
		switch ce := element.(type) {
		case *ast.ClassStaticBlock:
			walkStatement(ce.Block, emit)
		case *ast.FieldDefinition:
			walkExpression(ce.Key, emit)
			walkExpression(ce.Initializer, emit)
		case *ast.MethodDefinition:
			walkExpression(ce.Key, emit)
			if ce.Body != nil {
				walkStatement(ce.Body.Body, emit)
				if ce.Body.ParameterList != nil {
					for _, p := range ce.Body.ParameterList.List {
						walkExpression(p.Initializer, emit)
					}
				}
			}
		}
	}
}

// walkCallExpression handles function calls, classifying common patterns
// like fetch(), XMLHttpRequest.open(), $.ajax(), axios(), etc.
func walkCallExpression(call *ast.CallExpression, emit func(endpointItem)) {
	if call == nil {
		return
	}

	// Walk callee
	walkExpression(call.Callee, emit)

	// Walk arguments
	for _, arg := range call.ArgumentList {
		walkExpression(arg, emit)
	}

	// Classify the call and extract first string argument
	funcName := resolveFunctionName(call.Callee)
	callType := classifyCallType(funcName)
	if callType == "" {
		return
	}

	// Extract the URL argument (usually the first string argument)
	for _, arg := range call.ArgumentList {
		if str, ok := arg.(*ast.StringLiteral); ok {
			val := str.Value.String()
			if isURLLike(val) {
				emit(endpointItem{value: val, context: callType})
			}
			break
		}
		if tmpl, ok := arg.(*ast.TemplateLiteral); ok {
			for _, elem := range tmpl.Elements {
				if elem.Parsed.String() != "" && isURLLike(elem.Parsed.String()) {
					emit(endpointItem{value: elem.Parsed.String(), context: callType})
				}
			}
			break
		}
	}
}

// walkNewExpression handles new expressions like new WebSocket(), new URL(),
// new Request(), etc.
func walkNewExpression(ne *ast.NewExpression, emit func(endpointItem)) {
	if ne == nil {
		return
	}

	walkExpression(ne.Callee, emit)
	for _, arg := range ne.ArgumentList {
		walkExpression(arg, emit)
	}

	funcName := resolveFunctionName(ne.Callee)
	lower := strings.ToLower(funcName)

	var constructorType string
	switch {
	case lower == "websocket" || strings.HasSuffix(lower, ".websocket"):
		constructorType = "websocket"
	case lower == "url" || strings.HasSuffix(lower, ".url"):
		constructorType = "url_constructor"
	case lower == "request" || strings.HasSuffix(lower, ".request"):
		constructorType = "request"
	case lower == "eventsource" || strings.HasSuffix(lower, ".eventsource"):
		constructorType = "eventsource"
	default:
		return
	}

	for _, arg := range ne.ArgumentList {
		if str, ok := arg.(*ast.StringLiteral); ok {
			val := str.Value.String()
			if isURLLike(val) {
				emit(endpointItem{value: val, context: constructorType})
			}
			break
		}
	}
}

// resolveFunctionName extracts the function name from a callee expression.
func resolveFunctionName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Name.String()
	case *ast.DotExpression:
		obj := resolveFunctionName(e.Left)
		if obj != "" {
			return obj + "." + e.Identifier.Name.String()
		}
		return e.Identifier.Name.String()
	case *ast.OptionalChain:
		return resolveFunctionName(e.Expression)
	case *ast.Optional:
		return resolveFunctionName(e.Expression)
	}
	return ""
}

// classifyCallType determines the endpoint type from a function name.
func classifyCallType(funcName string) string {
	if funcName == "" {
		return ""
	}
	lower := strings.ToLower(funcName)

	switch {
	case lower == "fetch" || strings.HasSuffix(lower, ".fetch"):
		return "fetch"
	case strings.Contains(lower, "window.open"):
		return "window_open"
	case isXHROpen(funcName):
		return "xhr"
	case lower == "$.ajax" || lower == "jquery.ajax" || strings.HasSuffix(lower, ".ajax"):
		return "ajax"
	case lower == "$.get" || lower == "$.post" || lower == "$.getjson":
		return "ajax"
	case strings.Contains(lower, "axios") || strings.HasSuffix(lower, ".get") ||
		strings.HasSuffix(lower, ".post") || strings.HasSuffix(lower, ".put") ||
		strings.HasSuffix(lower, ".delete") || strings.HasSuffix(lower, ".patch"):
		return "http_client"
	case lower == "require" || lower == "import":
		return "import"
	case strings.Contains(lower, "redirect") || strings.Contains(lower, "navigate"):
		return "navigation"
	case strings.Contains(lower, "sendbeacon") || strings.HasSuffix(lower, ".sendbeacon"):
		return "beacon"
	}
	return ""
}

// isXHROpen checks if a function name represents an XMLHttpRequest.open() call.
func isXHROpen(funcName string) bool {
	lower := strings.ToLower(funcName)
	return strings.HasSuffix(lower, ".open") && lower != "window.open"
}

// classifyAssignment handles property assignments that indicate URL usage.
func classifyAssignment(dot *ast.DotExpression, right ast.Expression, emit func(endpointItem)) {
	propName := strings.ToLower(dot.Identifier.Name.String())
	var assignType string

	switch propName {
	case "href", "src", "action", "data", "url", "baseurl", "endpoint":
		assignType = "property_assignment"
	default:
		return
	}

	if str, ok := right.(*ast.StringLiteral); ok {
		val := str.Value.String()
		if isURLLike(val) {
			emit(endpointItem{value: val, context: assignType})
		}
	}
}

// checkAndEmitURL checks if a string is URL-like and emits it.
func checkAndEmitURL(value, context string, emit func(endpointItem)) {
	if isURLLike(value) {
		emit(endpointItem{value: value, context: context})
	}
}

// isURLLike determines if a string looks like a URL or path.
func isURLLike(s string) bool {
	if len(s) < 2 || len(s) > 2048 {
		return false
	}
	return urlLikeStringRegex.MatchString(s)
}

// deduplicateEndpoints removes duplicate endpoints and converts to JSLuiceEndpoint.
func deduplicateEndpoints(items []endpointItem) []JSLuiceEndpoint {
	seen := make(map[string]struct{})
	var endpoints []JSLuiceEndpoint

	for _, item := range items {
		if _, ok := seen[item.value]; ok {
			continue
		}
		seen[item.value] = struct{}{}
		endpoints = append(endpoints, JSLuiceEndpoint{
			Endpoint: item.value,
			Type:     item.context,
		})
	}
	return endpoints
}

// regexFallbackExtract extracts endpoints using regex when AST parsing fails.
func regexFallbackExtract(data string) []JSLuiceEndpoint {
	seen := make(map[string]struct{})
	var endpoints []JSLuiceEndpoint

	for _, pattern := range fallbackURLPatterns {
		matches := pattern.FindAllStringSubmatch(data, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			url := match[1]
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			endpoints = append(endpoints, JSLuiceEndpoint{
				Endpoint: url,
				Type:     "regex",
			})
		}
	}

	return endpoints
}
