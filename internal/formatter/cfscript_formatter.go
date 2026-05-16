package formatter

// cfscript_formatter.go — recursive pretty-printer for the cfscript sub-grammar.

import (
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ─── entry point ─────────────────────────────────────────────────────────────

// formatScriptNode dispatches a cfscript statement node and writes output.
// It is called for every direct child of a <cfscript> block or a block body.
func (f *Formatter) formatScriptNode(n *sitter.Node) {
	switch n.Kind() {
	// ── trivial whitespace tokens emitted by the grammar ──────────────────
	case "\n", "\r\n", "\r":
		// handled by caller's blank-line logic; skip
		return

	// ── comments ──────────────────────────────────────────────────────────
	case "comment":
		f.scriptLineComment(n)
	case "block_comment":
		f.scriptBlockComment(n)

	// ── declarations ──────────────────────────────────────────────────────
	case "component_declaration", "component":
		f.scriptComponent(n)
	case "function_definition", "function_declaration", "method_definition":
		f.scriptFunction(n)
	case "property_declaration":
		f.scriptProperty(n)
	case "variable_declaration":
		f.scriptVarDecl(n)

	// ── statements ────────────────────────────────────────────────────────
	case "expression_statement":
		f.scriptExprStmt(n)
	case "return_statement":
		f.scriptReturn(n)
	case "throw_statement":
		f.scriptThrow(n)
	case "break_statement":
		f.scriptBreak(n)
	case "continue_statement":
		f.scriptContinue(n)
	case "if_statement":
		f.scriptIf(n)
	case "switch_statement":
		f.scriptSwitch(n)
	case "while_statement":
		f.scriptWhile(n)
	case "do_statement":
		f.scriptDo(n)
	case "for_statement":
		f.scriptFor(n)
	case "for_in_statement", "for_of_statement":
		f.scriptForIn(n)
	case "try_statement":
		f.scriptTry(n)
	case "import_statement":
		f.scriptPassthru(n)

	// ── block (anonymous body) ─────────────────────────────────────────────
	case "statement_block", "block":
		// A bare block `{ ... }` not attached to anything.
		f.scriptBlock(n)

	// ── fallback: unknown node → emit raw, re-indented ────────────────────
	default:
		f.scriptRaw(n)
	}
}

// ─── helpers shared by all script formatters ─────────────────────────────────

// scriptWrite writes s, prepending indentation if we are at the beginning of
// a line. This is the primary output primitive for script formatting.
func (f *Formatter) scriptWrite(s string) { f.write(s) }
func (f *Formatter) scriptNL()            { f.nl() }

// parenExpr renders an expression wrapped in parens, avoiding double-wrapping
// when the node is already a parenthesized_expression.
// Long conditions are broken at logical operators.
func (f *Formatter) parenExpr(n *sitter.Node) string {
	var inner string
	if n != nil && n.Kind() == "parenthesized_expression" {
		inner = f.expr(n)
	} else {
		inner = "( " + f.expr(n) + " )"
	}
	// If the condition is too long, break at logical operators.
	// Use the larger of lineLen or indent estimate for the check.
	col := f.lineLen
	if col == 0 {
		col = len(f.opts.indent(f.level))
	}
	if col+len(inner) > f.opts.LineWidth {
		return f.normalizeCond(inner)
	}
	return inner
}

// iLine emits one indented line inside a script context.
func (f *Formatter) iLine(s string) {
	f.scriptNL()
	f.writeIndent()
	f.scriptWrite(s)
}

// scriptChildren iterates named statement-level children and dispatches each.
func (f *Formatter) scriptChildren(n *sitter.Node) {
	prevWasBlock := false
	prevWasMultiLine := false
	prevWasNamed := false
	prevEndRow := int(n.StartPosition().Row)
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() {
			continue
		}
		// Insert blank line after a block/multi-line statement, before a block, or when the source had one.
		startRow := int(c.StartPosition().Row)
		sourceGap := startRow-prevEndRow > 1
		if prevWasBlock || prevWasMultiLine || (prevWasNamed && isScriptBlockStmt(c)) || (prevWasNamed && sourceGap) {
			f.scriptWrite("\n")
		}
		f.formatScriptNode(c)
		prevWasBlock = isScriptBlockStmt(c)
		prevWasMultiLine = int(c.EndPosition().Row)-int(c.StartPosition().Row) > 0
		prevWasNamed = true
		prevEndRow = int(c.EndPosition().Row)
	}
}

func isScriptBlockStmt(n *sitter.Node) bool {
	switch n.Kind() {
	case "if_statement", "for_statement", "for_in_statement", "for_of_statement",
		"while_statement", "do_statement", "switch_statement", "try_statement":
		return true
	}
	return false
}

// scriptBlock renders a `{ ... }` block, indenting its contents.
func (f *Formatter) scriptBlock(n *sitter.Node) {
	f.scriptWrite(" {")
	f.scriptWrite("\n\n")
	f.level++
	f.scriptChildren(n)
	f.scriptWrite("\n")
	f.level--
	f.writeIndent()
	f.scriptWrite("}")
}

// scriptBlockOf renders the named child at field `field` as a block.
// Falls back to scriptBlock on the node directly if field lookup fails.
// func (f *Formatter) scriptBlockOf(n *sitter.Node, field string) {
// 	body := n.ChildByFieldName(field)
// 	if body == nil {
// 		// try last child heuristic
// 		body = n.Child(n.ChildCount() - 1)
// 	}
// 	if body.Kind() == "statement_block" || body.Kind() == "block" {
// 		f.scriptBlock(body)
// 	} else {
// 		// single-statement body — still wrap in braces for canonical form
// 		f.scriptWrite(" {")
// 		f.scriptWrite("\n")
// 		f.level++
// 		f.formatScriptNode(body)
// 		f.scriptWrite("\n")
// 		f.level--
// 		f.writeIndent()
// 		f.scriptWrite("}")
// 	}
// }

// expr renders an expression node inline and returns the string.
// Expressions are never written directly; they are embedded in statements.
func (f *Formatter) expr(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	// ── literals ──────────────────────────────────────────────────────────
	case "identifier", "property_identifier", "shorthand_property_identifier",
		"number", "true", "false", "null", "undefined", "this", "super":
		return f.text(n)

	case "string", "template_string":
		return f.exprString(n)

	// ── operators / compound expressions ──────────────────────────────────
	case "assignment_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		op := f.childToken(n, "=") // default
		for i := uint(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if !c.IsNamed() && c.Kind() != "=" {
				t := c.Kind()
				if strings.HasSuffix(t, "=") {
					op = t
				}
			}
		}
		leftStr := f.expr(left)
		rightStr := f.expr(right)
		result := fmt.Sprintf("%s %s %s", leftStr, op, rightStr)
		if len(result) > f.opts.LineWidth && !strings.Contains(rightStr, "\n") {
			f.level++
			rightStr = f.expr(right)
			f.level--
			if strings.Contains(rightStr, "\n") {
				indent := f.opts.indent(f.level + 1)
				return fmt.Sprintf("%s %s\n%s%s", leftStr, op, indent, rightStr)
			}
		}
		return result

	case "augmented_assignment_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		op := f.operatorToken(n)
		return fmt.Sprintf("%s %s %s", f.expr(left), op, f.expr(right))

	case "binary_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		op := f.operatorToken(n)
		if op == "" {
			op = f.gapOperator(n, left, right)
		}
		return fmt.Sprintf("%s %s %s", f.expr(left), op, f.expr(right))

	case "unary_expression":
		op := n.ChildByFieldName("operator")
		arg := n.ChildByFieldName("argument")
		if op == nil {
			op = n.Child(0)
		}
		opStr := f.text(op)
		// word operators need a space: typeof, void, delete, not
		if isWordOp(opStr) {
			return fmt.Sprintf("%s %s", opStr, f.expr(arg))
		}
		return fmt.Sprintf("%s%s", opStr, f.expr(arg))

	case "update_expression":
		arg := n.ChildByFieldName("argument")
		op := f.operatorToken(n)
		// prefix vs postfix
		if n.Child(0).Kind() == op || n.Child(0).Kind() == "++" || n.Child(0).Kind() == "--" {
			return fmt.Sprintf("%s%s", op, f.expr(arg))
		}
		return fmt.Sprintf("%s%s", f.expr(arg), op)

	case "ternary_expression":
		cond := n.ChildByFieldName("condition")
		cons := n.ChildByFieldName("consequence")
		alt := n.ChildByFieldName("alternative")
		condStr := f.expr(cond)
		consStr := f.expr(cons)
		altStr := f.expr(alt)
		inline := fmt.Sprintf("%s ? %s : %s", condStr, consStr, altStr)
		if len(inline) > f.opts.LineWidth {
			indent := f.opts.indent(f.level + 1)
			return condStr + "\n" + indent + "? " + consStr + "\n" + indent + ": " + altStr
		}
		return inline

	case "call_expression":
		fn := n.ChildByFieldName("function")
		args := n.ChildByFieldName("arguments")
		fnStr := f.expr(fn)
		// If fn has a chain break, evaluate args at deeper level.
		chainBroken := strings.Contains(fnStr, "\n")
		if chainBroken {
			f.level++
		}
		argsStr := f.exprArgs(args)
		result := fnStr + argsStr
		// If the full call exceeds line width and args are inline, split args.
		if !strings.Contains(argsStr, "\n") && len(result) > f.opts.LineWidth && args != nil && args.NamedChildCount() > 0 {
			f.level++
			var parts []string
			for i := uint(0); i < args.NamedChildCount(); i++ {
				parts = append(parts, f.expr(args.NamedChild(i)))
			}
			indent := f.opts.indent(f.level)
			f.level--
			outerIndent := f.opts.indent(f.level)
			argsStr = "(\n" + indent + strings.Join(parts, ",\n"+indent) + "\n" + outerIndent + ")"
		}
		if chainBroken {
			f.level--
		}
		return fnStr + argsStr

	case "new_expression":
		ctor := n.ChildByFieldName("constructor")
		args := n.ChildByFieldName("arguments")
		return fmt.Sprintf("new %s%s", f.expr(ctor), f.exprArgs(args))

	case "member_expression":
		obj := n.ChildByFieldName("object")
		prop := n.ChildByFieldName("property")
		objStr := f.expr(obj)
		propStr := f.expr(prop)
		inline := objStr + "." + propStr
		// Break if the object part is multi-line or the last line exceeds width.
		lastLine := inline
		if idx := strings.LastIndexByte(inline, '\n'); idx >= 0 {
			lastLine = inline[idx+1:]
		}
		if len(lastLine) > f.opts.LineWidth &&
			obj != nil && (obj.Kind() == "call_expression" || obj.Kind() == "member_expression") {
			indent := f.opts.indent(f.level + 1)
			return objStr + "\n" + indent + "." + propStr
		}
		return inline

	case "subscript_expression":
		obj := n.ChildByFieldName("object")
		idx := n.ChildByFieldName("index")
		return fmt.Sprintf("%s[%s]", f.expr(obj), f.expr(idx))

	case "parenthesized_expression":
		inner := n.NamedChild(0)
		return fmt.Sprintf("( %s )", f.expr(inner))

	case "sequence_expression":
		// comma-separated list
		var parts []string
		for i := uint(0); i < n.NamedChildCount(); i++ {
			parts = append(parts, f.expr(n.NamedChild(i)))
		}
		return strings.Join(parts, ", ")

	// ── spread / rest ─────────────────────────────────────────────────────
	case "spread_element":
		return "..." + f.expr(n.NamedChild(0))

	// ── array / object literals ───────────────────────────────────────────
	case "array":
		return f.exprArray(n)

	case "object":
		return f.exprObject(n)

	case "pair":
		key := n.ChildByFieldName("key")
		val := n.ChildByFieldName("value")
		keyStr := f.expr(key)
		valStr := f.expr(val)
		result := fmt.Sprintf("%s: %s", keyStr, valStr)
		if len(result) > f.opts.LineWidth && !strings.Contains(valStr, "\n") {
			f.level++
			valStr = f.expr(val)
			f.level--
			if strings.Contains(valStr, "\n") {
				indent := f.opts.indent(f.level + 1)
				return fmt.Sprintf("%s:\n%s%s", keyStr, indent, valStr)
			}
		}
		return result

	// ── functions ─────────────────────────────────────────────────────────
	case "arrow_function":
		return f.exprArrow(n)

	case "function_expression":
		return f.exprFunctionExpr(n)

	// ── type cast (CF-specific syntax) ────────────────────────────────────
	case "type_cast_expression":
		t := n.ChildByFieldName("type")
		val := n.ChildByFieldName("value")
		return fmt.Sprintf("(%s) %s", f.text(t), f.expr(val))

	// ── fallback ─────────────────────────────────────────────────────────
	default:
		return f.text(n)
	}
}

// ─── helpers for expr ────────────────────────────────────────────────────────

func isWordOp(op string) bool {
	switch op {
	case "typeof", "void", "delete", "not", "NOT":
		return true
	}
	return false
}

// operatorToken finds the operator anonymous token in a binary/unary expression.
func (f *Formatter) operatorToken(n *sitter.Node) string {
	op := n.ChildByFieldName("operator")
	if op != nil {
		return f.text(op)
	}
	// Fallback: first anonymous child
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() {
			return c.Kind()
		}
	}
	return ""
}

// gapOperator extracts a word operator (EQ, GT, AND, etc.) from the source
// text between left and right children of a binary_expression.
func (f *Formatter) gapOperator(_ *sitter.Node, left, right *sitter.Node) string {
	if left == nil || right == nil {
		return ""
	}
	gap := strings.TrimSpace(string(f.src[left.EndByte():right.StartByte()]))
	return gap
}

// childToken returns the first anonymous token child matching typ.
func (f *Formatter) childToken(n *sitter.Node, typ string) string {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() && c.Kind() == typ {
			return typ
		}
	}
	return typ
}

func (f *Formatter) exprArgs(args *sitter.Node) string {
	if args == nil {
		return "()"
	}
	var parts []string
	for i := uint(0); i < args.NamedChildCount(); i++ {
		parts = append(parts, f.expr(args.NamedChild(i)))
	}
	inline := "(" + strings.Join(parts, ", ") + ")"
	// Break onto separate lines if >3 arguments or inline exceeds line width.
	shouldBreak := len(parts) > 3 ||
		(len(parts) > 0 && len(inline) > f.opts.LineWidth)
	if shouldBreak {
		// Re-evaluate at deeper level so nested splits indent correctly.
		f.level++
		parts = parts[:0]
		for i := uint(0); i < args.NamedChildCount(); i++ {
			parts = append(parts, f.expr(args.NamedChild(i)))
		}
		indent := f.opts.indent(f.level)
		f.level--
		outerIndent := f.opts.indent(f.level)
		return "(\n" + indent + strings.Join(parts, ",\n"+indent) + "\n" + outerIndent + ")"
	}
	return inline
}

func (f *Formatter) exprArray(n *sitter.Node) string {
	if n.NamedChildCount() == 0 {
		return "[]"
	}
	var parts []string
	for i := uint(0); i < n.NamedChildCount(); i++ {
		parts = append(parts, f.expr(n.NamedChild(i)))
	}
	inline := "[" + strings.Join(parts, ", ") + "]"
	if f.lineLen+len(inline) <= f.opts.LineWidth {
		return inline
	}
	indent := f.indented() + f.opts.indent(1)
	return "[\n" + indent +
		strings.Join(parts, ",\n"+indent) +
		"\n" + f.indented() + "]"
}

func (f *Formatter) exprObject(n *sitter.Node) string {
	if n.NamedChildCount() == 0 {
		return "{}"
	}
	var parts []string
	for i := uint(0); i < n.NamedChildCount(); i++ {
		parts = append(parts, f.expr(n.NamedChild(i)))
	}
	joined := strings.Join(parts, ", ")
	inline := "{ " + joined + " }"
	if f.lineLen+len(inline) <= f.opts.LineWidth {
		return inline
	}
	indent := f.indented() + f.opts.indent(1)
	return "{\n" + indent +
		strings.Join(parts, ",\n"+indent) +
		"\n" + f.indented() + "}"
}

func (f *Formatter) exprString(n *sitter.Node) string {
	// Return the raw text; the grammar preserves the quote style.
	return f.text(n)
}

func (f *Formatter) exprArrow(n *sitter.Node) string {
	params := n.ChildByFieldName("parameters")
	body := n.ChildByFieldName("body")
	var paramStr string
	if params != nil {
		paramStr = f.exprParams(params)
	} else {
		// single param without parens
		p := n.ChildByFieldName("parameter")
		if p != nil {
			paramStr = f.text(p)
		}
	}
	if body != nil && (body.Kind() == "statement_block" || body.Kind() == "block") {
		// Render block inline for arrow functions; full block would need
		// newlines which aren't valid inside an expression context here.
		return fmt.Sprintf("%s => %s", paramStr, f.text(body))
	}
	return fmt.Sprintf("%s => %s", paramStr, f.expr(body))
}

func (f *Formatter) exprFunctionExpr(n *sitter.Node) string {
	name := n.ChildByFieldName("name")
	params := n.ChildByFieldName("parameters")
	body := n.ChildByFieldName("body")
	nameStr := ""
	if name != nil {
		nameStr = " " + f.text(name)
	}
	return fmt.Sprintf("function%s%s %s", nameStr, f.exprParams(params), f.text(body))
}

// exprParams renders a formal_parameters / parameter_list node.
func (f *Formatter) exprParams(params *sitter.Node) string {
	if params == nil {
		return "()"
	}
	// Check if parameters use the flat structure (function_declaration grammar)
	// where required/type/name are siblings separated by commas, rather than
	// being wrapped in required_parameter/optional_parameter nodes.
	if f.hasFlatParams(params) {
		return "(" + f.flatParams(params) + ")"
	}
	var parts []string
	for i := uint(0); i < params.NamedChildCount(); i++ {
		parts = append(parts, f.exprParam(params.NamedChild(i)))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// exprFuncDefParams renders function definition parameters, each on its own line.
func (f *Formatter) exprFuncDefParams(params *sitter.Node) string {
	if params == nil {
		return "()"
	}
	var parts []string
	if f.hasFlatParams(params) {
		// Parse flat params into individual param strings.
		var current []string
		for i := uint(0); i < params.ChildCount(); i++ {
			c := params.Child(i)
			switch c.Kind() {
			case "(", ")":
				continue
			case ",":
				if len(current) > 0 {
					parts = append(parts, strings.Join(current, " "))
					current = nil
				}
			case "required":
				current = append(current, "required")
			case "parameter_type":
				current = append(current, f.text(c.Child(0)))
			case "identifier":
				current = append(current, f.text(c))
			case "assignment_pattern":
				left := c.ChildByFieldName("left")
				right := c.ChildByFieldName("right")
				current = append(current, fmt.Sprintf("%s = %s", f.expr(left), f.expr(right)))
			default:
				if c.IsNamed() {
					current = append(current, f.text(c))
				}
			}
		}
		if len(current) > 0 {
			parts = append(parts, strings.Join(current, " "))
		}
	} else {
		for i := uint(0); i < params.NamedChildCount(); i++ {
			parts = append(parts, f.exprParam(params.NamedChild(i)))
		}
	}
	if len(parts) == 0 {
		return "()"
	}
	indent := f.opts.indent(f.level + 1)
	var sb strings.Builder
	sb.WriteString("(\n")
	for i, p := range parts {
		sb.WriteString(indent)
		sb.WriteString(p)
		if i < len(parts)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString(f.opts.indent(f.level))
	sb.WriteString(")")
	return sb.String()
}

// hasFlatParams returns true if formal_parameters uses the flat structure
// (parameter_type as a direct child rather than wrapped in required_parameter).
func (f *Formatter) hasFlatParams(params *sitter.Node) bool {
	for i := uint(0); i < params.ChildCount(); i++ {
		if params.Child(i).Kind() == "parameter_type" {
			return true
		}
	}
	return false
}

// flatParams renders parameters from the flat formal_parameters structure
// used by function_declaration: [required] [type] name [= default], ...
func (f *Formatter) flatParams(params *sitter.Node) string {
	var result []string
	var current []string
	for i := uint(0); i < params.ChildCount(); i++ {
		c := params.Child(i)
		switch c.Kind() {
		case "(", ")":
			continue
		case ",":
			if len(current) > 0 {
				result = append(result, strings.Join(current, " "))
				current = nil
			}
		case "required":
			current = append(current, "required")
		case "parameter_type":
			current = append(current, f.text(c.Child(0)))
		case "identifier":
			current = append(current, f.text(c))
		case "assignment_pattern":
			left := c.ChildByFieldName("left")
			right := c.ChildByFieldName("right")
			current = append(current, fmt.Sprintf("%s = %s", f.expr(left), f.expr(right)))
		default:
			if c.IsNamed() {
				current = append(current, f.text(c))
			}
		}
	}
	if len(current) > 0 {
		result = append(result, strings.Join(current, " "))
	}
	return strings.Join(result, ", ")
}

func (f *Formatter) exprParam(n *sitter.Node) string {
	switch n.Kind() {
	case "identifier":
		return f.text(n)
	case "assignment_pattern":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		return fmt.Sprintf("%s = %s", f.expr(left), f.expr(right))
	case "rest_pattern":
		return "..." + f.expr(n.NamedChild(0))
	case "required_parameter", "optional_parameter":
		// cfscript-specific: [required] [type] name [= default]
		return f.cfParam(n)
	default:
		return f.text(n)
	}
}

// cfParam renders a CF-style parameter declaration inside a function signature.
func (f *Formatter) cfParam(n *sitter.Node) string {
	var parts []string
	required := n.ChildByFieldName("required")
	typ := n.ChildByFieldName("type")
	name := n.ChildByFieldName("name")
	defVal := n.ChildByFieldName("default_value")
	if required != nil {
		parts = append(parts, f.text(required))
	}
	if typ != nil {
		parts = append(parts, f.text(typ))
	}
	if name != nil {
		parts = append(parts, f.text(name))
	}
	result := strings.Join(parts, " ")
	if defVal != nil {
		result += " = " + f.expr(defVal)
	}
	return result
}

// ─── statement formatters ─────────────────────────────────────────────────────

func (f *Formatter) scriptLineComment(n *sitter.Node) {
	f.iLine(f.text(n))
	f.scriptWrite("\n")
}

func (f *Formatter) scriptBlockComment(n *sitter.Node) {
	raw := f.text(n)
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if i == 0 {
			f.iLine(strings.TrimRight(line, " \t"))
		} else {
			f.scriptWrite("\n")
			f.writeIndent()
			f.scriptWrite(strings.TrimRight(line, " \t"))
		}
	}
	f.scriptWrite("\n")
}

// scriptComponent renders a CFC component declaration:
//
//	component [extends="X"] [implements="Y"] { ... }
func (f *Formatter) scriptComponent(n *sitter.Node) {
	// Collect modifiers / attributes that come before the body
	var attrs []string
	var body *sitter.Node
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "statement_block", "block", "class_body", "component_body":
			body = c
		case "identifier":
			// 'component' keyword itself — skip
		default:
			if c.IsNamed() {
				attrs = append(attrs, f.text(c))
			}
		}
	}
	header := "component"
	if len(attrs) > 0 {
		header += " " + strings.Join(attrs, " ")
	}
	f.iLine(header)
	if body != nil {
		f.scriptBlock(body)
	}
	f.scriptWrite("\n")
}

// scriptFunction renders a function definition:
//
//	[access] [returnType] function name([params]) { body }
func (f *Formatter) scriptFunction(n *sitter.Node) {
	// Gather prefix tokens (access modifier, return type) and the name.
	var prefix []string
	name := n.ChildByFieldName("name")
	params := n.ChildByFieldName("parameters")
	body := n.ChildByFieldName("body")
	retType := n.ChildByFieldName("return_type")

	// Walk children to pick up access modifiers that have no field name.
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		fieldName := n.FieldNameForChild(uint32(i))
		switch fieldName {
		case "name", "parameters", "body", "return_type":
			continue
		}
		if !c.IsNamed() {
			t := c.Kind()
			if t == "function" {
				// keyword
				continue
			}
		}
		if c.IsNamed() {
			prefix = append(prefix, f.text(c))
		}
	}

	var sig strings.Builder
	if len(prefix) > 0 {
		sig.WriteString(strings.Join(prefix, " "))
		sig.WriteString(" ")
	}
	if retType != nil {
		sig.WriteString(f.text(retType))
		sig.WriteString(" ")
	}
	sig.WriteString("function ")
	if name != nil {
		sig.WriteString(f.text(name))
	}

	paramStr := f.exprFuncDefParams(params)
	sig.WriteString(paramStr)

	f.iLine(sig.String())
	if body != nil {
		f.scriptBlock(body)
	}
	f.scriptWrite("\n")
}

// scriptProperty renders a CFC property declaration:
//
//	property [type] name [= default];
func (f *Formatter) scriptProperty(n *sitter.Node) {
	f.iLine(strings.TrimSpace(f.text(n)))
	f.scriptWrite("\n")
}

// scriptVarDecl renders: var/local name [= expr][, name [= expr]];
func (f *Formatter) scriptVarDecl(n *sitter.Node) {
	// keyword: var or local
	keyword := "var"
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !c.IsNamed() {
			t := c.Kind()
			if t == "var" || t == "local" {
				keyword = t
				break
			}
		}
	}
	var decls []string
	for i := uint(0); i < n.NamedChildCount(); i++ {
		d := n.NamedChild(i)
		switch d.Kind() {
		case "variable_declarator":
			vname := d.ChildByFieldName("name")
			vval := d.ChildByFieldName("value")
			s := f.expr(vname)
			if vval != nil {
				s += " = " + f.expr(vval)
			}
			decls = append(decls, s)
		default:
			decls = append(decls, f.expr(d))
		}
	}
	f.iLine(fmt.Sprintf("%s %s;", keyword, strings.Join(decls, ", ")))
	f.scriptWrite("\n")
}

func (f *Formatter) scriptExprStmt(n *sitter.Node) {
	// The named child is the expression; anonymous child is ";"
	inner := n.NamedChild(0)
	if inner == nil {
		return
	}
	f.iLine(f.expr(inner) + ";")
	f.scriptWrite("\n")
}

func (f *Formatter) scriptReturn(n *sitter.Node) {
	val := n.NamedChild(0)
	if val == nil {
		f.iLine("return;")
	} else {
		f.iLine("return " + f.expr(val) + ";")
	}
	f.scriptWrite("\n")
}

func (f *Formatter) scriptThrow(n *sitter.Node) {
	val := n.NamedChild(0)
	// For throw(arg, arg, ...) — split args onto new lines if too long.
	if val != nil && val.Kind() == "parenthesized_expression" {
		inner := val.NamedChild(0)
		if inner != nil && inner.Kind() == "sequence_expression" {
			var parts []string
			for i := uint(0); i < inner.NamedChildCount(); i++ {
				parts = append(parts, f.expr(inner.NamedChild(i)))
			}
			inline := "throw (" + strings.Join(parts, ", ") + ");"
			indentLen := len(f.opts.indent(f.level))
			if indentLen+len(inline) > f.opts.LineWidth {
				indent := f.opts.indent(f.level + 1)
				outerIndent := f.opts.indent(f.level)
				f.iLine("throw (\n" + indent + strings.Join(parts, ",\n"+indent) + "\n" + outerIndent + ");")
				f.scriptWrite("\n")
				return
			}
		}
	}
	f.iLine("throw " + f.expr(val) + ";")
	f.scriptWrite("\n")
}

func (f *Formatter) scriptBreak(n *sitter.Node) {
	label := n.NamedChild(0)
	if label != nil {
		f.iLine("break " + f.text(label) + ";")
	} else {
		f.iLine("break;")
	}
	f.scriptWrite("\n")
}

func (f *Formatter) scriptContinue(n *sitter.Node) {
	label := n.NamedChild(0)
	if label != nil {
		f.iLine("continue " + f.text(label) + ";")
	} else {
		f.iLine("continue;")
	}
	f.scriptWrite("\n")
}

// scriptIf renders if / else if / else chains.
func (f *Formatter) scriptIf(n *sitter.Node) {
	cond := n.ChildByFieldName("condition")
	cons := n.ChildByFieldName("consequence")
	alt := n.ChildByFieldName("alternative")

	f.iLine(fmt.Sprintf("if %s", f.parenExpr(cond)))
	f.scriptBlockOf2(cons)

	if alt != nil {
		switch alt.Kind() {
		case "else_clause":
			// else if or else
			inner := alt.NamedChild(0)
			if inner != nil && inner.Kind() == "if_statement" {
				f.scriptWrite(" else ")
				// Re-use scriptIf but write inline (no leading newline/indent).
				f.scriptIfInline(inner)
			} else {
				f.scriptWrite(" else")
				f.scriptBlockOf2(inner)
			}
		case "if_statement":
			f.scriptWrite(" else ")
			f.scriptIfInline(alt)
		default:
			f.scriptWrite(" else")
			f.scriptBlockOf2(alt)
		}
	}
	f.scriptWrite("\n")
}

// scriptIfInline is like scriptIf but does not prefix a newline+indent
// (used for `else if` continuation on the same line).
func (f *Formatter) scriptIfInline(n *sitter.Node) {
	cond := n.ChildByFieldName("condition")
	cons := n.ChildByFieldName("consequence")
	alt := n.ChildByFieldName("alternative")

	f.scriptWrite(fmt.Sprintf("if %s", f.parenExpr(cond)))
	f.scriptBlockOf2(cons)

	if alt != nil {
		switch alt.Kind() {
		case "else_clause":
			inner := alt.NamedChild(0)
			if inner != nil && inner.Kind() == "if_statement" {
				f.scriptWrite(" else ")
				f.scriptIfInline(inner)
			} else {
				f.scriptWrite(" else")
				f.scriptBlockOf2(inner)
			}
		default:
			f.scriptWrite(" else")
			f.scriptBlockOf2(alt)
		}
	}
}

// scriptBlockOf2 renders a statement as a braced block attached to the
// current line (e.g. the body of if/while/for).
func (f *Formatter) scriptBlockOf2(body *sitter.Node) {
	if body == nil {
		f.scriptWrite(" {}")
		return
	}
	if body.Kind() == "statement_block" || body.Kind() == "block" {
		f.scriptBlock(body)
	} else {
		f.scriptWrite(" {\n")
		f.level++
		f.formatScriptNode(body)
		f.level--
		f.writeIndent()
		f.scriptWrite("}")
	}
}

// scriptSwitch renders a switch statement.
func (f *Formatter) scriptSwitch(n *sitter.Node) {
	val := n.ChildByFieldName("value")
	body := n.ChildByFieldName("body")

	f.iLine(fmt.Sprintf("switch %s {", f.parenExpr(val)))
	f.scriptWrite("\n")
	f.level++
	if body != nil {
		for i := uint(0); i < body.NamedChildCount(); i++ {
			clause := body.NamedChild(i)
			switch clause.Kind() {
			case "switch_case":
				val2 := clause.ChildByFieldName("value")
				f.level--
				f.iLine(fmt.Sprintf("case %s:", f.expr(val2)))
				f.scriptWrite("\n")
				f.level++
				for j := uint(0); j < clause.NamedChildCount(); j++ {
					child := clause.NamedChild(j)
					if child == val2 {
						continue
					}
					f.formatScriptNode(child)
				}
			case "switch_default":
				f.level--
				f.iLine("default:")
				f.scriptWrite("\n")
				f.level++
				for j := uint(0); j < clause.NamedChildCount(); j++ {
					f.formatScriptNode(clause.NamedChild(j))
				}
			default:
				f.formatScriptNode(clause)
			}
		}
	}
	f.level--
	f.writeIndent()
	f.scriptWrite("}\n")
}

func (f *Formatter) scriptWhile(n *sitter.Node) {
	cond := n.ChildByFieldName("condition")
	body := n.ChildByFieldName("body")
	f.iLine(fmt.Sprintf("while %s", f.parenExpr(cond)))
	f.scriptBlockOf2(body)
	f.scriptWrite("\n")
}

func (f *Formatter) scriptDo(n *sitter.Node) {
	body := n.ChildByFieldName("body")
	cond := n.ChildByFieldName("condition")
	f.iLine("do")
	f.scriptBlockOf2(body)
	f.scriptWrite(fmt.Sprintf(" while %s;\n", f.parenExpr(cond)))
}

// scriptFor renders: for (init; cond; update) { body }
func (f *Formatter) scriptFor(n *sitter.Node) {
	init := n.ChildByFieldName("initializer")
	cond := n.ChildByFieldName("condition")
	incr := n.ChildByFieldName("increment")
	body := n.ChildByFieldName("body")

	initStr := f.forClause(init)
	condStr := f.forClause(cond)
	incrStr := f.forClause(incr)

	f.iLine(fmt.Sprintf("for ( %s; %s; %s )", initStr, condStr, incrStr))
	f.scriptBlockOf2(body)
	f.scriptWrite("\n")
}

// scriptForIn renders: for (var x in collection) { body }
// Also handles for...of.
func (f *Formatter) scriptForIn(n *sitter.Node) {
	left := n.ChildByFieldName("left")
	right := n.ChildByFieldName("right")
	body := n.ChildByFieldName("body")

	keyword := "in"
	if n.Kind() == "for_of_statement" {
		keyword = "of"
	}

	varKind := ""
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if n.FieldNameForChild(uint32(i)) == "kind" {
			varKind = f.text(c) + " "
			break
		}
	}

	f.iLine(fmt.Sprintf("for ( %s%s %s %s )", varKind, f.expr(left), keyword, f.expr(right)))
	f.scriptBlockOf2(body)
	f.scriptWrite("\n")
}

// forClause renders an optional for-loop clause (init/cond/incr).
func (f *Formatter) forClause(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	switch n.Kind() {
	case "empty_statement":
		return ""
	case "variable_declaration":
		// Inline version: var i = 0
		keyword := "var"
		for i := uint(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if !c.IsNamed() {
				t := c.Kind()
				if t == "var" || t == "local" {
					keyword = t
				}
			}
		}
		var decls []string
		for i := uint(0); i < n.NamedChildCount(); i++ {
			d := n.NamedChild(i)
			vname := d.ChildByFieldName("name")
			vval := d.ChildByFieldName("value")
			s := f.expr(vname)
			if vval != nil {
				s += " = " + f.expr(vval)
			}
			decls = append(decls, s)
		}
		return keyword + " " + strings.Join(decls, ", ")
	default:
		return f.expr(n)
	}
}

// scriptTry renders try / catch / finally.
func (f *Formatter) scriptTry(n *sitter.Node) {
	body := n.ChildByFieldName("body")
	handler := n.ChildByFieldName("handler")
	finalizer := n.ChildByFieldName("finalizer")

	f.iLine("try")
	if body != nil {
		f.scriptBlock(body)
	}

	if handler != nil {
		// catch_clause: catch (param) { body }
		param := handler.ChildByFieldName("parameter")
		hBody := handler.ChildByFieldName("body")
		if param != nil {
			f.scriptWrite(fmt.Sprintf(" catch (%s)", f.exprParam(param)))
		} else {
			f.scriptWrite(" catch")
		}
		if hBody != nil {
			f.scriptBlock(hBody)
		}
	}

	if finalizer != nil {
		fBody := finalizer.ChildByFieldName("body")
		if fBody == nil {
			fBody = finalizer
		}
		f.scriptWrite(" finally")
		f.scriptBlock(fBody)
	}
	f.scriptWrite("\n")
}

// scriptPassthru re-emits a node's text re-indented (last-resort fallback).
func (f *Formatter) scriptPassthru(n *sitter.Node) {
	f.iLine(strings.TrimSpace(f.text(n)))
	f.scriptWrite("\n")
}

// scriptRaw re-emits an unknown node line-by-line with indentation applied.
func (f *Formatter) scriptRaw(n *sitter.Node) {
	raw := strings.TrimSpace(f.text(n))
	if raw == "" {
		return
	}
	lines := strings.Split(raw, "\n")
	prevBlank := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevBlank {
				f.scriptWrite("\n")
			}
			prevBlank = true
			continue
		}
		prevBlank = false
		f.writeIndent()
		f.scriptWrite(trimmed + "\n")
	}
}
