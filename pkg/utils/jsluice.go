package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/unistring"
)

const (
	maxASTDepth     = 500
	parseTimeout    = 5 * time.Second
)

var (
	// CommonJSLibraryFileRegex is a regex to match common js library files.
	CommonJSLibraryFileRegex = `(?i)(?:amplify|quantserve|slideshow|jquery|modernizr|polyfill|vendor|modules|gtm|underscore?|tween|retina|selectivizr|cufon|angular|swf|sha1|freestyle|bootstrap|d3|backbone|videojs|google[-_]analytics|material|redux|knockout|datepicker|datetimepicker|ember|react|ng|fusion|analytics|libs?|vendors?|node[-_]modules|lodash|moment|chart|highcharts|raphael|prototype|mootools|dojo|ext|yui|web[-_]?components|polymer|vue|svelte|next|nuxt|gatsby|express|koa|hapi|socket[-_.]?io|axios|superagent|request|bluebird|rxjs|ramda|immutable|flux|redux[-_]saga|mobx|relay|apollo|graphql|three|phaser|pixi|babylon|cannon|hammer|howler|gsap|velocity|mo[-_.]?js|popper|shepherd|prism|highlight|markdown[-_]?it|codemirror|ace[-_]?editor|tinymce|ckeditor|quill|simplemde|monaco[-_]?editor|pdf[-_.]?js|jspdf|fabric|paper|konva|p5|processing|matter[-_.]?js|box2d|planck|chart[-_.]?js|plotly|echarts|d3[-_.]?force|sigma|c3|nvd3|amcharts|vis[-_.]?js|dagre[-_.]?d3|cytoscape|leaflet|openlayers|ol3|mapbox|cesium|turf|moment[-_.]?timezone|luxon|dayjs|date[-_.]?fns|date[-_.]?io|flatpickr|pikaday|fullcalendar|draggable|interact|sortable|dragula|dropzone|filepond|uppy|fine[-_.]?uploader|plyr|mediaelement|flowplayer|jwplayer|video[-_.]?js|mediaelement[-_.]?js|dash[-_.]?js|hls[-_.]?js|videojs|wavesurfer|soundmanager|amplitude|pizzicato|tone|adroll|doubleclick|facebook-pixel|ga-audiences|googlesyndication|adsbygoogle|gpt|amazon-adsystem|criteo|taboola|outbrain|bidswitch|bidswitch.net|spotxchange|yahoo|media.net|contextweb|openx|pubmatic|rubiconproject|indexexchange|appnexus|liveintent|triplelift|verizonmedia|synacor|sonobi|yieldmo|gumgum|smartadserver|mopub|pubnative|inmobi|chartboost|tapjoy|admob|unityads|vungle|flurry|matomy|altitude|dataxu|thetradedesk|exponential|zypmedia|quantcast|mediamath|bidswitch|mgid|revcontent|powerlinks|rhythmone|airpush|smaato|adcolony|mopub|leadbolt|mobfox|nativo|revjet|smartyads|avocarrot|epom|imobile|supersonicads|loopme|applovin|pandora|mytarget|bidvertiser|chitika|popads|propellerads|buysellads|adhit|hilltopads|plugrush|popcash|popunder|revenuehits|trafficjunky|trafficfactory|zero-|smartoasis)(?:[-._][\w\d]*)*\.js$`
	commonJSLibraryFileRegexCompiled = regexp.MustCompile(CommonJSLibraryFileRegex)

	// urlLikePattern matches strings that look like URLs or paths
	urlLikePattern = regexp.MustCompile(`^(?:https?://|//|/[a-zA-Z]|\.\.?/|\./|[a-zA-Z][a-zA-Z0-9+.-]*://|/api/|/v\d+/)`)

	// fallbackURLPatterns are compiled once for the regex fallback extractor
	fallbackURLPatterns = []*regexp.Regexp{
		regexp.MustCompile(`['"]((https?://|//)[^'"]+)['"]`),
		regexp.MustCompile(`['"](/[a-zA-Z][^'"]*?)['"]`),
		regexp.MustCompile(`['"](\./[^'"]+)['"]`),
		regexp.MustCompile(`['"](\.\./[^'"]+)['"]`),
	}
)

// IsPathCommonJSLibraryFile checks if a given path is a common js library file.
func IsPathCommonJSLibraryFile(path string) bool {
	return commonJSLibraryFileRegexCompiled.MatchString(path)
}

type JSLuiceEndpoint struct {
	Endpoint string
	Type     string
}

// ExtractJsluiceEndpoints extracts endpoints from JavaScript using a pure Go parser.
//
// This implementation uses goja's parser (pure Go, no CGO) instead of tree-sitter
// to extract URLs and endpoints from JavaScript files.
//
// We apply several optimizations before running the parser:
//   - We skip common js library files.
//   - We handle malformed JavaScript gracefully.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	extractor := newEndpointExtractor()
	extractor.extract(data)
	return extractor.endpoints
}

// endpointExtractor walks the JavaScript AST and extracts URL endpoints
type endpointExtractor struct {
	endpoints []JSLuiceEndpoint
	seen      map[string]bool
	depth     int
	done      chan struct{}
}

func newEndpointExtractor() *endpointExtractor {
	return &endpointExtractor{
		endpoints: make([]JSLuiceEndpoint, 0),
		seen:      make(map[string]bool),
		done:      make(chan struct{}),
	}
}

func (e *endpointExtractor) stopped() bool {
	select {
	case <-e.done:
		return true
	default:
		return false
	}
}

func (e *endpointExtractor) addEndpoint(url, urlType string) {
	url = strings.TrimSpace(url)
	if url == "" || e.seen[url] {
		return
	}
	e.seen[url] = true
	e.endpoints = append(e.endpoints, JSLuiceEndpoint{
		Endpoint: url,
		Type:     urlType,
	})
}

func (e *endpointExtractor) extract(data string) {
	ch := make(chan struct{}, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_ = fmt.Errorf("parser panic: %v", r)
			}
			ch <- struct{}{}
		}()

		program, err := parser.ParseFile(nil, "", data, parser.IgnoreRegExpErrors)
		if err != nil || e.stopped() {
			if !e.stopped() {
				e.extractWithRegex(data)
			}
			return
		}

		for _, stmt := range program.Body {
			if e.stopped() {
				return
			}
			e.walkStatement(stmt)
		}
	}()

	select {
	case <-ch:
		// Goroutine finished normally
	case <-time.After(parseTimeout):
		close(e.done)
		e.extractWithRegex(data)
	}
}

func (e *endpointExtractor) walkStatement(stmt ast.Statement) {
	if stmt == nil || e.stopped() {
		return
	}
	e.depth++
	defer func() { e.depth-- }()
	if e.depth > maxASTDepth {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		e.walkExpression(s.Expression)
	case *ast.VariableStatement:
		for _, binding := range s.List {
			e.walkBinding(binding)
		}
	case *ast.ReturnStatement:
		e.walkExpression(s.Argument)
	case *ast.IfStatement:
		e.walkExpression(s.Test)
		e.walkStatement(s.Consequent)
		e.walkStatement(s.Alternate)
	case *ast.BlockStatement:
		for _, stmt := range s.List {
			e.walkStatement(stmt)
		}
	case *ast.ForStatement:
		e.walkForLoopInitializer(s.Initializer)
		e.walkExpression(s.Test)
		e.walkExpression(s.Update)
		e.walkStatement(s.Body)
	case *ast.ForInStatement:
		e.walkForIntoVar(s.Into)
		e.walkExpression(s.Source)
		e.walkStatement(s.Body)
	case *ast.ForOfStatement:
		e.walkForIntoVar(s.Into)
		e.walkExpression(s.Source)
		e.walkStatement(s.Body)
	case *ast.WhileStatement:
		e.walkExpression(s.Test)
		e.walkStatement(s.Body)
	case *ast.DoWhileStatement:
		e.walkStatement(s.Body)
		e.walkExpression(s.Test)
	case *ast.SwitchStatement:
		e.walkExpression(s.Discriminant)
		for _, caseStmt := range s.Body {
			e.walkExpression(caseStmt.Test)
			for _, stmt := range caseStmt.Consequent {
				e.walkStatement(stmt)
			}
		}
	case *ast.TryStatement:
		e.walkStatement(s.Body)
		if s.Catch != nil {
			e.walkStatement(s.Catch.Body)
		}
		e.walkStatement(s.Finally)
	case *ast.FunctionDeclaration:
		if s.Function != nil {
			e.walkFunctionLiteral(s.Function)
		}
	case *ast.ClassDeclaration:
		if s.Class != nil {
			e.walkClassLiteral(s.Class)
		}
	case *ast.LexicalDeclaration:
		for _, binding := range s.List {
			e.walkBinding(binding)
		}
	case *ast.ThrowStatement:
		e.walkExpression(s.Argument)
	case *ast.WithStatement:
		e.walkExpression(s.Object)
		e.walkStatement(s.Body)
	case *ast.LabelledStatement:
		e.walkStatement(s.Statement)
	}
}

func (e *endpointExtractor) walkBinding(binding *ast.Binding) {
	if binding == nil {
		return
	}
	e.walkExpression(binding.Initializer)
}

func (e *endpointExtractor) walkForLoopInitializer(init ast.ForLoopInitializer) {
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

func (e *endpointExtractor) walkForIntoVar(into ast.ForInto) {
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

func (e *endpointExtractor) walkExpression(expr ast.Expression) {
	if expr == nil || e.stopped() {
		return
	}
	e.depth++
	defer func() { e.depth-- }()
	if e.depth > maxASTDepth {
		return
	}

	switch ex := expr.(type) {
	case *ast.StringLiteral:
		e.checkStringLiteral(uniStringToString(ex.Value), "stringLiteral")

	case *ast.TemplateLiteral:
		// Handle template literals like `${baseUrl}/api/users`
		for _, elem := range ex.Elements {
			e.checkStringLiteral(uniStringToString(elem.Parsed), "templateLiteral")
		}
		for _, expr := range ex.Expressions {
			e.walkExpression(expr)
		}

	case *ast.CallExpression:
		e.handleCallExpression(ex)
		e.walkExpression(ex.Callee)
		for _, arg := range ex.ArgumentList {
			e.walkExpression(arg)
		}

	case *ast.AssignExpression:
		e.handleAssignExpression(ex)
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Right)

	case *ast.DotExpression:
		e.walkExpression(ex.Left)

	case *ast.BracketExpression:
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Member)

	case *ast.ObjectLiteral:
		for _, prop := range ex.Value {
			e.walkProperty(prop)
		}

	case *ast.ArrayLiteral:
		for _, elem := range ex.Value {
			e.walkExpression(elem)
		}

	case *ast.FunctionLiteral:
		e.walkFunctionLiteral(ex)

	case *ast.ArrowFunctionLiteral:
		e.walkConciseBody(ex.Body)

	case *ast.BinaryExpression:
		// Handle string concatenation
		if result := e.tryExtractConcatenatedString(ex); result != "" {
			e.checkStringLiteral(result, "concatenatedString")
		}
		e.walkExpression(ex.Left)
		e.walkExpression(ex.Right)

	case *ast.UnaryExpression:
		e.walkExpression(ex.Operand)

	case *ast.ConditionalExpression:
		e.walkExpression(ex.Test)
		e.walkExpression(ex.Consequent)
		e.walkExpression(ex.Alternate)

	case *ast.NewExpression:
		e.handleNewExpression(ex)
		e.walkExpression(ex.Callee)
		for _, arg := range ex.ArgumentList {
			e.walkExpression(arg)
		}

	case *ast.SequenceExpression:
		for _, seqExpr := range ex.Sequence {
			e.walkExpression(seqExpr)
		}

	case *ast.ClassLiteral:
		e.walkClassLiteral(ex)

	case *ast.AwaitExpression:
		e.walkExpression(ex.Argument)

	case *ast.YieldExpression:
		e.walkExpression(ex.Argument)

	case *ast.SpreadElement:
		e.walkExpression(ex.Expression)

	case *ast.MetaProperty:
		// No expressions to walk

	case *ast.Identifier, *ast.NumberLiteral, *ast.BooleanLiteral,
		*ast.NullLiteral, *ast.RegExpLiteral, *ast.ThisExpression,
		*ast.SuperExpression, *ast.PrivateIdentifier, *ast.PrivateDotExpression:
		// No nested expressions to walk
	}
}

// walkConciseBody handles arrow function bodies which can be either
// a block statement or an expression
func (e *endpointExtractor) walkConciseBody(body ast.ConciseBody) {
	if body == nil {
		return
	}
	switch b := body.(type) {
	case *ast.BlockStatement:
		for _, stmt := range b.List {
			e.walkStatement(stmt)
		}
	case *ast.ExpressionBody:
		e.walkExpression(b.Expression)
	}
}

func (e *endpointExtractor) walkFunctionLiteral(fn *ast.FunctionLiteral) {
	if fn == nil || fn.Body == nil {
		return
	}
	for _, stmt := range fn.Body.List {
		e.walkStatement(stmt)
	}
}

func (e *endpointExtractor) walkClassLiteral(class *ast.ClassLiteral) {
	if class == nil {
		return
	}
	for _, elem := range class.Body {
		switch el := elem.(type) {
		case *ast.FieldDefinition:
			e.walkExpression(el.Initializer)
		case *ast.MethodDefinition:
			if el.Body != nil {
				e.walkFunctionLiteral(el.Body)
			}
		case *ast.ClassStaticBlock:
			if el.Block != nil {
				for _, stmt := range el.Block.List {
					e.walkStatement(stmt)
				}
			}
		}
	}
}

func (e *endpointExtractor) walkProperty(prop ast.Property) {
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

func (e *endpointExtractor) handleCallExpression(call *ast.CallExpression) {
	calleeName := e.getCalleeName(call.Callee)

	// Check for known URL-related function calls
	switch calleeName {
	case "fetch":
		// fetch(url) or fetch(url, options)
		if len(call.ArgumentList) > 0 {
			if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "fetch")
			}
		}
	case "open":
		// window.open(url) - but NOT xhr.open(method, url) which is handled separately
		// Only extract if the first argument looks like a URL (not an HTTP method)
		if len(call.ArgumentList) > 0 {
			if url := e.extractStringValue(call.ArgumentList[0]); url != "" && e.looksLikeURL(url) {
				e.addEndpoint(url, "windowOpen")
			}
		}
	case "replace":
		// location.replace(url)
		if e.isDotExpressionOn(call.Callee, "location", "replace") {
			if len(call.ArgumentList) > 0 {
				if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
					e.addEndpoint(url, "locationReplace")
				}
			}
		}
	case "assign":
		// location.assign(url)
		if e.isDotExpressionOn(call.Callee, "location", "assign") {
			if len(call.ArgumentList) > 0 {
				if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
					e.addEndpoint(url, "locationAssign")
				}
			}
		}
	case "navigate":
		// router.navigate(url) or similar
		if len(call.ArgumentList) > 0 {
			if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "navigate")
			}
		}
	case "redirect":
		if len(call.ArgumentList) > 0 {
			if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "redirect")
			}
		}
	case "get", "post", "put", "delete", "patch":
		// HTTP method calls like axios.get(url), http.post(url)
		if len(call.ArgumentList) > 0 {
			if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "httpMethod")
			}
		}
	case "ajax":
		// $.ajax({url: ...}) or jQuery ajax
		e.extractFromObjectArg(call.ArgumentList, "ajax")
	case "request":
		if len(call.ArgumentList) > 0 {
			if url := e.extractStringValue(call.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "request")
			} else {
				e.extractFromObjectArg(call.ArgumentList, "request")
			}
		}
	case "send":
		// XMLHttpRequest.send() - URL was set in open()
		// WebSocket send - not a URL
	}

	// Check for XMLHttpRequest.open(method, url)
	if e.isDotExpressionMatching(call.Callee, "open") {
		if len(call.ArgumentList) >= 2 {
			if url := e.extractStringValue(call.ArgumentList[1]); url != "" {
				e.addEndpoint(url, "xmlHttpRequest")
			}
		}
	}

	// Check for postMessage
	if calleeName == "postMessage" && len(call.ArgumentList) >= 2 {
		if url := e.extractStringValue(call.ArgumentList[1]); url != "" {
			e.addEndpoint(url, "postMessage")
		}
	}
}

func (e *endpointExtractor) handleNewExpression(newExpr *ast.NewExpression) {
	calleeName := e.getCalleeName(newExpr.Callee)

	switch calleeName {
	case "WebSocket", "EventSource":
		if len(newExpr.ArgumentList) > 0 {
			if url := e.extractStringValue(newExpr.ArgumentList[0]); url != "" {
				e.addEndpoint(url, strings.ToLower(calleeName))
			}
		}
	case "URL":
		if len(newExpr.ArgumentList) > 0 {
			if url := e.extractStringValue(newExpr.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "urlConstructor")
			}
		}
	case "Request":
		if len(newExpr.ArgumentList) > 0 {
			if url := e.extractStringValue(newExpr.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "requestConstructor")
			}
		}
	case "Worker", "SharedWorker":
		if len(newExpr.ArgumentList) > 0 {
			if url := e.extractStringValue(newExpr.ArgumentList[0]); url != "" {
				e.addEndpoint(url, "worker")
			}
		}
	case "Image":
		// Image objects can have src set later, not in constructor
	}
}

func (e *endpointExtractor) handleAssignExpression(assign *ast.AssignExpression) {
	// Check for assignments to URL-related properties
	propName := e.getPropertyName(assign.Left)

	switch propName {
	case "href", "src", "action", "url", "baseUrl", "apiUrl", "endpoint":
		if url := e.extractStringValue(assign.Right); url != "" {
			e.addEndpoint(url, "assignment")
		}
	case "location":
		if url := e.extractStringValue(assign.Right); url != "" {
			e.addEndpoint(url, "locationAssignment")
		}
	}

	// Check if assigning to document.location or window.location
	if e.isLocationAssignment(assign.Left) {
		if url := e.extractStringValue(assign.Right); url != "" {
			e.addEndpoint(url, "locationAssignment")
		}
	}
}

func (e *endpointExtractor) extractFromObjectArg(args []ast.Expression, urlType string) {
	if len(args) == 0 {
		return
	}

	obj, ok := args[0].(*ast.ObjectLiteral)
	if !ok {
		return
	}

	for _, prop := range obj.Value {
		if keyed, ok := prop.(*ast.PropertyKeyed); ok {
			keyName := e.getKeyName(keyed.Key)
			if keyName == "url" || keyName == "uri" {
				if url := e.extractStringValue(keyed.Value); url != "" {
					e.addEndpoint(url, urlType)
				}
			}
		}
	}
}

func (e *endpointExtractor) getCalleeName(expr ast.Expression) string {
	switch ex := expr.(type) {
	case *ast.Identifier:
		return uniStringToString(ex.Name)
	case *ast.DotExpression:
		return uniStringToString(ex.Identifier.Name)
	}
	return ""
}

func (e *endpointExtractor) getPropertyName(expr ast.Expression) string {
	switch ex := expr.(type) {
	case *ast.DotExpression:
		return uniStringToString(ex.Identifier.Name)
	case *ast.Identifier:
		return uniStringToString(ex.Name)
	}
	return ""
}

func (e *endpointExtractor) getKeyName(expr ast.Expression) string {
	switch ex := expr.(type) {
	case *ast.StringLiteral:
		return uniStringToString(ex.Value)
	case *ast.Identifier:
		return uniStringToString(ex.Name)
	}
	return ""
}

func (e *endpointExtractor) isDotExpressionOn(expr ast.Expression, object, method string) bool {
	dot, ok := expr.(*ast.DotExpression)
	if !ok {
		return false
	}
	if uniStringToString(dot.Identifier.Name) != method {
		return false
	}
	if ident, ok := dot.Left.(*ast.Identifier); ok {
		return uniStringToString(ident.Name) == object
	}
	return false
}

func (e *endpointExtractor) isDotExpressionMatching(expr ast.Expression, method string) bool {
	dot, ok := expr.(*ast.DotExpression)
	if !ok {
		return false
	}
	return uniStringToString(dot.Identifier.Name) == method
}

func (e *endpointExtractor) isLocationAssignment(expr ast.Expression) bool {
	switch ex := expr.(type) {
	case *ast.DotExpression:
		propName := uniStringToString(ex.Identifier.Name)
		if propName == "location" || propName == "href" {
			if ident, ok := ex.Left.(*ast.Identifier); ok {
				name := uniStringToString(ident.Name)
				return name == "document" || name == "window" || name == "location"
			}
			// Check for nested like document.location.href
			if inner, ok := ex.Left.(*ast.DotExpression); ok {
				return uniStringToString(inner.Identifier.Name) == "location"
			}
		}
	}
	return false
}

func (e *endpointExtractor) extractStringValue(expr ast.Expression) string {
	switch ex := expr.(type) {
	case *ast.StringLiteral:
		return uniStringToString(ex.Value)
	case *ast.TemplateLiteral:
		if len(ex.Elements) > 0 {
			return uniStringToString(ex.Elements[0].Parsed)
		}
	case *ast.BinaryExpression:
		return e.tryExtractConcatenatedString(ex)
	}
	return ""
}

func (e *endpointExtractor) tryExtractConcatenatedString(expr *ast.BinaryExpression) string {
	// Only handle + operator for string concatenation
	if expr.Operator.String() != "+" {
		return ""
	}

	left := e.extractStringValue(expr.Left)
	right := e.extractStringValue(expr.Right)

	// If we have at least one string part, construct the result
	if left != "" || right != "" {
		// Use "EXPR" as placeholder for unknown parts (like jsluice does)
		if left == "" {
			left = "EXPR"
		}
		if right == "" {
			right = "EXPR"
		}
		return left + right
	}

	return ""
}

func (e *endpointExtractor) checkStringLiteral(value, urlType string) {
	if value == "" {
		return
	}

	// Check if it looks like a URL or path
	if e.looksLikeURL(value) {
		e.addEndpoint(value, urlType)
	}
}

func (e *endpointExtractor) looksLikeURL(s string) bool {
	// Skip very short strings
	if len(s) < 2 {
		return false
	}

	// Skip strings that are just whitespace or special characters
	trimmed := strings.TrimSpace(s)
	if len(trimmed) < 2 {
		return false
	}

	// Check for URL-like patterns
	if urlLikePattern.MatchString(s) {
		return true
	}

	// Check for paths that start with /
	if strings.HasPrefix(s, "/") && len(s) > 1 {
		// Avoid matching things like "/", "//", or paths with only special chars
		rest := strings.TrimPrefix(s, "/")
		if len(rest) > 0 && (rest[0] >= 'a' && rest[0] <= 'z' ||
			rest[0] >= 'A' && rest[0] <= 'Z' ||
			rest[0] >= '0' && rest[0] <= '9') {
			return true
		}
	}

	return false
}

// extractWithRegex is a fallback when AST parsing fails
func (e *endpointExtractor) extractWithRegex(data string) {
	for _, pattern := range fallbackURLPatterns {
		matches := pattern.FindAllStringSubmatch(data, -1)
		for _, match := range matches {
			if len(match) > 1 {
				e.addEndpoint(match[1], "regexFallback")
			}
		}
	}
}

// uniStringToString converts goja's unistring.String to a regular Go string
func uniStringToString(s unistring.String) string {
	return string(s)
}
