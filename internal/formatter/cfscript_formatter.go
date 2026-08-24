package formatter

// cfscript_formatter.go — recursive pretty-printer for the cfscript sub-grammar.

import (
	"bytes"
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

// memberOperator returns the accessor token joining a member_expression's
// object and property. It is not always ".": Lucee and BoxLang spell static
// access `Widget::getData()`, and rendering that as `Widget.getData()` turns a
// static call into an instance call.
func memberOperator(n *sitter.Node) string {
	// `::` is reported as a named `static_chain` node, not an anonymous token.
	if sc := n.ChildByFieldName("static_chain"); sc != nil {
		return "::"
	}

	// Likewise `?.` is a named `optional_chain` node wrapping the anonymous
	// token, not a bare "?." child — the loop below skips every named child,
	// so without this check it fell through to the default "." and silently
	// turned a null-safe chain into one that throws.
	if oc := n.ChildByFieldName("optional_chain"); oc != nil {
		return "?."
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.IsNamed() {
			continue
		}

		switch c.Kind() {
		case ".", "::", "?.":
			return c.Kind()
		}
	}

	return "."
}

// writeInterveningComments emits any comment child of n lying strictly between
// byte offsets from and to, each on its own line. Comments in these positions —
// between a block and its `else`, or between `try` and its `catch` — belong to
// no field, so navigating to the continuation by field name skipped straight
// past them and deleted them. Reports whether anything was written, which tells
// the caller the continuation keyword can no longer sit on the closing brace.
func (f *Formatter) writeInterveningComments(n *sitter.Node, from, to uint) bool {
	wrote := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		switch c.Kind() {
		case "comment", "block_comment":
		default:
			continue
		}

		if c.StartByte() < from || c.EndByte() > to {
			continue
		}

		f.scriptWrite("\n")
		f.writeIndent()
		f.scriptWrite(strings.TrimSpace(f.text(c)))

		wrote = true
	}

	return wrote
}

// deferBlockComments queues the comments of n lying strictly between from and
// to for emission inside the next block that opens. A comment between a
// condition and an Allman-style brace belongs to no field, so navigating from
// the condition straight to the body skipped past it and deleted it. It cannot
// be emitted where it sits either: a "//" comment printed before the "{" would
// swallow the brace, so it is carried into the body instead.
func (f *Formatter) deferBlockComments(n, from, to *sitter.Node) {
	if n == nil || from == nil || to == nil {
		return
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		switch c.Kind() {
		case "comment", "block_comment", "cf_comment":
		default:
			continue
		}

		if c.StartByte() >= from.EndByte() && c.EndByte() <= to.StartByte() {
			f.pendingBlockComments = append(f.pendingBlockComments, c)
		}
	}
}

// elseBody returns the else clause's body, queueing any comments that precede
// it for emission inside that body. Such a comment is a named child like any
// other, so taking the first named child picked the comment as the body and
// rendered it in the body's place:
//
//	else //we have summary
//	{ ... }
//
// It cannot be left where it sits either — a "//" comment printed before the
// "{" swallows the brace.
func (f *Formatter) elseBody(alt *sitter.Node) *sitter.Node {
	var lead []*sitter.Node

	for i := uint(0); i < alt.NamedChildCount(); i++ {
		c := alt.NamedChild(i)

		switch c.Kind() {
		case "comment", "block_comment", "cf_comment":
			lead = append(lead, c)

			continue
		}

		f.pendingBlockComments = append(f.pendingBlockComments, lead...)

		return c
	}

	return nil
}

// flushBlockComments emits any comments queued by deferBlockComments. It is
// called just inside an opened block, so they land at the body's indentation
// exactly as a comment written there would — which is where a second format
// pass finds them, keeping the output stable.
func (f *Formatter) flushBlockComments() bool {
	pending := f.pendingBlockComments
	f.pendingBlockComments = nil

	for _, c := range pending {
		f.formatScriptNode(c)
	}

	return len(pending) > 0
}

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
//
// leadEmitted says whether something was already written into this body — a
// comment carried in from the construct's header by flushBlockComments. That
// comment is a sibling of the statements that follow and has to count as one:
// on the first format it was emitted outside this loop, so the statement after
// it saw no predecessor and got no blank line, while on the second format the
// comment had become an ordinary child and the blank line appeared. The file
// alternated between the two.
func (f *Formatter) scriptChildren(n *sitter.Node, leadEmitted bool) {
	prevWasBlock := false
	prevWasMultiLine := false
	prevWasNamed := leadEmitted
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

		outBefore := f.out.Len()
		f.formatScriptNode(c)

		prevWasBlock = isScriptBlockStmt(c)
		// Judge "was multi-line" from what was actually emitted, not from the
		// node's span in the source. The formatter expands single-line
		// statements into blocks (`function f() {}` becomes a braced body), so
		// reading the source made the separating blank line appear only once
		// the file had already been formatted — the output was not idempotent.
		//
		// The emitted text is trimmed first: every statement starts by opening
		// a new line, and counting that leading newline would class all of them
		// as multi-line and insert a blank line between every pair.
		prevWasMultiLine = false
		if f.out.Len() > outBefore {
			prevWasMultiLine = bytes.Contains(bytes.TrimSpace(f.out.Bytes()[outBefore:]), []byte{'\n'})
		}

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
	lead := f.flushBlockComments()
	f.scriptChildren(n, lead)
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
		text := f.text(n)
		if n.Kind() == "identifier" || n.Kind() == "property_identifier" {
			text = f.applyScopeCase(text)
		}

		return text

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
		cmts := f.delimitedComments(n)
		result := fmt.Sprintf("%s %s%s %s", leftStr, op, cmts, rightStr)

		if len(result) > f.opts.LineWidth && !strings.Contains(rightStr, "\n") {
			f.level++
			rightStr = f.expr(right)
			f.level--

			if strings.Contains(rightStr, "\n") {
				indent := f.opts.indent(f.level + 1)

				return fmt.Sprintf("%s %s%s\n%s%s", leftStr, op, cmts, indent, rightStr)
			}
		}

		return result

	case "augmented_assignment_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		op := f.operatorToken(n)

		return fmt.Sprintf("%s %s%s %s", f.expr(left), op, f.delimitedComments(n), f.expr(right))

	case "binary_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		op := f.operatorToken(n)
		cmts := f.delimitedComments(n)

		if op == "" {
			// gapOperator lifts the raw source between the operands, so
			// anything sitting in that gap — comments included — is already
			// carried across. Adding them again would emit them twice.
			op = f.gapOperator(n, left, right)
			cmts = ""
		}

		return fmt.Sprintf("%s %s%s %s", f.expr(left), op, cmts, f.expr(right))

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

			var isComment []bool

			for i := uint(0); i < args.NamedChildCount(); i++ {
				c := args.NamedChild(i)
				parts = append(parts, f.expr(c))
				isComment = append(isComment, c.Kind() == "cf_comment")
			}

			indent := f.opts.indent(f.level)
			f.level--
			outerIndent := f.opts.indent(f.level)

			var sb strings.Builder

			sb.WriteString("(\n")

			leading := f.opts.CommaPosition == "before"
			for i, p := range parts {
				if leading {
					if !isComment[i] && i > 0 {
						hasPrev := false

						for j := i - 1; j >= 0; j-- {
							if !isComment[j] {
								hasPrev = true

								break
							}
						}

						if hasPrev {
							sb.WriteString(indent)
							sb.WriteString(", ")
							sb.WriteString(p)
						} else {
							sb.WriteString(indent)
							sb.WriteString(p)
						}
					} else {
						sb.WriteString(indent)
						sb.WriteString(p)
					}
				} else {
					sb.WriteString(indent)
					sb.WriteString(p)

					if !isComment[i] {
						hasMore := false

						for j := i + 1; j < len(parts); j++ {
							if !isComment[j] {
								hasMore = true

								break
							}
						}

						if hasMore {
							sb.WriteString(",")
						}
					}
				}

				sb.WriteString("\n")
			}

			sb.WriteString(outerIndent)
			sb.WriteByte(')')
			argsStr = sb.String()
		}

		if chainBroken {
			f.level--
		}

		return fnStr + argsStr

	case "new_expression":
		ctor := n.ChildByFieldName("constructor")
		args := n.ChildByFieldName("arguments")

		// An inline component literal — `new component { property name="x"; function
		// f() {} }` — has neither a constructor nor an argument list: the class is
		// its body. The field-based rendering below found nothing in either field
		// and emitted `new ()`, deleting the keyword and the whole body with it
		// (16 files in the corpus, all rejected by the whitespaceOnly guard, so in
		// the editor this reads as format-on-save doing nothing).
		//
		// Emitted verbatim, in the same spirit as a function_expression's body:
		// it's a declaration rather than an expression to re-space, and rendering
		// it properly needs the statement machinery, which does not return a string.
		if ctor == nil && hasChildOfKind(n, "component_body") {
			return f.text(n)
		}

		// `new java:java.io.File(p)` — the type prefix is a single token that
		// already carries its colon, and dropping it changes which object gets
		// constructed, so it has to be reproduced verbatim.
		prefix := ""

		if p := n.ChildByFieldName("prefix"); p != nil {
			prefix = f.text(p)
		}

		return fmt.Sprintf("new %s%s%s", prefix, f.expr(ctor), f.exprArgs(args))

	case "member_expression":
		obj := n.ChildByFieldName("object")
		prop := n.ChildByFieldName("property")
		objStr := f.expr(obj)
		propStr := f.expr(prop)
		op := memberOperator(n)

		// A comment between a chained call and its next hop belongs to no
		// field, so joining object and property dropped it.
		if obj != nil && prop != nil {
			if comments := f.commentsBetween(n, obj.EndByte(), prop.StartByte()); len(comments) > 0 {
				indent := f.opts.indent(f.level + 1)

				var b strings.Builder

				b.WriteString(objStr)

				for _, c := range comments {
					b.WriteString("\n")
					b.WriteString(indent)
					b.WriteString(c)
				}

				b.WriteString("\n")
				b.WriteString(indent)
				b.WriteString(op)
				b.WriteString(propStr)

				return b.String()
			}
		}

		inline := objStr + op + propStr
		// Break if the object part is multi-line or the last line exceeds width.
		lastLine := inline
		if idx := strings.LastIndexByte(inline, '\n'); idx >= 0 {
			lastLine = inline[idx+1:]
		}

		if len(lastLine) > f.opts.LineWidth &&
			obj != nil && (obj.Kind() == "call_expression" || obj.Kind() == "member_expression") {
			indent := f.opts.indent(f.level + 1)

			return objStr + "\n" + indent + op + propStr
		}

		return inline

	case "subscript_expression":
		obj := n.ChildByFieldName("object")
		idx := n.ChildByFieldName("index")

		return fmt.Sprintf("%s[%s]", f.expr(obj), f.expr(idx))

	case "parenthesized_expression":
		// Every named child is rendered, not just the first. A comment inside
		// the parens — commonly a commented-out clause parked at the end of a
		// long condition — is a named child like any other, so taking child 0
		// dropped it, or worse rendered it as the expression itself.
		var sb strings.Builder

		sb.WriteString("( ")

		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)

			if i > 0 {
				sb.WriteString(" ")
			}

			switch c.Kind() {
			case "comment", "block_comment", "cf_comment":
				text := strings.TrimSpace(f.text(c))
				sb.WriteString(text)

				// A "//" comment runs to end of line, so without a break it
				// would swallow the rest of the condition and the ")".
				if strings.HasPrefix(text, "//") {
					sb.WriteString("\n")
				}
			default:
				sb.WriteString(f.expr(c))
			}
		}

		sb.WriteString(" )")

		return sb.String()

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

// delimitedComments returns the text of any delimited comment sitting directly
// inside n, prefixed with a space, or "" when there is none. Rendering an
// expression from its named fields alone drops these, and commenting an operand
// out mid-expression is a common way to park it:
//
//	<cfset cols = "a," &
//	    "b," &
//	<!---    "c," & --->
//	    "d">
//
// Only delimited forms are re-emitted. A "//" comment cannot be moved onto the
// same line as the operand that follows it without swallowing it, so it is left
// for the whitespace-only guard to report.
func (f *Formatter) delimitedComments(n *sitter.Node) string {
	var sb strings.Builder

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_comment", "block_comment":
			sb.WriteString(" ")
			sb.WriteString(strings.TrimSpace(f.text(c)))
		}
	}

	return sb.String()
}

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

// hasChildOfKind reports whether n has a direct child of the given kind.
func hasChildOfKind(n *sitter.Node, kind string) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		if n.Child(i).Kind() == kind {
			return true
		}
	}

	return false
}

// tagStyleArgs reports whether args is a script-syntax CF tag's attribute list —
// `cfdirectory(directory="x" action="create")` — rather than an ordinary argument
// list. The grammar models both as an `arguments` node holding assignment_expressions;
// what separates them is that the attribute form has no comma tokens at all. A single
// argument is ambiguous and treated as an ordinary call, since nothing is joined.
func tagStyleArgs(args *sitter.Node) bool {
	if args == nil || args.NamedChildCount() < 2 {
		return false
	}

	for i := uint(0); i < args.ChildCount(); i++ {
		if args.Child(i).Kind() == "," {
			return false
		}
	}

	return true
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

	var isComment []bool

	hasLineComment := false

	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		parts = append(parts, f.expr(c))
		isComment = append(isComment, isCommentKind(c.Kind()))

		if c.Kind() == "comment" {
			hasLineComment = true
		}
	}

	// A script-syntax CF tag call separates its attributes with spaces, not commas
	// (`cfdirectory(directory="#dir#" action="create")`), and the grammar hands both
	// forms over as an `arguments` node full of assignment_expressions. Joining with
	// ", " regardless inserted commas that were never in the source — a non-whitespace
	// change, so the guard rejected the file and format-on-save did nothing.
	useCommas := !tagStyleArgs(args)

	sep := ", "
	if !useCommas {
		sep = " "
	}

	inline := "(" + strings.Join(parts, sep) + ")"
	// Break onto separate lines if >3 arguments or inline exceeds line width.
	// A line comment forces the break unconditionally: joined inline it runs to
	// end of line and comments out every argument after it.
	shouldBreak := hasLineComment || len(parts) > 3 ||
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

		var sb strings.Builder

		sb.WriteString("(\n")

		// Space-separated attributes carry no separator to place, so neither comma
		// position applies to them.
		if !useCommas {
			for _, p := range parts {
				sb.WriteString(indent)
				sb.WriteString(p)
				sb.WriteString("\n")
			}

			sb.WriteString(outerIndent)
			sb.WriteByte(')')

			return sb.String()
		}

		leading := f.opts.CommaPosition == "before"
		for i, p := range parts {
			if leading {
				if !isComment[i] && i > 0 {
					hasPrev := false

					for j := i - 1; j >= 0; j-- {
						if !isComment[j] {
							hasPrev = true

							break
						}
					}

					if hasPrev {
						sb.WriteString(indent)
						sb.WriteString(", ")
						sb.WriteString(p)
					} else {
						sb.WriteString(indent)
						sb.WriteString(p)
					}
				} else {
					sb.WriteString(indent)
					sb.WriteString(p)
				}
			} else {
				sb.WriteString(indent)
				sb.WriteString(p)

				if !isComment[i] {
					hasMore := false

					for j := i + 1; j < len(parts); j++ {
						if !isComment[j] {
							hasMore = true

							break
						}
					}

					if hasMore {
						sb.WriteString(",")
					}
				}
			}

			sb.WriteString("\n")
		}

		sb.WriteString(outerIndent)
		sb.WriteByte(')')

		return sb.String()
	}

	return inline
}

// commentsBetween returns the text of any comment child of n lying strictly
// between byte offsets from and to.
func (f *Formatter) commentsBetween(n *sitter.Node, from, to uint) []string {
	var out []string

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if !isCommentKind(c.Kind()) {
			continue
		}

		if c.StartByte() < from || c.EndByte() > to {
			continue
		}

		out = append(out, strings.TrimSpace(f.text(c)))
	}

	return out
}

// isCommentKind reports whether a node kind is one of the comment forms that
// can appear among a call's arguments. Only cf_comment used to be recognised,
// so a cfscript `//` comment was given a trailing comma and swallowed the
// arguments that followed it.
func isCommentKind(kind string) bool {
	switch kind {
	case "comment", "block_comment", "cf_comment":
		return true
	}

	return false
}

// collectionItem is one entry of an array or struct literal. Comments are
// carried alongside the real elements so they survive formatting, but they
// never take a trailing comma.
type collectionItem struct {
	text      string
	isComment bool
}

// collectionItems renders a literal's children, reporting whether any of them
// is a line comment. A line comment runs to end of line, so a literal holding
// one can never be emitted inline — doing so commented out every element after
// it and destroyed the statement.
func (f *Formatter) collectionItems(n *sitter.Node) (items []collectionItem, hasLineComment bool) {
	items = make([]collectionItem, 0, n.NamedChildCount())

	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)

		switch c.Kind() {
		case "comment":
			hasLineComment = true

			items = append(items, collectionItem{text: strings.TrimSpace(f.text(c)), isComment: true})
		case "block_comment", "cf_comment":
			// A CFML comment reaches here from a tag-context struct literal
			// (`<cfset x = { ... }>`). Left in the default branch it counted as
			// an element and was given a trailing comma. Both forms are
			// delimited, so unlike "//" they do not force the literal
			// multi-line.
			items = append(items, collectionItem{text: strings.TrimSpace(f.text(c)), isComment: true})
		default:
			items = append(items, collectionItem{text: f.expr(c)})
		}
	}

	return items, hasLineComment
}

// joinCollectionInline joins items for a single-line literal. Only safe when no
// item is a line comment, which would swallow the rest of the line.
//
// Commas follow the same rule as the multi-line form: they separate elements,
// and a comment is not one. Comma-joining every item turned an interleaved
// comment into a list entry — `[1, 2, <!--- why --->, 3]`. The comma owed to the
// element before the comment still gets written, so the elements either side
// stay separated.
func joinCollectionInline(items []collectionItem) string {
	lastElement := -1

	for i, it := range items {
		if !it.isComment {
			lastElement = i
		}
	}

	var b strings.Builder

	for i, it := range items {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(it.text)

		if !it.isComment && i < lastElement {
			b.WriteString(",")
		}
	}

	return b.String()
}

// joinCollectionLines lays items out one per line, giving a trailing comma to
// every element that still has an element after it, and never to a comment.
func joinCollectionLines(items []collectionItem, indent string) string {
	lastElement := -1

	for i, it := range items {
		if !it.isComment {
			lastElement = i
		}
	}

	var b strings.Builder

	for i, it := range items {
		if i > 0 {
			b.WriteString("\n")
			b.WriteString(indent)
		}

		b.WriteString(it.text)

		if !it.isComment && i < lastElement {
			b.WriteString(",")
		}
	}

	return b.String()
}

func (f *Formatter) exprArray(n *sitter.Node) string {
	if n.NamedChildCount() == 0 {
		return "[]"
	}

	items, hasLineComment := f.collectionItems(n)

	if !hasLineComment {
		inline := "[" + joinCollectionInline(items) + "]"
		if f.lineLen+len(inline) <= f.opts.LineWidth {
			return inline
		}
	}

	indent := f.indented() + f.opts.indent(1)

	return "[\n" + indent + joinCollectionLines(items, indent) + "\n" + f.indented() + "]"
}

func (f *Formatter) exprObject(n *sitter.Node) string {
	if n.NamedChildCount() == 0 {
		return "{}"
	}

	items, hasLineComment := f.collectionItems(n)

	if !hasLineComment {
		inline := "{ " + joinCollectionInline(items) + " }"
		if f.lineLen+len(inline) <= f.opts.LineWidth {
			return inline
		}
	}

	indent := f.indented() + f.opts.indent(1)

	return "{\n" + indent + joinCollectionLines(items, indent) + "\n" + f.indented() + "}"
}

func (f *Formatter) exprString(n *sitter.Node) string {
	// Return the raw text; the grammar preserves the quote style.
	return f.text(n)
}

func (f *Formatter) exprArrow(n *sitter.Node) string {
	params := n.ChildByFieldName("parameters")
	body := n.ChildByFieldName("body")

	// Lucee spells a closure `=>` and a lambda `->`; they differ in what they
	// capture, so the source's own arrow has to survive formatting.
	arrow := arrowToken(n)

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
		return fmt.Sprintf("%s %s %s", paramStr, arrow, f.text(body))
	}

	return fmt.Sprintf("%s %s %s", paramStr, arrow, f.expr(body))
}

// arrowToken returns the arrow an arrow_function was written with, defaulting
// to `=>` if the grammar ever produces one without an anonymous arrow child.
func arrowToken(n *sitter.Node) string {
	for i := uint(0); i < n.ChildCount(); i++ {
		if kind := n.Child(i).Kind(); kind == "=>" || kind == "->" {
			return kind
		}
	}

	return "=>"
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
			case "array_return_suffix":
				current = appendTypeSuffix(current, f.text(c))
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

	leading := f.opts.CommaPosition == "before"
	for i, p := range parts {
		if leading {
			if i > 0 {
				sb.WriteString(indent)
				sb.WriteString(", ")
				sb.WriteString(p)
			} else {
				sb.WriteString(indent)
				sb.WriteString(p)
			}
		} else {
			sb.WriteString(indent)
			sb.WriteString(p)

			if i < len(parts)-1 {
				sb.WriteString(",")
			}
		}

		sb.WriteString("\n")
	}

	sb.WriteString(f.opts.indent(f.level))
	sb.WriteString(")")

	return sb.String()
}

// hasFlatParams returns true if formal_parameters uses the flat structure:
// required/type/name/default as direct siblings of the params node rather
// than wrapped in a single per-parameter node. required_parameter and
// optional_parameter, which the non-flat path below was written for, do not
// exist anywhere in the current grammar (cfscript/grammar.js's
// _formal_parameter never wraps a parameter in one) — every parameter is flat.
//
// A typed parameter always has a parameter_type sibling, so that alone used
// to gate this. An untyped parameter does not, but "required" on one is
// exactly as flat: `required timeUnit = "milliseconds"` parses as the
// anonymous token "required" followed by an assignment_pattern sibling, not a
// single wrapping node — so without this check, a formal_parameters made up
// entirely of untyped parameters took the non-flat path, which iterates named
// children only and silently dropped every "required" it found.
func (f *Formatter) hasFlatParams(params *sitter.Node) bool {
	for i := uint(0); i < params.ChildCount(); i++ {
		switch params.Child(i).Kind() {
		case "parameter_type", "required":
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
		case "array_return_suffix":
			current = appendTypeSuffix(current, f.text(c))
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

// appendTypeSuffix glues an array_return_suffix (`[]`, always exactly that —
// the grammar lexes it as one token) onto the type token it qualifies, so
// `string[] v` does not come back as `string [] v`. The parts are joined with
// spaces, so it cannot simply be appended as another element.
func appendTypeSuffix(parts []string, suffix string) []string {
	if len(parts) == 0 {
		return append(parts, suffix)
	}

	parts[len(parts)-1] += suffix

	return parts
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

// componentKeywords are the declaration keywords that may precede a component
// body. The grammar emits them as anonymous nodes, so they have to be
// recognised explicitly rather than picked up as named children.
var componentKeywords = map[string]bool{
	"component": true,
	"interface": true,
	"abstract":  true,
	"final":     true,
}

// scriptComponent renders a CFC component declaration:
//
//	component [extends="X"] [implements="Y"] { ... }
func (f *Formatter) scriptComponent(n *sitter.Node) {
	// Collect modifiers / attributes that come before the body
	var attrs []string

	var body *sitter.Node

	// Declaration keywords are anonymous nodes preceding the body. The header
	// used to be hardcoded to "component", which rewrote `interface {}` as
	// `component {}` and dropped `abstract`/`final` modifiers.
	var keywords []string

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "statement_block", "block", "class_body", "component_body":
			body = c
		case "identifier":
			// 'component' keyword itself — skip
		default:
			switch {
			case c.IsNamed():
				attrs = append(attrs, f.text(c))
			case componentKeywords[strings.ToLower(c.Kind())]:
				keywords = append(keywords, f.text(c))
			}
		}
	}

	header := strings.Join(keywords, " ")
	if header == "" {
		header = "component"
	}

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

	// Attributes written after the parameter list — `function f() localmode="true" {}`
	// — are siblings of the parameters, not of the modifiers. Hoisting one into
	// the prefix emits `localmode="true" function f() {}`, which does not compile,
	// so anything starting past the parameter list is kept as a suffix.
	var attrs []string

	attrsFrom := uint(0)
	haveParams := params != nil

	if haveParams {
		attrsFrom = params.EndByte()
	}

	sawFuncKeyword := false

	// Walk children to pick up access modifiers that have no field name.
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		fieldName := n.FieldNameForChild(uint32(i))
		switch fieldName {
		case "name", "parameters", "body", "return_type":
			continue
		}

		// Drop the `function` keyword itself, which the signature re-emits.
		// Only the first one: `function function f()` declares a function
		// returning a function, and the second token is the keyword.
		if !c.IsNamed() && c.Kind() == "function" && !sawFuncKeyword {
			sawFuncKeyword = true

			continue
		}

		if haveParams && c.StartByte() >= attrsFrom {
			attrs = append(attrs, f.text(c))

			continue
		}

		// `User[] function getUsers()` — the suffix is its own node
		// sitting beside the return type, not part of it.
		if c.Kind() == "array_return_suffix" {
			prefix = appendTypeSuffix(prefix, f.text(c))

			continue
		}

		// Anonymous children reach here too. The grammar tokenises some type
		// and modifier keywords (`query`, `abstract`, `final`) as anonymous
		// nodes rather than named identifiers; gating on IsNamed dropped them
		// from the signature entirely.
		prefix = append(prefix, f.text(c))
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

	if len(attrs) > 0 {
		sig.WriteString(" ")
		sig.WriteString(strings.Join(attrs, " "))
	}

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

// declKeyword returns the leading keyword(s) of a variable_declaration node.
// The grammar's variable_declaration accepts "var", "final", or the combined
// "final var" / "var final" (cfscript/grammar.js), each spelled as its own
// anonymous child rather than one token — a loop that stops at the first
// "var" it sees, or that only recognises "var", silently drops "final" and
// renders it as an ordinary local, discarding the immutability it declares.
// Every keyword actually present is kept, in the order it was written;
// "var" is the fallback only when the loop finds none, which should not
// happen for a real variable_declaration node.
func declKeyword(n *sitter.Node) string {
	var kws []string

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.IsNamed() {
			continue
		}

		switch c.Kind() {
		case "var", "local", "final":
			kws = append(kws, c.Kind())
		}
	}

	if len(kws) == 0 {
		return "var"
	}

	return strings.Join(kws, " ")
}

// scriptVarDecl renders: var/local/final name [= expr][, name [= expr]];
func (f *Formatter) scriptVarDecl(n *sitter.Node) {
	keyword := declKeyword(n)

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

	// throw(type = "x", message = "y"): the grammar gives this the same
	// `arguments` node a call expression gets, so render it through the same
	// helper rather than re-implementing argument layout here. Falling through
	// to the generic path below emitted the argument list verbatim, which left
	// throw as the only call in the language whose named arguments were not
	// spaced like every other call's — `throw (type="x")` beside
	// `writeLog(text = "x")`. exprArgs also handles splitting long lists, which
	// the bespoke code below did separately.
	if val != nil && val.Kind() == "arguments" {
		f.iLine("throw" + f.exprArgs(val) + ";")
		f.scriptWrite("\n")

		return
	}

	// Older grammars modelled the same call as a parenthesized sequence.
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
	f.deferBlockComments(n, cond, cons)
	f.scriptBlockOf2(cons)

	if alt != nil {
		lead := f.elseLead(n, cons, alt)

		switch alt.Kind() {
		case "else_clause":
			// else if or else
			inner := f.elseBody(alt)
			if inner != nil && inner.Kind() == "if_statement" {
				f.scriptWrite(lead + " ")
				// Re-use scriptIf but write inline (no leading newline/indent).
				f.scriptIfInline(inner)
			} else {
				f.scriptWrite(lead)
				f.scriptBlockOf2(inner)
			}
		case "if_statement":
			f.scriptWrite(lead + " ")
			f.scriptIfInline(alt)
		default:
			f.scriptWrite(lead)
			f.scriptBlockOf2(alt)
		}
	}

	f.scriptWrite("\n")
}

// elseLead emits any comments sitting between the consequence and the
// alternative, and returns the text to introduce the `else` with: attached to
// the closing brace normally, or starting a fresh line once a comment has
// been written between them.
func (f *Formatter) elseLead(n, cons, alt *sitter.Node) string {
	if cons == nil || alt == nil {
		return " else"
	}

	if !f.writeInterveningComments(n, cons.EndByte(), alt.StartByte()) {
		return " else"
	}

	f.scriptWrite("\n")
	f.writeIndent()

	return "else"
}

// scriptIfInline is like scriptIf but does not prefix a newline+indent
// (used for `else if` continuation on the same line).
func (f *Formatter) scriptIfInline(n *sitter.Node) {
	cond := n.ChildByFieldName("condition")
	cons := n.ChildByFieldName("consequence")
	alt := n.ChildByFieldName("alternative")

	f.scriptWrite(fmt.Sprintf("if %s", f.parenExpr(cond)))
	f.deferBlockComments(n, cond, cons)
	f.scriptBlockOf2(cons)

	if alt != nil {
		lead := f.elseLead(n, cons, alt)

		switch alt.Kind() {
		case "else_clause":
			inner := f.elseBody(alt)
			if inner != nil && inner.Kind() == "if_statement" {
				f.scriptWrite(lead + " ")
				f.scriptIfInline(inner)
			} else {
				f.scriptWrite(lead)
				f.scriptBlockOf2(inner)
			}
		default:
			f.scriptWrite(lead)
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

		return
	}

	// A single-statement body gains braces here. The padding has to match
	// scriptBlock's exactly: on a second format the braces are in the source,
	// so scriptBlock renders the same code and any difference in blank lines
	// makes formatting non-idempotent — an unchanged file kept producing a
	// new diff on every save.
	f.scriptWrite(" {")
	f.scriptWrite("\n\n")

	f.level++

	if f.flushBlockComments() && isScriptBlockStmt(body) {
		f.scriptWrite("\n")
	}

	f.formatScriptNode(body)
	f.scriptWrite("\n")

	f.level--
	f.writeIndent()
	f.scriptWrite("}")
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
					if val2 != nil && child.StartByte() == val2.StartByte() && child.EndByte() == val2.EndByte() {
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
		keyword := declKeyword(n)

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
	finalizer := n.ChildByFieldName("finalizer")

	f.iLine("try")

	if body != nil {
		f.scriptBlock(body)
	}

	// Every catch clause carries the same `handler` field name, so
	// ChildByFieldName returns only the first one — a try with several
	// catches silently lost all but the first, bodies included. Walk the
	// children instead.
	prevEnd := uint(0)
	if body != nil {
		prevEnd = body.EndByte()
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() != "catch_clause" {
			continue
		}

		f.scriptCatch(c, f.clauseLead(n, prevEnd, c, "catch"))
		prevEnd = c.EndByte()
	}

	if finalizer != nil {
		fBody := finalizer.ChildByFieldName("body")
		if fBody == nil {
			fBody = finalizer
		}

		f.scriptWrite(f.clauseLead(n, prevEnd, finalizer, "finally"))
		f.scriptBlock(fBody)
	}

	f.scriptWrite("\n")
}

// clauseLead emits any comments between the previous clause and this one, and
// returns the keyword text to introduce it with — attached to the closing
// brace normally, or on a fresh line once a comment has been written.
func (f *Formatter) clauseLead(parent *sitter.Node, from uint, clause *sitter.Node, keyword string) string {
	if clause == nil || from == 0 {
		return " " + keyword
	}

	if !f.writeInterveningComments(parent, from, clause.StartByte()) {
		return " " + keyword
	}

	f.scriptWrite("\n")
	f.writeIndent()

	return keyword
}

// scriptCatch renders one catch clause: `catch (<type> <param>) { ... }`.
// The exception type is a separate `type` field, not part of the parameter —
// rendering only the parameter turned `catch (java.lang.Exception e)` into
// `catch (e)`, widening what the handler catches.
func (f *Formatter) scriptCatch(n *sitter.Node, lead string) {
	catchType := n.ChildByFieldName("type")
	param := n.ChildByFieldName("parameter")
	body := n.ChildByFieldName("body")

	var parts []string

	if catchType != nil {
		parts = append(parts, f.text(catchType))
	}

	if param != nil {
		parts = append(parts, f.exprParam(param))
	}

	if len(parts) > 0 {
		f.scriptWrite(fmt.Sprintf("%s (%s)", lead, strings.Join(parts, " ")))
	} else {
		f.scriptWrite(lead)
	}

	if body != nil {
		f.scriptBlock(body)
	}
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
