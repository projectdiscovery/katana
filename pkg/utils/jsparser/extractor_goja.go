//go:build !cgo

package jsparser

import (
	"strings"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
)

type gojaExtractor struct{}

// default js parser when CGO_ENABLED=0 i.e. disabled
func New() Extractor {
	return &gojaExtractor{}
}

func (e *gojaExtractor) Name() string {
	return "goja"
}

func (e *gojaExtractor) Extract(data string) []Endpoint {
	program, err := parser.ParseFile(nil, "", data, 0)
	if err != nil {
		return DedupeEndpoints(extractFromSource(data))
	}

	collector := newGojaCollector()
	collector.walkProgram(program)

	endpoints := append([]Endpoint{}, collector.endpoints...)
	endpoints = append(endpoints, extractFromSource(data)...)

	return DedupeEndpoints(endpoints)
}

type gojaCollector struct {
	endpoints []Endpoint
	scope     []map[string][]string
}

func newGojaCollector() *gojaCollector {
	return &gojaCollector{
		scope: []map[string][]string{{}},
	}
}

func (c *gojaCollector) walkProgram(program *ast.Program) {
	if program == nil {
		return
	}

	for _, decl := range program.DeclarationList {
		c.walkVariableDeclaration(decl)
	}
	for _, stmt := range program.Body {
		c.walkStatement(stmt)
	}
}

func (c *gojaCollector) walkStatement(stmt ast.Statement) {
	switch n := stmt.(type) {
	case *ast.BlockStatement:
		c.pushScope()
		defer c.popScope()

		for _, item := range n.List {
			c.walkStatement(item)
		}

	case *ast.ExpressionStatement:
		c.addValues(extractExpressionStrings(n.Expression, c))

	case *ast.VariableStatement:
		for _, binding := range n.List {
			c.walkBinding(binding)
		}

	case *ast.LexicalDeclaration:
		for _, binding := range n.List {
			c.walkBinding(binding)
		}

	case *ast.ReturnStatement:
		c.addValues(extractExpressionStrings(n.Argument, c))

	case *ast.ThrowStatement:
		c.addValues(extractExpressionStrings(n.Argument, c))

	case *ast.IfStatement:
		c.addValues(extractExpressionStrings(n.Test, c))
		c.walkStatement(n.Consequent)
		if n.Alternate != nil {
			c.walkStatement(n.Alternate)
		}

	case *ast.TryStatement:
		c.walkStatement(n.Body)
		if n.Catch != nil {
			c.pushScope()
			if name := bindingTargetName(n.Catch.Parameter); name != "" {
				c.currentScope()[name] = nil
			}
			c.walkStatement(n.Catch.Body)
			c.popScope()
		}
		if n.Finally != nil {
			c.walkStatement(n.Finally)
		}

	case *ast.ForStatement:
		if n.Initializer != nil {
			c.walkForLoopInitializer(n.Initializer)
		}
		if n.Test != nil {
			c.addValues(extractExpressionStrings(n.Test, c))
		}
		if n.Update != nil {
			c.addValues(extractExpressionStrings(n.Update, c))
		}
		c.walkStatement(n.Body)

	case *ast.ForInStatement:
		c.walkForInto(n.Into)
		c.addValues(extractExpressionStrings(n.Source, c))
		c.walkStatement(n.Body)

	case *ast.ForOfStatement:
		c.walkForInto(n.Into)
		c.addValues(extractExpressionStrings(n.Source, c))
		c.walkStatement(n.Body)

	case *ast.SwitchStatement:
		c.addValues(extractExpressionStrings(n.Discriminant, c))
		for _, body := range n.Body {
			for _, stmt := range body.Consequent {
				c.walkStatement(stmt)
			}
		}

	case *ast.FunctionDeclaration:
		if n.Function != nil {
			if n.Function.Name != nil {
				c.currentScope()[n.Function.Name.Name.String()] = nil
			}
			c.walkFunctionLiteral(n.Function)
		}

	case *ast.LabelledStatement:
		c.walkStatement(n.Statement)

	case *ast.WithStatement:
		c.addValues(extractExpressionStrings(n.Object, c))
		c.walkStatement(n.Body)

	case *ast.WhileStatement:
		c.addValues(extractExpressionStrings(n.Test, c))
		c.walkStatement(n.Body)

	case *ast.DoWhileStatement:
		c.walkStatement(n.Body)
		c.addValues(extractExpressionStrings(n.Test, c))
	}
}

func (c *gojaCollector) walkForLoopInitializer(init ast.ForLoopInitializer) {
	switch n := init.(type) {
	case *ast.ForLoopInitializerExpression:
		c.addValues(extractExpressionStrings(n.Expression, c))
	case *ast.ForLoopInitializerVarDeclList:
		for _, binding := range n.List {
			c.walkBinding(binding)
		}
	case *ast.ForLoopInitializerLexicalDecl:
		for _, binding := range n.LexicalDeclaration.List {
			c.walkBinding(binding)
		}
	}
}

func (c *gojaCollector) walkForInto(into ast.ForInto) {
	switch n := into.(type) {
	case *ast.ForIntoVar:
		c.walkBinding(n.Binding)
	case *ast.ForDeclaration:
		if name := bindingTargetName(n.Target); name != "" {
			c.currentScope()[name] = nil
		}
	case *ast.ForIntoExpression:
		c.addValues(extractExpressionStrings(n.Expression, c))
	}
}

func (c *gojaCollector) walkBinding(binding *ast.Binding) {
	if binding == nil {
		return
	}

	name := bindingName(binding)
	values := extractExpressionStrings(binding.Initializer, c)

	if name != "" {
		c.setIdentifier(name, values)
	}

	c.addValues(values)
}

// walk var declaration
func (c *gojaCollector) walkVariableDeclaration(decl *ast.VariableDeclaration) {
	if decl == nil {
		return
	}

	for _, binding := range decl.List {
		c.walkBinding(binding)
	}
}

// fn literals
func (c *gojaCollector) walkFunctionLiteral(fn *ast.FunctionLiteral) {
	if fn == nil || fn.Body == nil {
		return
	}

	c.pushScope()
	defer c.popScope()

	if fn.ParameterList != nil {
		for _, param := range fn.ParameterList.List {
			if param == nil {
				continue
			}
			if name := bindingName(param); name != "" {
				c.currentScope()[name] = nil
			}
		}
	}

	for _, decl := range fn.DeclarationList {
		c.walkVariableDeclaration(decl)
	}

	for _, stmt := range fn.Body.List {
		c.walkStatement(stmt)
	}
}

func (c *gojaCollector) addValues(values []string) {
	for _, value := range values {
		value = NormalizeEndpoint(value)
		if !IsLikelyEndpoint(value) {
			continue
		}
		c.endpoints = append(c.endpoints, Endpoint{
			Endpoint: value,
			Type:     ClassifyEndpoint(value),
		})
	}
}

func (c *gojaCollector) assignExpression(left ast.Expression, values []string) {
	switch n := left.(type) {
	case *ast.Identifier:
		c.setIdentifier(n.Name.String(), values)
	case *ast.DotExpression:
		if name := calleeName(n); name != "" {
			c.setIdentifier(name, values)
		}
	case *ast.BracketExpression:
		if name := calleeName(n.Left); name != "" {
			c.setIdentifier(name, values)
		}
	}
}

func (c *gojaCollector) resolveIdentifier(name string) []string {
	for i := len(c.scope) - 1; i >= 0; i-- {
		if values, ok := c.scope[i][name]; ok {
			return values
		}
	}
	return nil
}

func (c *gojaCollector) setIdentifier(name string, values []string) {
	if name == "" {
		return
	}
	for i := len(c.scope) - 1; i >= 0; i-- {
		if _, ok := c.scope[i][name]; ok {
			c.scope[i][name] = values
			return
		}
	}
	c.currentScope()[name] = values
}

func (c *gojaCollector) pushScope() {
	c.scope = append(c.scope, map[string][]string{})
}

func (c *gojaCollector) popScope() {
	if len(c.scope) > 1 {
		c.scope = c.scope[:len(c.scope)-1]
	}
}

func (c *gojaCollector) currentScope() map[string][]string {
	return c.scope[len(c.scope)-1]
}

func extractExpressionStrings(expr ast.Expression, collector *gojaCollector) []string {
	switch n := expr.(type) {
	case nil:
		return nil

	case *ast.StringLiteral:
		return singleValue(n.Value.String())

	case *ast.TemplateLiteral:
		return templateLiteralValues(n, collector)

	case *ast.Identifier:
		return collector.resolveIdentifier(n.Name.String())

	case *ast.AssignExpression:
		values := extractExpressionStrings(n.Right, collector)
		collector.assignExpression(n.Left, values)
		return values

	case *ast.ConditionalExpression:
		values := extractExpressionStrings(n.Consequent, collector)
		values = append(values, extractExpressionStrings(n.Alternate, collector)...)
		return values

	case *ast.SequenceExpression:
		values := []string{}
		for _, item := range n.Sequence {
			values = append(values, extractExpressionStrings(item, collector)...)
		}
		return values

	case *ast.BinaryExpression:
		if n.Operator == token.PLUS {
			left := extractExpressionStrings(n.Left, collector)
			right := extractExpressionStrings(n.Right, collector)
			return combineValues(left, right)
		}
		values := extractExpressionStrings(n.Left, collector)
		values = append(values, extractExpressionStrings(n.Right, collector)...)
		return values

	case *ast.ObjectLiteral:
		return objectLiteralValues(n, collector)

	case *ast.ArrayLiteral:
		return arrayLiteralValues(n, collector)

	case *ast.CallExpression:
		return callExpressionValues(n, collector)

	case *ast.NewExpression:
		values := []string{}
		for _, arg := range n.ArgumentList {
			values = append(values, extractExpressionStrings(arg, collector)...)
		}
		return values

	case *ast.BracketExpression:
		values := extractExpressionStrings(n.Left, collector)
		values = append(values, extractExpressionStrings(n.Member, collector)...)
		return values

	case *ast.DotExpression:
		resolved := collector.resolveIdentifier(calleeName(n))
		if len(resolved) > 0 {
			return resolved
		}
		return extractExpressionStrings(n.Left, collector)

	case *ast.UnaryExpression:
		return extractExpressionStrings(n.Operand, collector)

	case *ast.FunctionLiteral:
		collector.walkFunctionLiteral(n)
		return nil
	}

	return nil
}

func objectLiteralValues(obj *ast.ObjectLiteral, collector *gojaCollector) []string {
	if obj == nil {
		return nil
	}

	values := []string{}
	for _, item := range obj.Value {
		switch prop := item.(type) {
		case *ast.PropertyKeyed:
			values = append(values, extractExpressionStrings(prop.Value, collector)...)
		case *ast.PropertyShort:
			values = append(values, extractExpressionStrings(&prop.Name, collector)...)
			values = append(values, extractExpressionStrings(prop.Initializer, collector)...)
		case *ast.SpreadElement:
			values = append(values, extractExpressionStrings(prop.Expression, collector)...)
		}
	}
	return values
}

func arrayLiteralValues(arr *ast.ArrayLiteral, collector *gojaCollector) []string {
	if arr == nil {
		return nil
	}

	values := []string{}
	for _, item := range arr.Value {
		values = append(values, extractExpressionStrings(item, collector)...)
	}
	return values
}

func callExpressionValues(call *ast.CallExpression, collector *gojaCollector) []string {
	if call == nil {
		return nil
	}

	callee := calleeName(call.Callee)
	values := []string{}

	switch callee {
	case "fetch", "axios", "$.get", "$.post", "$.ajax", "jQuery.get", "jQuery.post", "jQuery.ajax":
		if len(call.ArgumentList) > 0 {
			values = append(values, extractExpressionStrings(call.ArgumentList[0], collector)...)
		}
	case "axios.get", "axios.post", "axios.put", "axios.patch", "axios.delete", "axios.request":
		if len(call.ArgumentList) > 0 {
			values = append(values, extractExpressionStrings(call.ArgumentList[0], collector)...)
		}
	case "XMLHttpRequest.open":
		if len(call.ArgumentList) > 1 {
			values = append(values, extractExpressionStrings(call.ArgumentList[1], collector)...)
		}
	}

	for _, arg := range call.ArgumentList {
		values = append(values, extractExpressionStrings(arg, collector)...)
	}
	return values
}

func templateLiteralValues(lit *ast.TemplateLiteral, collector *gojaCollector) []string {
	if lit == nil || len(lit.Elements) == 0 {
		return nil
	}

	values := []string{""}
	for i, elem := range lit.Elements {
		raw := ""
		if elem != nil {
			raw = elem.Literal
		}

		values = combineValues(values, []string{raw})

		if i < len(lit.Expressions) {
			exprValues := extractExpressionStrings(lit.Expressions[i], collector)
			if len(exprValues) == 0 {
				exprValues = []string{""}
			}
			values = combineValues(values, exprValues)
		}
	}

	return values
}

func combineValues(left, right []string) []string {
	switch {
	case len(left) == 0 && len(right) == 0:
		return nil
	case len(left) == 0:
		return cloneValues(right)
	case len(right) == 0:
		return cloneValues(left)
	}

	values := make([]string, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			values = append(values, concatJSValues(l, r))
		}
	}
	return values
}

// for "/api" + "/users" = "/api/users"
func concatJSValues(left, right string) string {
	left = NormalizeEndpoint(left)
	right = NormalizeEndpoint(right)

	switch {
	case left == "":
		return right
	case right == "":
		return left
	default:
		return left + right
	}
}

func cloneValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func calleeName(expr ast.Expression) string {
	switch n := expr.(type) {
	case *ast.Identifier:
		return n.Name.String()
	case *ast.DotExpression:
		left := calleeName(n.Left)
		if left == "" {
			return n.Identifier.Name.String()
		}
		return left + "." + n.Identifier.Name.String()
	case *ast.BracketExpression:
		return calleeName(n.Left)
	}
	return ""
}

func bindingName(binding *ast.Binding) string {
	if binding == nil {
		return ""
	}
	return bindingTargetName(binding.Target)
}

func bindingTargetName(target ast.BindingTarget) string {
	if target == nil {
		return ""
	}
	if ident, ok := target.(*ast.Identifier); ok {
		return ident.Name.String()
	}
	return ""
}

func extractFromSource(data string) []Endpoint {
	fields := strings.FieldsFunc(data, func(r rune) bool {
		switch r {
		case ' ', '\n', '\r', '\t', '"', '\'', '`', '(', ')', '[', ']', '{', '}', ',', ';':
			return true
		default:
			return false
		}
	})

	endpoints := make([]Endpoint, 0, len(fields))
	for _, field := range fields {
		field = cleanToken(field)
		if !IsLikelyEndpoint(field) {
			continue
		}

		endpoints = append(endpoints, Endpoint{
			Endpoint: field,
			Type:     ClassifyEndpoint(field),
		})
	}

	return endpoints
}

func cleanToken(value string) string {
	value = NormalizeEndpoint(value)
	value = strings.Trim(value, "+")
	value = strings.Trim(value, ".")
	value = strings.Trim(value, ":")

	for {
		trimmed := strings.TrimRight(value, ")}]>;,")
		if trimmed == value {
			break
		}
		value = trimmed
	}

	return value
}

func singleValue(value string) []string {
	value = NormalizeEndpoint(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

