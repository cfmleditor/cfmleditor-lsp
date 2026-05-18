package formatter

// cfquery_formatter.go — pretty-printer for the cfquery sub-grammar.

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// sqlClauseKeywords are SQL keywords that start a new clause and should begin
// on their own line.
var sqlClauseKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true,
	"JOIN": true,
	"ORDER": true, "GROUP": true, "HAVING": true,
	"INSERT": true, "UPDATE": true, "DELETE": true,
	"SET": true, "VALUES": true, "INTO": true,
	"UNION": true, "EXCEPT": true, "INTERSECT": true,
	"AND": true, "OR": true,
	"LIMIT": true, "OFFSET": true,
}

// sqlJoinModifiers are keywords that precede JOIN and should be kept on the same line.
var sqlJoinModifiers = map[string]bool{
	"INNER": true, "LEFT": true, "RIGHT": true, "OUTER": true, "CROSS": true,
}

// sqlKeywords are SQL keywords that should be uppercased but do not start a new line.
var sqlKeywords = map[string]bool{
	"AS": true, "ON": true,
}

// formatCFQuery pretty-prints a <cfquery>…</cfquery> block by re-parsing
// the content with the CFQuery sub-grammar.
func (f *Formatter) formatCFQuery(n *sitter.Node) {
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<cfquery" + f.renderAttrs("cfquery", attrs) + ">\n")
	f.level++

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() == "cf_query_content" {
			querySrc := f.src[c.StartByte():c.EndByte()]
			if f.opts.QueryFormat && f.opts.ParseQuery != nil {
				tree := f.opts.ParseQuery(querySrc)
				if tree != nil {
					if tree.RootNode().HasError() {
						f.recordParseError("cfquery", tree.RootNode(), querySrc, c.StartPosition().Row)
					}
					origSrc := f.src
					f.src = querySrc
					f.formatQueryChildren(tree.RootNode())
					f.src = origSrc
					tree.Close()
				}
			} else {
				// Emit verbatim, trimmed
				trimmed := strings.TrimSpace(string(querySrc))
				if trimmed != "" {
					f.writeIndent()
					f.write(trimmed + "\n")
				}
			}
		}
	}

	f.level--
	f.writeIndent()
	f.write("</cfquery>\n")
}

// formatQueryChildren walks the CFQuery parse tree and emits formatted SQL.
func (f *Formatter) formatQueryChildren(root *sitter.Node) {
	first := true
	for i := uint(0); i < root.ChildCount(); i++ {
		c := root.Child(i)
		f.formatQueryNode(c, &first)
	}
	if f.pendingComma {
		f.write(",")
		f.pendingComma = false
	}
	// Ensure trailing newline
	if !f.lastNL {
		f.write("\n")
	}
}

// formatQueryNode emits a single query grammar node, inserting newlines
// before SQL clause keywords.
func (f *Formatter) formatQueryNode(n *sitter.Node, first *bool) {
	kind := n.Kind()

	switch kind {
	case "query_keyword":
		text := f.text(n)
		upper := strings.ToUpper(text)
		out := upper
		if !f.opts.QueryUppercaseKeywords {
			out = text
		}
		switch {
		case sqlClauseKeywords[upper]:
			// JOIN stays on the same line if preceded by a join modifier
			prevIsJoinMod := false
			if upper == "JOIN" {
				if prev := n.PrevSibling(); prev != nil {
					prevText := strings.ToUpper(strings.TrimSpace(f.text(prev)))
					prevIsJoinMod = sqlJoinModifiers[prevText]
				}
			}
			if prevIsJoinMod {
				f.write(" " + out)
			} else {
				if !*first {
					if f.pendingComma {
						f.pendingComma = false
						f.appendTrailingComma()
					}
					f.write("\n")
				}
				f.writeIndent()
				f.write(out)
			}
			*first = false
		case sqlJoinModifiers[upper]:
			// Stay on same line if preceded by another join modifier (e.g. LEFT OUTER)
			prevIsJoinMod := false
			if prev := n.PrevSibling(); prev != nil {
				prevText := strings.ToUpper(strings.TrimSpace(f.text(prev)))
				prevIsJoinMod = sqlJoinModifiers[prevText]
			}
			if prevIsJoinMod {
				f.write(" " + out)
			} else {
				if !*first {
					f.write("\n")
				}
				f.writeIndent()
				f.write(out)
			}
			*first = false
		case upper == "ON":
			// ON indents one level deeper than the JOIN
			if !*first {
				f.write("\n")
			}
			f.level++
			f.writeIndent()
			f.write(out)
			f.level--
			*first = false
		default:
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			f.write(out)
		}

	case "query_identifier":
		text := f.text(n)
		upper := strings.ToUpper(text)
		switch {
		case sqlClauseKeywords[upper]:
			prevIsJoinMod := false
			if upper == "JOIN" {
				if prev := n.PrevSibling(); prev != nil {
					prevText := strings.ToUpper(strings.TrimSpace(f.text(prev)))
					prevIsJoinMod = sqlJoinModifiers[prevText]
				}
			}
			if prevIsJoinMod {
				if f.opts.QueryUppercaseKeywords {
					f.write(" " + upper)
				} else {
					f.write(" " + text)
				}
			} else {
				if !*first {
					if f.pendingComma {
						f.pendingComma = false
						f.appendTrailingComma()
					}
					f.write("\n")
				}
				f.writeIndent()
				if f.opts.QueryUppercaseKeywords {
					f.write(upper)
				} else {
					f.write(text)
				}
			}
			*first = false
		case sqlJoinModifiers[upper]:
			prevIsJoinMod := false
			if prev := n.PrevSibling(); prev != nil {
				prevText := strings.ToUpper(strings.TrimSpace(f.text(prev)))
				prevIsJoinMod = sqlJoinModifiers[prevText]
			}
			if prevIsJoinMod {
				if f.opts.QueryUppercaseKeywords {
					f.write(" " + upper)
				} else {
					f.write(" " + text)
				}
			} else {
				if !*first {
					f.write("\n")
				}
				f.writeIndent()
				if f.opts.QueryUppercaseKeywords {
					f.write(upper)
				} else {
					f.write(text)
				}
			}
			*first = false
		case upper == "ON":
			if !*first {
				f.write("\n")
			}
			f.level++
			f.writeIndent()
			if f.opts.QueryUppercaseKeywords {
				f.write(upper)
			} else {
				f.write(text)
			}
			f.level--
			*first = false
		case sqlKeywords[upper]:
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			if f.opts.QueryUppercaseKeywords {
				f.write(upper)
			} else {
				f.write(text)
			}
		default:
			if *first {
				f.writeIndent()
				*first = false
			} else if prev := n.PrevSibling(); prev != nil && prev.EndByte() == n.StartByte() {
				// No whitespace between nodes — keep adjacent
			} else {
				f.write(" ")
			}
			f.write(strings.ToLower(text))
		}

	case "query_function_name":
		if *first {
			f.writeIndent()
			*first = false
		} else {
			f.write(" ")
		}
		f.write(strings.ToLower(f.text(n)))

	case "query_function":
		// Handles both SQL functions (COUNT, UPPER) and table(columns), VALUES(...)
		nameNode := n.ChildByFieldName("name")
		argsNode := n.ChildByFieldName("arguments")

		// Determine name text and whether it's a keyword
		var nameText string
		isClauseKW := false
		if nameNode != nil {
			switch nameNode.Kind() {
			case "query_keyword":
				if f.opts.QueryUppercaseKeywords {
					nameText = strings.ToUpper(f.text(nameNode))
				} else {
					nameText = f.text(nameNode)
				}
				isClauseKW = sqlClauseKeywords[strings.ToUpper(f.text(nameNode))]
			case "query_function_name":
				nameText = strings.ToLower(f.text(nameNode))
			default:
				// Adjacent name( = function call (preserve case), name ( = table (lowercase)
				if argsNode != nil && nameNode.EndByte() == argsNode.StartByte() {
					nameText = f.text(nameNode)
				} else {
					nameText = strings.ToLower(f.text(nameNode))
				}
			}
		}

		// Clause keywords (VALUES, SET) start a new line
		if isClauseKW {
			if !*first {
				f.write("\n")
			}
			f.writeIndent()
			f.write(nameText)
			*first = false
		} else {
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			f.write(nameText)
		}

		// Emit arguments - use parenthesized handler for proper wrapping
		if argsNode != nil {
			// After INTO/UPDATE, name(cols) is a table — force space before (
			if !isClauseKW && nameNode != nil &&
				nameNode.Kind() != "query_keyword" && nameNode.Kind() != "query_function_name" &&
				nameNode.EndByte() == argsNode.StartByte() {
				if prev := n.PrevSibling(); prev != nil && prev.Kind() == "query_keyword" {
					prevText := strings.ToUpper(f.text(prev))
					if prevText == "INTO" || prevText == "UPDATE" {
						f.write(" ")
					}
				}
			}
			f.formatQueryParenthesized(argsNode, first)
		}

	case "query_math_expression":
		left := n.ChildByFieldName("left")
		op := n.ChildByFieldName("operator")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		if op != nil {
			f.write(" " + f.text(op) + " ")
		}
		f.emitQueryExtrasAndRight(n, right, first)

	case "query_comparison_expression":
		left := n.ChildByFieldName("left")
		op := n.ChildByFieldName("operator")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		if op != nil {
			f.write(" " + f.text(op) + " ")
		}
		f.emitQueryExtrasAndRight(n, right, first)

	case "query_comma":
		switch {
		case f.opts.queryCommaPreserve():
			// Preserve: keep comma in its original position (leading or trailing)
			if *first {
				f.writeIndent()
				f.write(",")
				*first = false
			} else {
				f.write(",")
				nextIsTag := n.NextSibling() != nil && n.NextSibling().Kind() == "cf_selfclose_tag"
				if f.lineLen > f.opts.QueryLineWidth || nextIsTag {
					f.write("\n")
					*first = true
				}
			}
		case f.opts.queryCommaLeading():
			if *first {
				f.writeIndent()
				f.write(",")
				*first = false
			} else {
				nextIsTag := n.NextSibling() != nil && n.NextSibling().Kind() == "cf_selfclose_tag"
				if f.lineLen > f.opts.QueryLineWidth || nextIsTag {
					f.write("\n")
					f.writeIndent()
					f.write(",")
				} else {
					f.write(",")
				}
				*first = false
			}
		default:
			// Trailing comma mode ("after")
			if *first {
				f.pendingComma = true
			} else {
				nextIsTag := n.NextSibling() != nil && n.NextSibling().Kind() == "cf_selfclose_tag"
				if f.lineLen > f.opts.QueryLineWidth || nextIsTag {
					f.write(",\n")
					*first = true
				} else {
					f.write(",")
					*first = false
				}
			}
		}

	case "cf_selfclose_tag":
		// Embedded CF tag (e.g. <cfqueryparam ...>)
		if *first {
			f.writeIndent()
			*first = false
		} else {
			f.write(" ")
		}
		f.writeQuerySelfCloseTag(n)
		// Stay inline if followed by a non-clause keyword like AS
		if next := n.NextSibling(); next != nil &&
			(next.Kind() == "query_keyword" && !sqlClauseKeywords[strings.ToUpper(f.text(next))]) ||
			(next != nil && next.Kind() == "query_identifier" && sqlKeywords[strings.ToUpper(f.text(next))]) {
			*first = false
		} else {
			if f.pendingComma {
				f.write(",")
				f.pendingComma = false
			}
			f.write("\n")
			*first = true
		}

	case "query_assignment_expression":
		// e.g. active = 1 or id = <cfqueryparam ...>
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		f.write(" = ")
		f.emitQueryExtrasAndRight(n, right, first)

	case "query_operator":
		if *first {
			f.writeIndent()
			*first = false
		} else {
			f.write(" ")
		}
		f.write(f.text(n))

	case "query_open_paren", "query_close_paren":
		if *first {
			f.writeIndent()
			*first = false
		} else {
			f.write(" ")
		}
		f.write(f.text(n))

	case "query_alias":
		// e.g. u.name or table alias
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		f.write(".")
		f.formatQueryNodeInline(right)

	case "cf_if_tag":
		// Flush pending comma — if previous content is a regular item, trail it
		if f.pendingComma {
			f.pendingComma = false
			if prev := n.PrevSibling(); prev != nil && prev.Kind() != "cf_if_tag" && prev.Kind() != "cf_tag" {
				f.appendTrailingComma()
			}
		}
		// If cfif is on the same source line as its prev sibling (no newline
		// in between), emit inline to preserve constructs like:
		// FROM <cfif cond>table_a<cfelse>table_b</cfif> alias
		if prev := n.PrevSibling(); prev != nil && !f.hasNewlineBetween(prev.EndByte(), n.StartByte()) {
			f.formatQueryCFIfInline(n)
		} else {
			f.formatQueryCFIf(n)
			*first = true
		}

	case "cf_if_alt":
		f.formatQueryCFIfAlt(n)
		*first = true

	case "cf_tag":
		f.formatQueryCFTag(n)
		*first = true

	case "parenthesized_query_node":
		f.formatQueryParenthesized(n, first)

	case "cf_comment":
		if !f.lastNL {
			f.write("\n")
		}
		f.writeIndent()
		f.write(strings.TrimSpace(f.text(n)))
		f.write("\n")
		*first = true

	default:
		// Other nodes: emit text inline
		text := strings.TrimSpace(f.text(n))
		if text != "" {
			if *first {
				f.writeIndent()
				*first = false
			} else if prev := n.PrevSibling(); prev != nil && prev.EndByte() == n.StartByte() {
				// No whitespace between nodes — keep adjacent
			} else {
				f.write(" ")
			}
			f.write(text)
		}
	}
}

// formatQueryCFIf handles <cfif>...</cfif> blocks inside cfquery.
func (f *Formatter) formatQueryCFIf(n *sitter.Node) {
	if !f.lastNL {
		f.write("\n")
	}

	var cond string
	phase := 0 // 0=before condition, 1=in condition, 2=body
	bodyFirst := true
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := c.Kind()

		if kind == "<cf" && phase == 0 {
			phase = 1
			continue
		}
		if kind == ">" && phase == 1 {
			f.writeIndent()
			f.write("<cfif ")
			cond = f.normalizeCond(strings.TrimSpace(cond))
			f.write(cond + ">\n")
			f.level++
			phase = 2
			continue
		}
		if kind == "</cf" || (kind == ">" && phase == 2) {
			continue
		}
		if kind == "cf_if_alt" {
			if f.pendingComma {
				f.write(",")
				f.pendingComma = false
			}
			if !f.lastNL {
				f.write("\n")
			}
			f.formatQueryCFIfAlt(c)
			continue
		}
		switch phase {
		case 1:
			cond += f.text(c)
		case 2:
			f.formatQueryNode(c, &bodyFirst)
		}
	}
	if f.pendingComma {
		f.write(",")
		f.pendingComma = false
	}
	if !f.lastNL {
		f.write("\n")
	}
	f.level--
	f.writeIndent()
	f.write("</cfif>\n")
}

// formatQueryCFIfInline emits a cfif block inline (no newlines) when it is
// on the same source line as surrounding tokens (e.g. table name
// constructed via FROM <cfif>name_a<cfelse>name_b</cfif> alias).
func (f *Formatter) formatQueryCFIfInline(n *sitter.Node) {
	src := strings.TrimSpace(string(f.src[n.StartByte():n.EndByte()]))
	f.write(src)
}

// hasNewlineBetween checks if the source between two byte offsets contains a newline.
func (f *Formatter) hasNewlineBetween(start, end uint) bool {
	if start >= end {
		return false
	}
	for i := start; i < end; i++ {
		if f.src[i] == '\n' {
			return true
		}
	}
	return false
}

// formatQueryCFIfAlt handles <cfelse> and <cfelseif> inside cfquery.
func (f *Formatter) formatQueryCFIfAlt(n *sitter.Node) {
	f.level--

	tagEmitted := false
	bodyFirst := true
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "<cf":
			continue
		case "cf_else_tag":
			f.writeIndent()
			f.write("<cfelse>\n")
			f.level++
			tagEmitted = true
		case "cf_elseif_tag":
			var cond string
			for j := uint(0); j < c.ChildCount(); j++ {
				ch := c.Child(j)
				if ch.Kind() != ">" {
					cond += f.text(ch)
				}
			}
			cond = strings.TrimSpace(cond)
			if strings.HasPrefix(strings.ToLower(cond), "elseif") {
				cond = strings.TrimSpace(cond[6:])
			}
			f.writeIndent()
			f.write("<cfelseif ")
			cond = f.normalizeCond(cond)
			f.write(cond + ">\n")
			f.level++
			tagEmitted = true
		case "cf_if_alt":
			if f.pendingComma {
				f.write(",")
				f.pendingComma = false
			}
			if !f.lastNL {
				f.write("\n")
			}
			f.formatQueryCFIfAlt(c)
		default:
			if tagEmitted {
				f.formatQueryNode(c, &bodyFirst)
			}
		}
	}
	if f.pendingComma {
		f.write(",")
		f.pendingComma = false
	}
	if !f.lastNL {
		f.write("\n")
	}
}

// formatQueryCFTag handles block CF tags like <cfloop>...</cfloop> inside cfquery.
func (f *Formatter) formatQueryCFTag(n *sitter.Node) {
	if !f.lastNL {
		f.write("\n")
	}

	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">\n")
	f.level++

	// Emit body children (skip start/end tags)
	first := true
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_end_tag":
			continue
		default:
			f.formatQueryNode(c, &first)
		}
	}
	if !f.lastNL {
		f.write("\n")
	}

	f.level--
	f.writeIndent()
	f.write("</" + name + ">\n")
}

// formatQueryParenthesized handles parenthesized expressions like VALUES (...).
func (f *Formatter) formatQueryParenthesized(n *sitter.Node, first *bool) {
	// Check if content has embedded CF tags (recursively)
	hasTag := f.queryNodeHasTag(n)

	// No space before paren if immediately adjacent to previous token in source
	adjacentToPrev := false
	if prev := n.PrevSibling(); prev != nil && prev.EndByte() == n.StartByte() {
		// Only actual function names suppress inner spaces
		if prev.Kind() == "query_function_name" {
			adjacentToPrev = true
		}
	}

	if !hasTag {
		// Simple parenthesized node - check if it fits on one line
		if *first {
			f.writeIndent()
			*first = false
		} else if !adjacentToPrev {
			f.write(" ")
		}

		// Calculate inline length
		inlineLen := 1 // "("
		for i := uint(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			switch c.Kind() {
			case "(", ")":
			case "query_comma":
				inlineLen += 2 // ", "
			default:
				if inlineLen > 1 {
					inlineLen++ // space
				}
				inlineLen += len(strings.TrimSpace(f.text(c)))
			}
		}
		inlineLen++ // ")"

		if f.lineLen+inlineLen <= f.opts.QueryLineWidth {
			// Fits inline
			f.write("(")
			if !adjacentToPrev {
				f.write(" ")
			}
			firstInner := true
			for i := uint(0); i < n.ChildCount(); i++ {
				c := n.Child(i)
				switch c.Kind() {
				case "(", ")":
					continue
				case "query_comma":
					f.write(",")
				default:
					if !firstInner {
						f.write(" ")
					}
					firstInner = false
					f.formatQueryNodeInline(c)
				}
			}
			if !adjacentToPrev {
				f.write(" ")
			}
			f.write(")")
		} else {
			// Expand multi-line
			f.write("(\n")
			f.level++
			firstInner := true
			for i := uint(0); i < n.ChildCount(); i++ {
				c := n.Child(i)
				switch c.Kind() {
				case "(", ")":
					continue
				case "query_comma":
					if f.lineLen > f.opts.QueryLineWidth {
						if f.opts.queryCommaLeading() {
							f.write("\n")
							f.writeIndent()
							f.write(",")
						} else {
							f.write(",\n")
							f.writeIndent()
						}
					} else {
						f.write(",")
					}
				default:
					if !firstInner {
						f.write(" ")
					} else {
						f.writeIndent()
					}
					firstInner = false
					f.formatQueryNodeInline(c)
				}
			}
			f.write("\n")
			f.level--
			f.writeIndent()
			f.write(")")
		}
		return
	}

	// Complex parenthesized node with tags - emit with indentation
	if *first {
		f.writeIndent()
		*first = false
	} else if !adjacentToPrev {
		f.write(" ")
	}
	f.write("(\n")
	f.level++
	innerFirst := true
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "(", ")":
			continue
		case "query_comma":
			if !f.lastNL {
				f.write("\n")
			}
			f.writeIndent()
			f.write(",")
			innerFirst = false
		default:
			f.formatQueryNode(c, &innerFirst)
		}
	}
	if !f.lastNL {
		f.write("\n")
	}
	f.level--
	f.writeIndent()
	f.write(")")
}

// formatQueryNodeInline emits a query node without leading space/newline logic.
func (f *Formatter) formatQueryNodeInline(n *sitter.Node) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "query_keyword":
		if f.opts.QueryUppercaseKeywords {
			f.write(strings.ToUpper(f.text(n)))
		} else {
			f.write(f.text(n))
		}
	case "query_identifier":
		text := f.text(n)
		upper := strings.ToUpper(text)
		if sqlClauseKeywords[upper] || sqlKeywords[upper] {
			if f.opts.QueryUppercaseKeywords {
				f.write(upper)
			} else {
				f.write(text)
			}
		} else {
			f.write(strings.ToLower(text))
		}
	case "query_function_name":
		f.write(strings.ToLower(f.text(n)))
	case "query_function":
		nameNode := n.ChildByFieldName("name")
		argsNode := n.ChildByFieldName("arguments")
		if nameNode != nil {
			f.write(strings.ToLower(f.text(nameNode)))
		}
		if argsNode != nil {
			f.write("(")
			firstInner := true
			for i := uint(0); i < argsNode.ChildCount(); i++ {
				c := argsNode.Child(i)
				switch c.Kind() {
				case "(", ")":
					continue
				case "query_comma":
					f.write(",")
				default:
					if !firstInner {
						f.write(" ")
					}
					firstInner = false
					f.formatQueryNodeInline(c)
				}
			}
			f.write(")")
		}
	case "query_math_expression":
		left := n.ChildByFieldName("left")
		op := n.ChildByFieldName("operator")
		right := n.ChildByFieldName("right")
		f.formatQueryNodeInline(left)
		if op != nil {
			f.write(" " + f.text(op) + " ")
		}
		f.formatQueryNodeInline(right)
	case "query_comparison_expression":
		left := n.ChildByFieldName("left")
		op := n.ChildByFieldName("operator")
		right := n.ChildByFieldName("right")
		f.formatQueryNodeInline(left)
		if op != nil {
			f.write(" " + f.text(op) + " ")
		}
		f.formatQueryNodeInline(right)
	case "query_alias":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNodeInline(left)
		f.write(".")
		f.formatQueryNodeInline(right)
	case "query_assignment_expression":
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNodeInline(left)
		f.write(" = ")
		f.formatQueryNodeInline(right)
	case "query_operator":
		f.write(" " + f.text(n))
	case "query_open_paren", "query_close_paren":
		f.write(" " + f.text(n))
	case "cf_selfclose_tag":
		f.writeQuerySelfCloseTag(n)
	case "parenthesized_query_node":
		first := true
		f.formatQueryParenthesized(n, &first)
	default:
		f.write(strings.TrimSpace(f.text(n)))
	}
}

// writeQuerySelfCloseTag renders a cf_selfclose_tag with normalised attributes,
// matching the CFML formatter's self-close style.
func (f *Formatter) writeQuerySelfCloseTag(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)
	f.write("<" + name + f.renderAttrs(name, attrs) + " />")
}

// emitQueryExtrasAndRight emits any cf_comment extras in a node, then the
// right-hand field node. If comments are present, the right node is emitted
// on a new indented line; otherwise it's emitted inline.
func (f *Formatter) emitQueryExtrasAndRight(n *sitter.Node, right *sitter.Node, first *bool) {
	hasComment := false
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() == "cf_comment" {
			if !f.lastNL {
				f.write("\n")
			}
			f.level++
			f.writeIndent()
			f.write(f.normalizeCFComment(strings.TrimSpace(f.text(c))))
			f.write("\n")
			f.level--
			hasComment = true
		}
	}
	if hasComment {
		*first = true
		f.formatQueryNode(right, first)
	} else {
		f.formatQueryNodeInline(right)
	}
	if right != nil && right.Kind() == "cf_selfclose_tag" {
		f.write("\n")
		*first = true
	}
}

// queryNodeHasTag checks recursively if a node contains any CF tags.
func (f *Formatter) queryNodeHasTag(n *sitter.Node) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_selfclose_tag", "cf_if_tag", "cf_tag":
			return true
		}
		if c.ChildCount() > 0 && f.queryNodeHasTag(c) {
			return true
		}
	}
	return false
}
