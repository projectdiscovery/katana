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

	// fileExtensions used by maybeURL to determine if a path might be a URL
	urlFileExtensions = map[string]bool{
		"js": true, "css": true, "html": true, "htm": true, "xhtml": true, "xlsx": true,
		"xls": true, "docx": true, "doc": true, "pdf": true, "rss": true, "xml": true,
		"php": true, "phtml": true, "asp": true, "aspx": true, "asmx": true, "ashx": true,
		"cgi": true, "pl": true, "rb": true, "py": true, "do": true, "jsp": true,
		"jspa": true, "json": true, "jsonp": true, "txt": true,
	}

	// location-related assignment targets
	locationAssignmentNames = map[string]bool{
		"location":     true,
		"this.url":     true,
		"this._url":    true,
		"this.baseUrl": true,
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

// maybeURL checks if a string could potentially be a URL or path.
// Ported from BishopFox/jsluice's MaybeURL logic.
func maybeURL(in string) bool {
	if !strings.ContainsAny(in, "/?.") {
		return false
	}
	if strings.ContainsAny(in, " ()!<>'\"`{}^$,") {
		return false
	}
	if strings.HasPrefix(in, "/") {
		return true
	}

	u, err := url.Parse(in)
	if err != nil {
		return false
	}

	if u.Scheme != "" {
		s := strings.ToLower(u.Scheme)
		if s != "http" && s != "https" {
			return false
		}
	}

	if len(strings.Split(u.Hostname(), ".")) > 1 {
		return true
	}

	for _, vv := range u.Query() {
		if len(vv) > 0 && len(vv[0]) > 0 {
			return true
		}
	}

	if !strings.ContainsAny(u.Path, ".") {
		return false
	}

	parts := strings.Split(u.Path, ".")
	ext := strings.ToLower(parts[len(parts)-1])
	return urlFileExtensions[ext]
}

// exprToString extracts a string value from an AST expression.
// Returns the string and true if the expression is a string literal or
// a template literal with only static parts.
func exprToString(expr ast.Expression) (string, bool) {
	switch e := expr.(type) {
	case *ast.StringLiteral:
		return e.Value.String(), true
	case *ast.TemplateLiteral:
		if len(e.Elements) > 0 {
			var sb strings.Builder
			for _, elem := range e.Elements {
				sb.WriteString(elem.Parsed.String())
			}
			result := sb.String()
			if result != "" {
				return result, true
			}
		}
	}
	return "", false
}

// exprName returns a dotted name for an expression (e.g., "window.open", "location.href").
func exprName(expr ast.Expression) string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return e.Name.String()
	case *ast.DotExpression:
		left := exprName(e.Left)
		if left != "" {
			return left + "." + e.Identifier.Name.String()
		}
		return e.Identifier.Name.String()
	}
	return ""
}

// isLocationAssignment checks if the left side of an assignment is a location-related target.
func isLocationAssignment(name string) bool {
	if locationAssignmentNames[name] {
		return true
	}
	if strings.HasSuffix(name, ".href") || strings.HasSuffix(name, ".src") || strings.HasSuffix(name, ".location") {
		return true
	}
	return false
}

// objectLiteralURL extracts the "url" property string from an object literal expression.
func objectLiteralURL(expr ast.Expression) string {
	obj, ok := expr.(*ast.ObjectLiteral)
	if !ok {
		return ""
	}
	for _, prop := range obj.Value {
		pk, ok := prop.(*ast.PropertyKeyed)
		if !ok {
			continue
		}
		keyName := ""
		switch k := pk.Key.(type) {
		case *ast.StringLiteral:
			keyName = k.Value.String()
		case *ast.Identifier:
			keyName = k.Name.String()
		}
		if keyName == "url" {
			if s, ok := exprToString(pk.Value); ok {
				return s
			}
		}
	}
	return ""
}

// ExtractJsluiceEndpoints extracts URL endpoints from JavaScript source code
// using a pure-Go JavaScript parser (dop251/goja).
//
// This replaces the previous BishopFox/jsluice-based implementation that required
// CGO via go-tree-sitter, enabling cross-platform builds without CGO.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	program, err := parser.ParseFile(nil, "", data, 0)
	if err != nil {
		// If parsing fails, fall back to regex-based extraction
		return extractURLsByRegex(data)
	}

	seen := make(map[string]bool)
	var endpoints []JSLuiceEndpoint

	addEndpoint := func(ep, typ string) {
		if ep == "" || seen[ep] {
			return
		}
		// Filter out unwanted schemes
		lower := strings.ToLower(ep)
		if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "tel:") ||
			strings.HasPrefix(lower, "about:") || strings.HasPrefix(lower, "javascript:") {
			return
		}
		if u, err := url.Parse(ep); err == nil && u.Hostname() == "www.w3.org" {
			return
		}
		seen[ep] = true
		endpoints = append(endpoints, JSLuiceEndpoint{Endpoint: ep, Type: typ})
	}

	walkExpression(program, func(expr ast.Expression) {
		switch e := expr.(type) {
		case *ast.StringLiteral:
			s := e.Value.String()
			if maybeURL(s) {
				addEndpoint(s, "stringLiteral")
			}

		case *ast.TemplateLiteral:
			if s, ok := exprToString(e); ok && maybeURL(s) {
				addEndpoint(s, "stringLiteral")
			}

		case *ast.AssignExpression:
			leftName := exprName(e.Left)
			if isLocationAssignment(leftName) {
				if s, ok := exprToString(e.Right); ok {
					addEndpoint(s, "locationAssignment")
				}
			}

		case *ast.CallExpression:
			callName := exprName(e.Callee)
			if len(e.ArgumentList) == 0 {
				return
			}

			// Handle call-shape-specific branches first (some don't need a string first arg)
			switch {
			case callName == "$.ajax" || callName == "jQuery.ajax":
				// $.ajax({url: "/path"}) or $.ajax("/path")
				if firstArg, ok := exprToString(e.ArgumentList[0]); ok {
					addEndpoint(firstArg, "$.ajax")
				} else if urlStr := objectLiteralURL(e.ArgumentList[0]); urlStr != "" {
					addEndpoint(urlStr, "$.ajax")
				}
				return
			case (callName == "XMLHttpRequest.open" || strings.HasSuffix(callName, ".open")) && len(e.ArgumentList) >= 2:
				// xhr.open(method, "/url") — URL is the second argument
				if secondArg, ok := exprToString(e.ArgumentList[1]); ok {
					addEndpoint(secondArg, "XMLHttpRequest.open")
				}
				return
			}

			// For remaining patterns, the first argument must be a string
			firstArg, ok := exprToString(e.ArgumentList[0])
			if !ok {
				return
			}

			switch {
			case callName == "fetch":
				addEndpoint(firstArg, "fetch")
			case callName == "window.open" || callName == "open":
				addEndpoint(firstArg, "window.open")
			case strings.HasSuffix(callName, "location.replace"):
				addEndpoint(firstArg, "locationReplacement")
			case callName == "$.get" || callName == "jQuery.get":
				addEndpoint(firstArg, "$.get")
			case callName == "$.post" || callName == "jQuery.post":
				addEndpoint(firstArg, "$.post")
			default:
				if maybeURL(firstArg) {
					addEndpoint(firstArg, callName)
				}
			}
		}
	})

	return endpoints
}

// walkExpression recursively walks the AST and calls fn for each expression node.
func walkExpression(program *ast.Program, fn func(ast.Expression)) {
	for _, stmt := range program.Body {
		walkStatement(stmt, fn)
	}
}

func walkStatement(stmt ast.Statement, fn func(ast.Expression)) {
	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		walkExpr(s.Expression, fn)
	case *ast.ReturnStatement:
		if s.Argument != nil {
			walkExpr(s.Argument, fn)
		}
	case *ast.BlockStatement:
		for _, st := range s.List {
			walkStatement(st, fn)
		}
	case *ast.IfStatement:
		walkExpr(s.Test, fn)
		walkStatement(s.Consequent, fn)
		if s.Alternate != nil {
			walkStatement(s.Alternate, fn)
		}
	case *ast.VariableStatement:
		for _, b := range s.List {
			if b.Initializer != nil {
				walkExpr(b.Initializer, fn)
			}
		}
	case *ast.ForStatement:
		if s.Initializer != nil {
			if init, ok := s.Initializer.(*ast.ForLoopInitializerExpression); ok {
				walkExpr(init.Expression, fn)
			} else if init, ok := s.Initializer.(*ast.ForLoopInitializerVarDeclList); ok {
				for _, b := range init.List {
					if b.Initializer != nil {
						walkExpr(b.Initializer, fn)
					}
				}
			} else if init, ok := s.Initializer.(*ast.ForLoopInitializerLexicalDecl); ok {
				for _, b := range init.LexicalDeclaration.List {
					if b.Initializer != nil {
						walkExpr(b.Initializer, fn)
					}
				}
			}
		}
		if s.Test != nil {
			walkExpr(s.Test, fn)
		}
		if s.Update != nil {
			walkExpr(s.Update, fn)
		}
		walkStatement(s.Body, fn)
	case *ast.ForInStatement:
		walkExpr(s.Source, fn)
		walkStatement(s.Body, fn)
	case *ast.ForOfStatement:
		walkExpr(s.Source, fn)
		walkStatement(s.Body, fn)
	case *ast.SwitchStatement:
		walkExpr(s.Discriminant, fn)
		for _, c := range s.Body {
			if c.Test != nil {
				walkExpr(c.Test, fn)
			}
			for _, st := range c.Consequent {
				walkStatement(st, fn)
			}
		}
	case *ast.TryStatement:
		walkStatement(s.Body, fn)
		if s.Catch != nil {
			walkStatement(s.Catch.Body, fn)
		}
		if s.Finally != nil {
			walkStatement(s.Finally, fn)
		}
	case *ast.ThrowStatement:
		walkExpr(s.Argument, fn)
	case *ast.WhileStatement:
		walkExpr(s.Test, fn)
		walkStatement(s.Body, fn)
	case *ast.DoWhileStatement:
		walkExpr(s.Test, fn)
		walkStatement(s.Body, fn)
	case *ast.WithStatement:
		walkExpr(s.Object, fn)
		walkStatement(s.Body, fn)
	case *ast.LabelledStatement:
		walkStatement(s.Statement, fn)
	case *ast.LexicalDeclaration:
		for _, b := range s.List {
			if b.Initializer != nil {
				walkExpr(b.Initializer, fn)
			}
		}
	case *ast.FunctionDeclaration:
		if s.Function != nil && s.Function.Body != nil {
			walkStatement(s.Function.Body, fn)
		}
	case *ast.ClassDeclaration:
		if s.Class != nil {
			for _, elem := range s.Class.Body {
				switch ce := elem.(type) {
				case *ast.MethodDefinition:
					if ce.Body != nil && ce.Body.Body != nil {
						walkStatement(ce.Body.Body, fn)
					}
				case *ast.FieldDefinition:
					if ce.Initializer != nil {
						walkExpr(ce.Initializer, fn)
					}
				}
			}
		}
	}
}

func walkExpr(expr ast.Expression, fn func(ast.Expression)) {
	if expr == nil {
		return
	}
	fn(expr)

	switch e := expr.(type) {
	case *ast.AssignExpression:
		walkExpr(e.Left, fn)
		walkExpr(e.Right, fn)
	case *ast.BinaryExpression:
		walkExpr(e.Left, fn)
		walkExpr(e.Right, fn)
	case *ast.CallExpression:
		walkExpr(e.Callee, fn)
		for _, arg := range e.ArgumentList {
			walkExpr(arg, fn)
		}
	case *ast.ConditionalExpression:
		walkExpr(e.Test, fn)
		walkExpr(e.Consequent, fn)
		walkExpr(e.Alternate, fn)
	case *ast.DotExpression:
		walkExpr(e.Left, fn)
	case *ast.ArrayLiteral:
		for _, v := range e.Value {
			walkExpr(v, fn)
		}
	case *ast.ObjectLiteral:
		for _, prop := range e.Value {
			if pk, ok := prop.(*ast.PropertyKeyed); ok {
				walkExpr(pk.Key, fn)
				walkExpr(pk.Value, fn)
			}
		}
	case *ast.FunctionLiteral:
		if e.Body != nil {
			walkStatement(e.Body, fn)
		}
	case *ast.ArrowFunctionLiteral:
		switch b := e.Body.(type) {
		case *ast.BlockStatement:
			walkStatement(b, fn)
		case *ast.ExpressionBody:
			walkExpr(b.Expression, fn)
		}
	case *ast.UnaryExpression:
		walkExpr(e.Operand, fn)
	case *ast.NewExpression:
		walkExpr(e.Callee, fn)
		for _, arg := range e.ArgumentList {
			walkExpr(arg, fn)
		}
	case *ast.SequenceExpression:
		for _, s := range e.Sequence {
			walkExpr(s, fn)
		}
	case *ast.BracketExpression:
		walkExpr(e.Left, fn)
		walkExpr(e.Member, fn)
	case *ast.TemplateLiteral:
		for _, ex := range e.Expressions {
			walkExpr(ex, fn)
		}
	case *ast.SpreadElement:
		walkExpr(e.Expression, fn)
	case *ast.YieldExpression:
		if e.Argument != nil {
			walkExpr(e.Argument, fn)
		}
	case *ast.AwaitExpression:
		if e.Argument != nil {
			walkExpr(e.Argument, fn)
		}
	}
}

// extractURLsByRegex is a fallback URL extractor for when AST parsing fails.
// It uses regex to find URL-like strings in JavaScript source.
var urlRegex = regexp.MustCompile(`(?:"|'|` + "`" + `)((https?://[^\s"'` + "`" + `]+)|(/[a-zA-Z0-9_/.-]+(?:\?[^\s"'` + "`" + `]*)?))(?:"|'|` + "`" + `)`)

func extractURLsByRegex(data string) []JSLuiceEndpoint {
	seen := make(map[string]bool)
	var endpoints []JSLuiceEndpoint

	matches := urlRegex.FindAllStringSubmatch(data, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		ep := m[1]
		if ep == "" || seen[ep] {
			continue
		}
		if maybeURL(ep) {
			seen[ep] = true
			endpoints = append(endpoints, JSLuiceEndpoint{Endpoint: ep, Type: "regex"})
		}
	}
	return endpoints
}
