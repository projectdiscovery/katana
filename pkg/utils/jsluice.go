package utils

import (
	"regexp"
	"strings"

	"github.com/dop251/goja/ast"
	jsparser "github.com/dop251/goja/parser"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex         = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	// urlLikeStringRegex matches strings that look like URL paths or full URLs.
	urlLikeStringRegex = regexp.MustCompile(`^(?:https?://[^\s'"` + "`" + `]+|/[a-zA-Z0-9_\-./]+(?:\?[^\s'"` + "`" + `]*)?)$`)
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

// ExtractJsluiceEndpoints extracts endpoints from JavaScript code using
// a pure-Go AST parser (dop251/goja). It parses the JS into an AST and
// walks it to find URL-like string literals in relevant contexts (fetch,
// XMLHttpRequest, window.open, assignments, etc.).
//
// If AST parsing fails (e.g. malformed JS), it falls back to regex-based
// extraction.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	// Preprocess: handle ES6 import/export statements that goja doesn't
	// support natively by stripping them before parsing.
	preprocessed := preprocessES6(data)

	program, err := jsparser.ParseFile(nil, "", preprocessed, 0)
	if err != nil {
		// AST parsing failed — fall back to regex extraction
		return regexFallbackExtract(data)
	}

	seen := make(map[string]struct{})
	var endpoints []JSLuiceEndpoint

	walkAST(program, func(value, kind string) {
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		endpoints = append(endpoints, JSLuiceEndpoint{
			Endpoint: value,
			Type:     kind,
		})
	})

	return endpoints
}

// es6ImportExportRegex matches ES6 import/export lines that goja's parser
// does not support.
var es6ImportExportRegex = regexp.MustCompile(`(?m)^\s*(?:import\s+.*?(?:from\s+)?['"].*?['"]|export\s+(?:default\s+)?(?:function|class|const|let|var|{))[^;]*;?\s*$`)

// preprocessES6 strips ES6 import/export statements to allow goja to parse
// the rest of the file.
func preprocessES6(data string) string {
	return es6ImportExportRegex.ReplaceAllString(data, "")
}

// walkAST traverses the JavaScript AST and calls emit for each URL-like string found.
func walkAST(program *ast.Program, emit func(value, kind string)) {
	for _, stmt := range program.Body {
		walkStatement(stmt, emit)
	}
}

// walkStatement dispatches to the appropriate handler for each statement type.
func walkStatement(stmt ast.Statement, emit func(value, kind string)) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		walkExpression(s.Expression, emit)
	case *ast.VariableStatement:
		for _, binding := range s.List {
			walkExpression(binding.Initializer, emit)
		}
	case *ast.ReturnStatement:
		walkExpression(s.Argument, emit)
	case *ast.BlockStatement:
		for _, inner := range s.List {
			walkStatement(inner, emit)
		}
	case *ast.IfStatement:
		walkExpression(s.Test, emit)
		walkStatement(s.Consequent, emit)
		walkStatement(s.Alternate, emit)
	case *ast.ForStatement:
		walkForLoopInitializer(s.Initializer, emit)
		walkExpression(s.Test, emit)
		walkExpression(s.Update, emit)
		walkStatement(s.Body, emit)
	case *ast.ForInStatement:
		walkExpression(s.Source, emit)
		walkStatement(s.Body, emit)
	case *ast.ForOfStatement:
		walkExpression(s.Source, emit)
		walkStatement(s.Body, emit)
	case *ast.WhileStatement:
		walkExpression(s.Test, emit)
		walkStatement(s.Body, emit)
	case *ast.DoWhileStatement:
		walkExpression(s.Test, emit)
		walkStatement(s.Body, emit)
	case *ast.SwitchStatement:
		walkExpression(s.Discriminant, emit)
		for _, c := range s.Body {
			walkExpression(c.Test, emit)
			for _, inner := range c.Consequent {
				walkStatement(inner, emit)
			}
		}
	case *ast.TryStatement:
		walkStatement(s.Body, emit)
		if s.Catch != nil {
			walkStatement(s.Catch.Body, emit)
		}
		if s.Finally != nil {
			walkStatement(s.Finally, emit)
		}
	case *ast.ThrowStatement:
		walkExpression(s.Argument, emit)
	case *ast.FunctionDeclaration:
		if s.Function != nil {
			walkStatement(s.Function.Body, emit)
			for _, p := range s.Function.ParameterList.List {
				walkExpression(p.Initializer, emit)
			}
		}
	case *ast.ClassDeclaration:
		if s.Class != nil {
			walkClass(s.Class, emit)
		}
	case *ast.LexicalDeclaration:
		for _, binding := range s.List {
			walkExpression(binding.Initializer, emit)
		}
	case *ast.WithStatement:
		walkExpression(s.Object, emit)
		walkStatement(s.Body, emit)
	case *ast.LabelledStatement:
		walkStatement(s.Statement, emit)
	}
}

// walkForLoopInitializer handles the ForLoopInitializer interface variants.
func walkForLoopInitializer(init ast.ForLoopInitializer, emit func(value, kind string)) {
	if init == nil {
		return
	}
	switch i := init.(type) {
	case *ast.ForLoopInitializerExpression:
		walkExpression(i.Expression, emit)
	case *ast.ForLoopInitializerVarDeclList:
		for _, binding := range i.List {
			walkExpression(binding.Initializer, emit)
		}
	case *ast.ForLoopInitializerLexicalDecl:
		for _, binding := range i.LexicalDeclaration.List {
			walkExpression(binding.Initializer, emit)
		}
	}
}

// walkClass walks a class body for method/field definitions.
func walkClass(cls *ast.ClassLiteral, emit func(value, kind string)) {
	if cls == nil {
		return
	}
	walkExpression(cls.SuperClass, emit)
	for _, elem := range cls.Body {
		switch ce := elem.(type) {
		case *ast.FieldDefinition:
			walkExpression(ce.Key, emit)
			walkExpression(ce.Initializer, emit)
		case *ast.MethodDefinition:
			walkExpression(ce.Key, emit)
			if ce.Body != nil {
				walkStatement(ce.Body.Body, emit)
			}
		case *ast.ClassStaticBlock:
			if ce.Block != nil {
				walkStatement(ce.Block, emit)
			}
		}
	}
}

// walkExpression traverses an expression node, extracting URL-like strings.
func walkExpression(expr ast.Expression, emit func(value, kind string)) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.StringLiteral:
		checkAndEmitURL(e.Value.String(), "string", emit)
	case *ast.TemplateLiteral:
		for _, te := range e.Elements {
			if te.Parsed.String() != "" {
				checkAndEmitURL(te.Parsed.String(), "template", emit)
			}
		}
		for _, texpr := range e.Expressions {
			walkExpression(texpr, emit)
		}
	case *ast.CallExpression:
		walkCallExpression(e, emit)
	case *ast.AssignExpression:
		walkAssignExpression(e, emit)
	case *ast.ObjectLiteral:
		for _, prop := range e.Value {
			walkProperty(prop, emit)
		}
	case *ast.ArrayLiteral:
		for _, item := range e.Value {
			walkExpression(item, emit)
		}
	case *ast.BinaryExpression:
		walkExpression(e.Left, emit)
		walkExpression(e.Right, emit)
	case *ast.UnaryExpression:
		walkExpression(e.Operand, emit)
	case *ast.ConditionalExpression:
		walkExpression(e.Test, emit)
		walkExpression(e.Consequent, emit)
		walkExpression(e.Alternate, emit)
	case *ast.DotExpression:
		walkExpression(e.Left, emit)
	case *ast.BracketExpression:
		walkExpression(e.Left, emit)
		walkExpression(e.Member, emit)
	case *ast.FunctionLiteral:
		walkStatement(e.Body, emit)
		if e.ParameterList != nil {
			for _, p := range e.ParameterList.List {
				walkExpression(p.Initializer, emit)
			}
		}
	case *ast.ArrowFunctionLiteral:
		walkConciseBody(e.Body, emit)
		if e.ParameterList != nil {
			for _, p := range e.ParameterList.List {
				walkExpression(p.Initializer, emit)
			}
		}
	case *ast.NewExpression:
		walkExpression(e.Callee, emit)
		for _, arg := range e.ArgumentList {
			walkExpression(arg, emit)
		}
	case *ast.SequenceExpression:
		for _, item := range e.Sequence {
			walkExpression(item, emit)
		}
	case *ast.ClassLiteral:
		walkClass(e, emit)
	case *ast.YieldExpression:
		walkExpression(e.Argument, emit)
	case *ast.AwaitExpression:
		walkExpression(e.Argument, emit)
	}
}

// walkProperty handles the Property interface variants (PropertyKeyed, PropertyShort, SpreadElement).
func walkProperty(prop ast.Property, emit func(value, kind string)) {
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

// walkConciseBody handles the ConciseBody interface (BlockStatement or ExpressionBody).
func walkConciseBody(body ast.ConciseBody, emit func(value, kind string)) {
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

// walkCallExpression handles function call expressions, detecting common
// URL-passing patterns like fetch(), XMLHttpRequest.open(), window.open(),
// $.ajax(), axios.get(), etc.
func walkCallExpression(call *ast.CallExpression, emit func(value, kind string)) {
	// Walk the callee expression
	walkExpression(call.Callee, emit)

	// Determine the function name for context-aware type labeling
	funcName := getCallName(call)

	// Walk all arguments, emitting URL-like strings with appropriate types
	for i, arg := range call.ArgumentList {
		// For known API functions, try to type the first URL argument specifically
		if i == 0 || (i == 1 && isXHROpen(funcName)) {
			if sl, ok := arg.(*ast.StringLiteral); ok {
				value := sl.Value.String()
				if isURLLike(value) {
					kind := classifyCallType(funcName)
					emit(value, kind)
					continue
				}
			}
		}
		walkExpression(arg, emit)
	}
}

// walkAssignExpression handles assignment expressions, detecting patterns
// like window.location.href = "/path", img.src = "/image.png", etc.
func walkAssignExpression(assign *ast.AssignExpression, emit func(value, kind string)) {
	walkExpression(assign.Left, emit)

	// Check if this is an assignment to a URL-related property
	if dot, ok := assign.Left.(*ast.DotExpression); ok {
		propName := dot.Identifier.Name.String()
		if isURLProperty(propName) {
			if sl, ok := assign.Right.(*ast.StringLiteral); ok {
				value := sl.Value.String()
				if isURLLike(value) {
					emit(value, "assignment")
					return
				}
			}
		}
	}
	walkExpression(assign.Right, emit)
}

// getCallName extracts a human-readable function name from a call expression.
func getCallName(call *ast.CallExpression) string {
	switch callee := call.Callee.(type) {
	case *ast.Identifier:
		return callee.Name.String()
	case *ast.DotExpression:
		objName := ""
		if id, ok := callee.Left.(*ast.Identifier); ok {
			objName = id.Name.String()
		}
		methodName := callee.Identifier.Name.String()
		if objName != "" {
			return objName + "." + methodName
		}
		return methodName
	}
	return ""
}

// classifyCallType returns an endpoint type based on the function being called.
func classifyCallType(funcName string) string {
	lower := strings.ToLower(funcName)
	switch {
	case lower == "fetch":
		return "fetch"
	case strings.Contains(lower, "open"):
		return "xhr"
	case strings.HasPrefix(lower, "$.") || strings.HasPrefix(lower, "jquery"):
		return "jquery"
	case strings.HasPrefix(lower, "axios"):
		return "axios"
	case lower == "require" || lower == "import":
		return "import"
	default:
		return "call"
	}
}

// isXHROpen checks if the function name matches XMLHttpRequest.open or similar.
func isXHROpen(funcName string) bool {
	lower := strings.ToLower(funcName)
	return strings.HasSuffix(lower, ".open") && funcName != "window.open"
}

// isURLProperty checks if a property name typically contains URLs.
func isURLProperty(name string) bool {
	switch strings.ToLower(name) {
	case "href", "src", "action", "url", "uri", "endpoint",
		"location", "pathname", "origin", "baseurl", "apiurl",
		"redirect", "returnurl", "next", "callback", "target":
		return true
	}
	return false
}

// isURLLike checks if a string looks like a URL or URL path.
func isURLLike(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s == "/" || s == "#" || s == "." || s == ".." {
		return false
	}
	// Skip common non-URL strings
	if strings.HasPrefix(s, "data:") || strings.HasPrefix(s, "javascript:") ||
		strings.HasPrefix(s, "mailto:") || strings.HasPrefix(s, "blob:") {
		return false
	}
	return urlLikeStringRegex.MatchString(s)
}

// checkAndEmitURL checks if a string value is URL-like and emits it.
func checkAndEmitURL(value, kind string, emit func(value, kind string)) {
	if isURLLike(value) {
		emit(value, kind)
	}
}

// regexFallbackExtract uses regex to extract endpoints when AST parsing fails.
func regexFallbackExtract(data string) []JSLuiceEndpoint {
	matches := ExtractRelativeEndpoints(data)
	seen := make(map[string]struct{})
	var endpoints []JSLuiceEndpoint

	for _, match := range matches {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		endpoints = append(endpoints, JSLuiceEndpoint{
			Endpoint: match,
			Type:     "regex",
		})
	}
	return endpoints
}
