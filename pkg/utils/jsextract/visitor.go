package jsextract

import (
	"strings"

	"github.com/dop251/goja/ast"
)

// urlVisitor walks the JavaScript AST and collects URL endpoints.
type urlVisitor struct {
	endpoints []Endpoint
	seen      map[string]bool
}

// addEndpoint adds an endpoint if not already seen and valid.
func (v *urlVisitor) addEndpoint(url, typ string) {
	url = strings.TrimSpace(url)
	if url == "" || v.seen[url] {
		return
	}
	if !isValidURL(url) {
		return
	}
	v.seen[url] = true
	v.endpoints = append(v.endpoints, Endpoint{URL: url, Type: typ})
}

// walkProgram walks the entire program AST.
func (v *urlVisitor) walkProgram(program *ast.Program) {
	for _, stmt := range program.Body {
		v.walkStatement(stmt)
	}
}

// walkStatement handles different statement types.
func (v *urlVisitor) walkStatement(stmt ast.Statement) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExpressionStatement:
		v.walkExpression(s.Expression)
	case *ast.VariableStatement:
		for _, binding := range s.List {
			if binding.Initializer != nil {
				v.walkExpression(binding.Initializer)
			}
		}
	case *ast.LexicalDeclaration:
		for _, binding := range s.List {
			if binding.Initializer != nil {
				v.walkExpression(binding.Initializer)
			}
		}
	case *ast.BlockStatement:
		for _, stmt := range s.List {
			v.walkStatement(stmt)
		}
	case *ast.IfStatement:
		v.walkExpression(s.Test)
		v.walkStatement(s.Consequent)
		if s.Alternate != nil {
			v.walkStatement(s.Alternate)
		}
	case *ast.ForStatement:
		v.walkForLoopInitializer(s.Initializer)
		if s.Test != nil {
			v.walkExpression(s.Test)
		}
		if s.Update != nil {
			v.walkExpression(s.Update)
		}
		v.walkStatement(s.Body)
	case *ast.ForInStatement:
		v.walkExpression(s.Source)
		v.walkStatement(s.Body)
	case *ast.ForOfStatement:
		v.walkExpression(s.Source)
		v.walkStatement(s.Body)
	case *ast.WhileStatement:
		v.walkExpression(s.Test)
		v.walkStatement(s.Body)
	case *ast.DoWhileStatement:
		v.walkStatement(s.Body)
		v.walkExpression(s.Test)
	case *ast.FunctionDeclaration:
		v.walkFunctionLiteral(s.Function)
	case *ast.ReturnStatement:
		if s.Argument != nil {
			v.walkExpression(s.Argument)
		}
	case *ast.TryStatement:
		v.walkStatement(s.Body)
		if s.Catch != nil {
			v.walkStatement(s.Catch.Body)
		}
		if s.Finally != nil {
			v.walkStatement(s.Finally)
		}
	case *ast.SwitchStatement:
		v.walkExpression(s.Discriminant)
		for _, caseStmt := range s.Body {
			if caseStmt.Test != nil {
				v.walkExpression(caseStmt.Test)
			}
			for _, stmt := range caseStmt.Consequent {
				v.walkStatement(stmt)
			}
		}
	case *ast.ThrowStatement:
		v.walkExpression(s.Argument)
	case *ast.ClassDeclaration:
		v.walkClassLiteral(s.Class)
	}
}

// walkForLoopInitializer handles for loop initializers.
func (v *urlVisitor) walkForLoopInitializer(init ast.ForLoopInitializer) {
	if init == nil {
		return
	}
	// ForLoopInitializer can be ForDeclaration or Expression
	// We handle it by checking if it's an expression we can walk
	if expr, ok := init.(ast.Expression); ok {
		v.walkExpression(expr)
	}
	// ForDeclaration types have their own handling via statement walking
}

// walkExpression handles different expression types.
func (v *urlVisitor) walkExpression(expr ast.Expression) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.CallExpression:
		v.handleCallExpression(e)
	case *ast.AssignExpression:
		v.handleAssignExpression(e)
	case *ast.StringLiteral:
		v.handleStringLiteral(e)
	case *ast.TemplateLiteral:
		v.handleTemplateLiteral(e)
	case *ast.BinaryExpression:
		// For string concatenation, extract the combined URL first
		if e.Operator.String() == "+" {
			combined := extractStringValue(e)
			if combined != "" && isLikelyURL(combined) {
				v.addEndpoint(combined, "string")
				return // Don't walk children to avoid duplicate extractions
			}
		}
		// For non-concatenation or invalid URLs, walk the parts
		v.walkExpression(e.Left)
		v.walkExpression(e.Right)
	case *ast.ObjectLiteral:
		for _, prop := range e.Value {
			v.walkProperty(prop)
		}
	case *ast.ArrayLiteral:
		for _, elem := range e.Value {
			v.walkExpression(elem)
		}
	case *ast.FunctionLiteral:
		v.walkFunctionLiteral(e)
	case *ast.ArrowFunctionLiteral:
		v.walkArrowFunction(e)
	case *ast.ConditionalExpression:
		v.walkExpression(e.Test)
		v.walkExpression(e.Consequent)
		v.walkExpression(e.Alternate)
	case *ast.DotExpression:
		v.walkExpression(e.Left)
	case *ast.BracketExpression:
		v.walkExpression(e.Left)
		v.walkExpression(e.Member)
	case *ast.SequenceExpression:
		for _, seq := range e.Sequence {
			v.walkExpression(seq)
		}
	case *ast.UnaryExpression:
		v.walkExpression(e.Operand)
	case *ast.NewExpression:
		v.walkExpression(e.Callee)
		for _, arg := range e.ArgumentList {
			v.walkExpression(arg)
		}
	case *ast.AwaitExpression:
		v.walkExpression(e.Argument)
	case *ast.YieldExpression:
		if e.Argument != nil {
			v.walkExpression(e.Argument)
		}
	case *ast.ClassLiteral:
		v.walkClassLiteral(e)
	}
}

// walkProperty handles object properties.
func (v *urlVisitor) walkProperty(prop ast.Property) {
	switch p := prop.(type) {
	case *ast.PropertyKeyed:
		v.walkExpression(p.Key)
		v.walkExpression(p.Value)
	case *ast.PropertyShort:
		if p.Initializer != nil {
			v.walkExpression(p.Initializer)
		}
	case *ast.SpreadElement:
		v.walkExpression(p.Expression)
	}
}

// walkFunctionLiteral walks a function body.
func (v *urlVisitor) walkFunctionLiteral(fn *ast.FunctionLiteral) {
	if fn == nil || fn.Body == nil {
		return
	}
	v.walkStatement(fn.Body)
}

// walkArrowFunction walks an arrow function body.
func (v *urlVisitor) walkArrowFunction(fn *ast.ArrowFunctionLiteral) {
	if fn == nil || fn.Body == nil {
		return
	}
	// ConciseBody can be ExpressionBody (concise) or BlockStatement (block body)
	switch body := fn.Body.(type) {
	case *ast.ExpressionBody:
		// Concise arrow function: () => expression
		v.walkExpression(body.Expression)
	case *ast.BlockStatement:
		// Block arrow function: () => { statements }
		for _, stmt := range body.List {
			v.walkStatement(stmt)
		}
	case ast.Expression:
		v.walkExpression(body)
	case ast.Statement:
		v.walkStatement(body)
	}
}

// walkClassLiteral walks class body.
func (v *urlVisitor) walkClassLiteral(class *ast.ClassLiteral) {
	if class == nil {
		return
	}
	for _, elem := range class.Body {
		switch e := elem.(type) {
		case *ast.MethodDefinition:
			if e.Body != nil {
				v.walkFunctionLiteral(e.Body)
			}
		case *ast.FieldDefinition:
			if e.Initializer != nil {
				v.walkExpression(e.Initializer)
			}
		case *ast.ClassStaticBlock:
			v.walkStatement(e.Block)
		}
	}
}

// handleCallExpression checks for URL-related function calls.
func (v *urlVisitor) handleCallExpression(call *ast.CallExpression) {
	// First, handle specific call types to add URLs with correct type
	// This must happen before walking arguments so that strings are marked as seen
	// with the correct type (e.g., "fetch" instead of "string")
	callee := call.Callee
	switch c := callee.(type) {
	case *ast.Identifier:
		v.handleIdentifierCall(c.Name.String(), call)
	case *ast.DotExpression:
		v.handleMemberCallExpression(c, call)
	case *ast.BracketExpression:
		v.handleBracketCallExpression(c, call)
	}

	// Then walk arguments to extract any nested URLs
	// URLs already added with specific types will be skipped (already seen)
	for _, arg := range call.ArgumentList {
		v.walkExpression(arg)
	}
}

// handleIdentifierCall handles direct function calls like fetch(), open(), etc.
func (v *urlVisitor) handleIdentifierCall(name string, call *ast.CallExpression) {
	switch name {
	case "fetch":
		v.extractFromArgs(call.ArgumentList, "fetch")
	case "open":
		v.extractFromArgs(call.ArgumentList, "window.open")
	case "require":
		v.extractFromArgs(call.ArgumentList, "require")
	case "importScripts":
		v.extractFromArgs(call.ArgumentList, "importScripts")
	}
}

// handleMemberCallExpression handles method calls like xhr.open(), window.open(), etc.
func (v *urlVisitor) handleMemberCallExpression(member *ast.DotExpression, call *ast.CallExpression) {
	propName := member.Identifier.Name.String()

	switch propName {
	case "open":
		// XMLHttpRequest.open(method, url) or window.open(url)
		objName := getExpressionName(member.Left)
		if objName == "window" && len(call.ArgumentList) >= 1 {
			v.extractFromArgs(call.ArgumentList, "window.open")
		} else if len(call.ArgumentList) >= 2 {
			// xhr.open(method, url) - URL is second argument
			if str := extractStringValue(call.ArgumentList[1]); str != "" {
				v.addEndpoint(str, "xhr")
			}
		}
	case "send":
		if len(call.ArgumentList) >= 1 {
			v.extractFromArgs(call.ArgumentList, "send")
		}
	case "assign", "replace":
		// location.assign(url), location.replace(url)
		v.extractFromArgs(call.ArgumentList, "location")
	case "setAttribute":
		// element.setAttribute("href", url)
		if len(call.ArgumentList) >= 2 {
			if attr := extractStringValue(call.ArgumentList[0]); isURLAttribute(attr) {
				if url := extractStringValue(call.ArgumentList[1]); url != "" {
					v.addEndpoint(url, "setAttribute")
				}
			}
		}
	case "ajax", "get", "post", "put", "delete", "patch":
		// jQuery/axios style: $.ajax({url: "..."}) or $.get("/url")
		v.extractFromArgs(call.ArgumentList, propName)
	case "fetch":
		v.extractFromArgs(call.ArgumentList, "fetch")
	case "navigate", "navigateTo", "redirect", "redirectTo", "push", "pushState":
		v.extractFromArgs(call.ArgumentList, "navigate")
	case "load", "getScript", "getJSON":
		v.extractFromArgs(call.ArgumentList, propName)
	case "src":
		// img.src("/image.png") style
		v.extractFromArgs(call.ArgumentList, "src")
	}
}

// handleBracketCallExpression handles calls like obj["method"]().
func (v *urlVisitor) handleBracketCallExpression(bracket *ast.BracketExpression, call *ast.CallExpression) {
	if propName := extractStringValue(bracket.Member); propName != "" {
		switch propName {
		case "open", "fetch", "ajax", "get", "post":
			v.extractFromArgs(call.ArgumentList, propName)
		}
	}
}

// handleAssignExpression checks for location and URL attribute assignments.
func (v *urlVisitor) handleAssignExpression(assign *ast.AssignExpression) {
	// First, handle specific assignment types to add URLs with correct type
	// This must happen before walking the right side so that strings are marked as seen
	// with the correct type (e.g., "location" instead of "string")
	switch left := assign.Left.(type) {
	case *ast.DotExpression:
		propName := left.Identifier.Name.String()
		objName := getExpressionName(left.Left)

		// Check for location assignments: window.location = "url", location.href = "url"
		if isLocationObject(objName) || propName == "location" {
			if propName == "href" || propName == "pathname" || propName == "location" || propName == "search" {
				if url := extractStringValue(assign.Right); url != "" {
					v.addEndpoint(url, "location")
				}
			}
		}

		// Check for URL attribute assignments: element.src = "url", element.href = "url"
		if isURLAttribute(propName) {
			if url := extractStringValue(assign.Right); url != "" {
				v.addEndpoint(url, "assignment")
			}
		}

	case *ast.BracketExpression:
		// obj["href"] = "url" style
		if propName := extractStringValue(left.Member); isURLAttribute(propName) {
			if url := extractStringValue(assign.Right); url != "" {
				v.addEndpoint(url, "assignment")
			}
		}
	}

	// Then walk the right side to extract any nested URLs
	// URLs already added with specific types will be skipped (already seen)
	v.walkExpression(assign.Right)
}

// handleStringLiteral extracts URLs from string literals.
func (v *urlVisitor) handleStringLiteral(str *ast.StringLiteral) {
	v.extractURLFromString(str.Value.String(), "string")
}

// handleTemplateLiteral extracts URLs from template literals.
func (v *urlVisitor) handleTemplateLiteral(tmpl *ast.TemplateLiteral) {
	if len(tmpl.Expressions) == 0 {
		// Simple template without expressions
		fullValue := buildTemplateString(tmpl, false)
		v.extractURLFromString(fullValue, "string")
	} else {
		// Template with expressions - build with EXPR placeholders
		fullValue := buildTemplateString(tmpl, true)
		if isLikelyURL(fullValue) {
			v.addEndpoint(fullValue, "template")
		}
	}

	// Walk expressions in template
	for _, expr := range tmpl.Expressions {
		v.walkExpression(expr)
	}
}

// extractFromArgs extracts URL from function arguments.
func (v *urlVisitor) extractFromArgs(args []ast.Expression, typ string) {
	if len(args) == 0 {
		return
	}

	// First argument is typically the URL
	if url := extractStringValue(args[0]); url != "" {
		v.addEndpoint(url, typ)
		return
	}

	// Check object literal for url property (jQuery/axios style)
	if obj, ok := args[0].(*ast.ObjectLiteral); ok {
		for _, prop := range obj.Value {
			if keyed, ok := prop.(*ast.PropertyKeyed); ok {
				keyName := getPropertyKeyName(keyed.Key)
				if keyName == "url" || keyName == "href" || keyName == "src" || keyName == "path" {
					if url := extractStringValue(keyed.Value); url != "" {
						v.addEndpoint(url, typ)
					}
				}
			}
		}
	}
}

// extractURLFromString checks if a string looks like a URL/path and adds it.
func (v *urlVisitor) extractURLFromString(s, typ string) {
	s = strings.TrimSpace(s)
	if isLikelyURL(s) {
		v.addEndpoint(s, typ)
	}
}
