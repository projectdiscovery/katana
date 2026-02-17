package utils

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

// JSLuiceEndpoint represents an endpoint extracted from JavaScript source code.
type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript source code using
// a pure-Go JavaScript parser (dop251/goja). This replaces the previous
// implementation that used BishopFox/jsluice which depended on
// smacker/go-tree-sitter (a CGo dependency).
//
// The function parses JavaScript into an AST and walks it to find:
//   - String literals that look like URLs/paths
//   - Assignment expressions (e.g. location.href = "/path")
//   - Function calls like fetch(), window.open(), XMLHttpRequest.open()
//   - jQuery-style AJAX calls ($.get, $.post, $.ajax)
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	program, err := parser.ParseFile(nil, "", data, 0)
	if err != nil {
		// If parsing fails, fall back to regex-based extraction
		return extractEndpointsRegex(data)
	}

	extractor := &jsEndpointExtractor{
		source: data,
		seen:   make(map[string]struct{}),
	}
	extractor.walkProgram(program)
	return extractor.endpoints
}

// knownFileExts is a set of known file extensions for URL heuristics.
var knownFileExts = map[string]struct{}{
	"js": {}, "css": {}, "html": {}, "htm": {}, "xhtml": {}, "xlsx": {},
	"xls": {}, "docx": {}, "doc": {}, "pdf": {}, "rss": {}, "xml": {},
	"php": {}, "phtml": {}, "asp": {}, "aspx": {}, "asmx": {}, "ashx": {},
	"cgi": {}, "pl": {}, "rb": {}, "py": {}, "do": {}, "jsp": {},
	"jspa": {}, "json": {}, "jsonp": {}, "txt": {},
}

// jsEndpointExtractor walks a JavaScript AST to extract URL endpoints.
type jsEndpointExtractor struct {
	source    string
	endpoints []JSLuiceEndpoint
	seen      map[string]struct{}
}

// addEndpoint adds an endpoint if it has not been seen before and passes filters.
func (e *jsEndpointExtractor) addEndpoint(rawURL, urlType string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return
	}

	// Filter out data:, tel:, about:, javascript: schemes
	lower := strings.ToLower(rawURL)
	if strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "about:") ||
		strings.HasPrefix(lower, "javascript:") {
		return
	}

	// Skip www.w3.org URLs (too common, not useful)
	if u, err := url.Parse(rawURL); err == nil {
		if u.Hostname() == "www.w3.org" {
			return
		}
	}

	// Deduplicate
	if _, ok := e.seen[rawURL]; ok {
		return
	}
	e.seen[rawURL] = struct{}{}

	e.endpoints = append(e.endpoints, JSLuiceEndpoint{
		Endpoint: rawURL,
		Type:     urlType,
	})
}

// maybeURL checks if a string could be a URL or path.
func maybeURL(s string) bool {
	if !strings.ContainsAny(s, "/?.") {
		return false
	}
	if strings.ContainsAny(s, " ()!<>'\"`{}^$,") {
		return false
	}
	if strings.HasPrefix(s, "/") {
		return true
	}

	u, err := url.Parse(s)
	if err != nil {
		return false
	}

	// Check valid scheme
	if u.Scheme != "" {
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" {
			return false
		}
	}

	// Valid-looking hostname?
	if len(strings.Split(u.Hostname(), ".")) > 1 {
		return true
	}

	// Valid query string with at least one value?
	for _, vv := range u.Query() {
		if len(vv) > 0 && len(vv[0]) > 0 {
			return true
		}
	}

	// Known file extension?
	if !strings.Contains(u.Path, ".") {
		return false
	}
	parts := strings.Split(u.Path, ".")
	ext := parts[len(parts)-1]
	_, ok := knownFileExts[ext]
	return ok
}

// walkProgram walks the top-level AST program node.
func (e *jsEndpointExtractor) walkProgram(program *ast.Program) {
	for _, stmt := range program.Body {
		e.walkStatement(stmt)
	}
	for _, decl := range program.DeclarationList {
		for _, binding := range decl.List {
			e.walkBinding(binding)
		}
	}
}

// walkStatement dispatches to the appropriate handler for each statement type.
func (e *jsEndpointExtractor) walkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		e.walkExpression(s.Expression)
	case *ast.BlockStatement:
		if s != nil {
			for _, item := range s.List {
				e.walkStatement(item)
			}
		}
	case *ast.IfStatement:
		e.walkExpression(s.Test)
		e.walkStatement(s.Consequent)
		e.walkStatement(s.Alternate)
	case *ast.WhileStatement:
		e.walkExpression(s.Test)
		e.walkStatement(s.Body)
	case *ast.DoWhileStatement:
		e.walkExpression(s.Test)
		e.walkStatement(s.Body)
	case *ast.ForStatement:
		e.walkForLoopInitializer(s.Initializer)
		e.walkExpression(s.Test)
		e.walkExpression(s.Update)
		e.walkStatement(s.Body)
	case *ast.ForInStatement:
		e.walkForInto(s.Into)
		e.walkExpression(s.Source)
		e.walkStatement(s.Body)
	case *ast.ForOfStatement:
		e.walkForInto(s.Into)
		e.walkExpression(s.Source)
		e.walkStatement(s.Body)
	case *ast.ReturnStatement:
		e.walkExpression(s.Argument)
	case *ast.SwitchStatement:
		e.walkExpression(s.Discriminant)
		for _, c := range s.Body {
			e.walkExpression(c.Test)
			for _, cs := range c.Consequent {
				e.walkStatement(cs)
			}
		}
	case *ast.TryStatement:
		e.walkStatement(s.Body)
		if s.Catch != nil {
			e.walkStatement(s.Catch.Body)
		}
		e.walkStatement(s.Finally)
	case *ast.ThrowStatement:
		e.walkExpression(s.Argument)
	case *ast.VariableStatement:
		for _, binding := range s.List {
			e.walkBinding(binding)
		}
	case *ast.LabelledStatement:
		e.walkStatement(s.Statement)
	case *ast.WithStatement:
		e.walkExpression(s.Object)
		e.walkStatement(s.Body)
	case *ast.LexicalDeclaration:
		for _, binding := range s.List {
			e.walkBinding(binding)
		}
	case *ast.FunctionDeclaration:
		if s.Function != nil {
			e.walkFunctionLiteral(s.Function)
		}
	case *ast.ClassDeclaration:
		if s.Class != nil {
			e.walkClassLiteral(s.Class)
		}
	}
}

// walkForLoopInitializer walks a for-loop initializer.
func (e *jsEndpointExtractor) walkForLoopInitializer(init ast.ForLoopInitializer) {
	if init == nil {
		return
	}
	switch i := init.(type) {
	case *ast.ForLoopInitializerExpression:
		e.walkExpression(i.Expression)
	case *ast.ForLoopInitializerVarDeclList:
		for _, binding := range i.List {
			e.walkBinding(binding)
		}
	case *ast.ForLoopInitializerLexicalDecl:
		for _, binding := range i.LexicalDeclaration.List {
			e.walkBinding(binding)
		}
	}
}

// walkForInto walks a for-in/for-of target.
func (e *jsEndpointExtractor) walkForInto(into ast.ForInto) {
	if into == nil {
		return
	}
	switch i := into.(type) {
	case *ast.ForIntoExpression:
		e.walkExpression(i.Expression)
	case *ast.ForIntoVar:
		e.walkBinding(i.Binding)
	}
}

// walkBinding walks a variable binding (for var/let/const).
func (e *jsEndpointExtractor) walkBinding(b *ast.Binding) {
	if b == nil {
		return
	}
	e.walkExpression(b.Initializer)
}

// walkFunctionLiteral walks a function literal node.
func (e *jsEndpointExtractor) walkFunctionLiteral(fn *ast.FunctionLiteral) {
	if fn == nil {
		return
	}
	e.walkStatement(fn.Body)
	if fn.ParameterList != nil {
		for _, p := range fn.ParameterList.List {
			e.walkBinding(p)
		}
	}
}

// walkClassLiteral walks a class literal node.
func (e *jsEndpointExtractor) walkClassLiteral(cls *ast.ClassLiteral) {
	if cls == nil {
		return
	}
	for _, elem := range cls.Body {
		switch el := elem.(type) {
		case *ast.MethodDefinition:
			if el.Body != nil {
				e.walkFunctionLiteral(el.Body)
			}
		case *ast.FieldDefinition:
			e.walkExpression(el.Initializer)
		case *ast.ClassStaticBlock:
			e.walkStatement(el.Block)
		}
	}
}

// walkConciseBody walks an arrow function body (either a block or an expression).
func (e *jsEndpointExtractor) walkConciseBody(body ast.ConciseBody) {
	if body == nil {
		return
	}
	switch b := body.(type) {
	case *ast.BlockStatement:
		e.walkStatement(b)
	case *ast.ExpressionBody:
		e.walkExpression(b.Expression)
	}
}

// walkExpression dispatches to the appropriate handler for each expression type.
func (e *jsEndpointExtractor) walkExpression(expr ast.Expression) {
	if expr == nil {
		return
	}
	switch ex := expr.(type) {
	case *ast.StringLiteral:
		val := ex.Value.String()
		if maybeURL(val) {
			e.addEndpoint(val, "stringLiteral")
		}

	case *ast.TemplateLiteral:
		// Walk the expressions embedded in the template literal
		for _, expr := range ex.Expressions {
			e.walkExpression(expr)
		}

	case *ast.AssignExpression:
		e.checkAssignExpression(ex)
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Right)

	case *ast.CallExpression:
		e.checkCallExpression(ex)
		e.walkExpression(ex.Callee)
		for _, arg := range ex.ArgumentList {
			e.walkExpression(arg)
		}

	case *ast.BinaryExpression:
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Right)

	case *ast.UnaryExpression:
		e.walkExpression(ex.Operand)

	case *ast.ConditionalExpression:
		e.walkExpression(ex.Test)
		e.walkExpression(ex.Consequent)
		e.walkExpression(ex.Alternate)

	case *ast.ArrayLiteral:
		for _, val := range ex.Value {
			e.walkExpression(val)
		}

	case *ast.ObjectLiteral:
		for _, prop := range ex.Value {
			e.walkProperty(prop)
		}

	case *ast.FunctionLiteral:
		e.walkFunctionLiteral(ex)

	case *ast.ArrowFunctionLiteral:
		e.walkConciseBody(ex.Body)
		if ex.ParameterList != nil {
			for _, p := range ex.ParameterList.List {
				e.walkBinding(p)
			}
		}

	case *ast.ClassLiteral:
		e.walkClassLiteral(ex)

	case *ast.DotExpression:
		e.walkExpression(ex.Left)

	case *ast.PrivateDotExpression:
		e.walkExpression(ex.Left)

	case *ast.BracketExpression:
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Member)

	case *ast.NewExpression:
		e.walkExpression(ex.Callee)
		for _, arg := range ex.ArgumentList {
			e.walkExpression(arg)
		}

	case *ast.SequenceExpression:
		for _, item := range ex.Sequence {
			e.walkExpression(item)
		}

	case *ast.SpreadElement:
		e.walkExpression(ex.Expression)

	case *ast.YieldExpression:
		e.walkExpression(ex.Argument)

	case *ast.AwaitExpression:
		e.walkExpression(ex.Argument)

	case *ast.OptionalChain:
		e.walkExpression(ex.Expression)

	case *ast.Optional:
		e.walkExpression(ex.Expression)
	}
}

// walkProperty extracts expressions from object property values.
func (e *jsEndpointExtractor) walkProperty(prop ast.Property) {
	if prop == nil {
		return
	}
	switch p := prop.(type) {
	case *ast.PropertyKeyed:
		e.walkExpression(p.Key)
		e.walkExpression(p.Value)
	case *ast.PropertyShort:
		e.walkExpression(p.Initializer)
	case *ast.SpreadElement:
		e.walkExpression(p.Expression)
	}
}

// getExpressionString attempts to extract a string value from an expression,
// collapsing string concatenations.
func (e *jsEndpointExtractor) getExpressionString(expr ast.Expression) (string, bool) {
	if expr == nil {
		return "", false
	}
	switch ex := expr.(type) {
	case *ast.StringLiteral:
		return ex.Value.String(), true
	case *ast.BinaryExpression:
		// Handle string concatenation: "path/" + variable + "/endpoint"
		if ex.Operator.String() == "+" {
			left, leftOk := e.getExpressionString(ex.Left)
			right, rightOk := e.getExpressionString(ex.Right)
			if leftOk && rightOk {
				return left + right, true
			}
			if leftOk {
				return left + "EXPR", true
			}
			if rightOk {
				return "EXPR" + right, true
			}
		}
	case *ast.TemplateLiteral:
		// Try to extract useful parts from template literals
		var sb strings.Builder
		for i, elem := range ex.Elements {
			if elem.Parsed.String() != "" {
				sb.WriteString(elem.Parsed.String())
			}
			if i < len(ex.Expressions) {
				sb.WriteString("EXPR")
			}
		}
		result := sb.String()
		if result != "" {
			return result, true
		}
	}
	return "", false
}

// isStringy checks if an expression starts with or is a string.
func isStringy(expr ast.Expression) bool {
	if expr == nil {
		return false
	}
	switch ex := expr.(type) {
	case *ast.StringLiteral:
		return true
	case *ast.TemplateLiteral:
		return true
	case *ast.BinaryExpression:
		if ex.Operator.String() == "+" {
			return isStringy(ex.Left)
		}
	}
	return false
}

// getExpressionContent returns the source string for an expression.
func (e *jsEndpointExtractor) getExpressionContent(expr ast.Expression) string {
	if expr == nil {
		return ""
	}
	start := int(expr.Idx0()) - 1
	end := int(expr.Idx1()) - 1
	if start < 0 || end < 0 || start >= len(e.source) || end > len(e.source) || start >= end {
		return ""
	}
	return e.source[start:end]
}

// checkAssignExpression checks assignment expressions for URL patterns.
// Matches patterns like: location.href = "/path", this.url = "/api/..."
func (e *jsEndpointExtractor) checkAssignExpression(expr *ast.AssignExpression) {
	leftContent := e.getExpressionContent(expr.Left)

	interestingAssignments := []string{
		"location", "this.url", "this._url", "this.baseUrl",
	}
	interestingSuffixes := []string{".href", ".src", ".location"}

	isInteresting := false
	for _, name := range interestingAssignments {
		if leftContent == name {
			isInteresting = true
			break
		}
	}
	if !isInteresting {
		for _, suffix := range interestingSuffixes {
			if strings.HasSuffix(leftContent, suffix) {
				isInteresting = true
				break
			}
		}
	}

	if !isInteresting {
		return
	}

	if !isStringy(expr.Right) {
		return
	}

	if val, ok := e.getExpressionString(expr.Right); ok && val != "" {
		e.addEndpoint(val, "locationAssignment")
	}
}

// checkCallExpression checks function calls for URL patterns.
// Matches patterns like: fetch("/api/..."), window.open("/path"), $.get("/url"), etc.
func (e *jsEndpointExtractor) checkCallExpression(expr *ast.CallExpression) {
	callName := e.getExpressionContent(expr.Callee)
	if callName == "" {
		return
	}

	if len(expr.ArgumentList) == 0 {
		return
	}

	firstArg := expr.ArgumentList[0]

	switch {
	// fetch(url, [init])
	case callName == "fetch":
		if !isStringy(firstArg) {
			return
		}
		if val, ok := e.getExpressionString(firstArg); ok && val != "" {
			e.addEndpoint(val, "fetch")
		}

	// window.open(url) or open(url)
	case callName == "window.open" || callName == "open":
		if !isStringy(firstArg) {
			return
		}
		if val, ok := e.getExpressionString(firstArg); ok && val != "" {
			e.addEndpoint(val, "window.open")
		}

	// location.replace(url)
	case strings.HasSuffix(callName, "location.replace"):
		if !isStringy(firstArg) {
			return
		}
		if val, ok := e.getExpressionString(firstArg); ok && val != "" {
			e.addEndpoint(val, "locationReplacement")
		}

	// XMLHttpRequest.open(method, url)
	case strings.HasSuffix(callName, ".open"):
		if len(expr.ArgumentList) < 2 {
			return
		}
		secondArg := expr.ArgumentList[1]
		if !isStringy(secondArg) {
			return
		}
		if val, ok := e.getExpressionString(secondArg); ok && val != "" {
			e.addEndpoint(val, "xhr.open")
		}

	// jQuery: $.get, $.post, $.ajax, jQuery.get, jQuery.post, jQuery.ajax
	case callName == "$.get" || callName == "$.post" || callName == "jQuery.get" || callName == "jQuery.post":
		if !isStringy(firstArg) {
			return
		}
		if val, ok := e.getExpressionString(firstArg); ok && val != "" {
			e.addEndpoint(val, callName)
		}

	case callName == "$.ajax" || callName == "jQuery.ajax":
		// $.ajax({url: "/path", ...})
		if objLit, ok := firstArg.(*ast.ObjectLiteral); ok {
			for _, prop := range objLit.Value {
				if keyed, ok := prop.(*ast.PropertyKeyed); ok {
					keyContent := e.getExpressionContent(keyed.Key)
					keyContent = strings.Trim(keyContent, `"'`)
					if keyContent == "url" {
						if val, ok := e.getExpressionString(keyed.Value); ok && val != "" {
							e.addEndpoint(val, callName)
						}
					}
				}
			}
		}

	// Fallback: other function calls with a URL-like first argument
	default:
		if !isStringy(firstArg) {
			return
		}
		if val, ok := e.getExpressionString(firstArg); ok && maybeURL(val) {
			e.addEndpoint(val, callName)
		}
	}
}

// extractEndpointsRegex is a fallback regex-based extractor for when AST parsing fails.
// This handles cases where the JavaScript is malformed or uses syntax the parser
// cannot handle.
func extractEndpointsRegex(data string) []JSLuiceEndpoint {
	var endpoints []JSLuiceEndpoint
	seen := make(map[string]struct{})

	// Match common URL patterns in JavaScript strings
	patterns := []*regexp.Regexp{
		// Absolute URLs
		regexp.MustCompile(`["'\x60](https?://[^\s"'\x60<>{}|\\^]+)["'\x60]`),
		// Root-relative paths
		regexp.MustCompile(`["'\x60](/[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%-]+)["'\x60]`),
		// Relative paths with file extensions
		regexp.MustCompile(`["'\x60](\./[a-zA-Z0-9._~:/?#\[\]@!$&'()*+,;=%-]+)["'\x60]`),
	}

	for _, re := range patterns {
		matches := re.FindAllStringSubmatch(data, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			val := match[1]
			if _, ok := seen[val]; ok {
				continue
			}
			seen[val] = struct{}{}
			endpoints = append(endpoints, JSLuiceEndpoint{
				Endpoint: val,
				Type:     "stringLiteral",
			})
		}
	}
	return endpoints
}
