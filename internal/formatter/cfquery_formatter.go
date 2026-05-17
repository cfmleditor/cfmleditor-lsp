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
	"INNER": true, "LEFT": true, "RIGHT": true, "OUTER": true, "CROSS": true,
	"JOIN": true, "ON": true,
	"ORDER": true, "GROUP": true, "HAVING": true,
	"INSERT": true, "UPDATE": true, "DELETE": true,
	"SET": true, "VALUES": true, "INTO": true,
	"UNION": true, "EXCEPT": true, "INTERSECT": true,
	"AND": true, "OR": true,
	"LIMIT": true, "OFFSET": true,
}

// sqlKeywords are SQL keywords that should be uppercased but do not start a new line.
var sqlKeywords = map[string]bool{
	"AS": true,
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
			if f.opts.ParseQuery != nil {
				tree := f.opts.ParseQuery(querySrc)
				if tree != nil {
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
		if sqlClauseKeywords[upper] {
			if !*first {
				f.write("\n")
			}
			f.writeIndent()
			f.write(upper)
			*first = false
		} else {
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			f.write(upper)
		}

	case "query_identifier":
		text := f.text(n)
		upper := strings.ToUpper(text)
		switch {
		case sqlClauseKeywords[upper]:
			if !*first {
				f.write("\n")
			}
			f.writeIndent()
			f.write(upper)
			*first = false
		case sqlKeywords[upper]:
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			f.write(upper)
		default:
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			f.write(strings.ToLower(text))
		}

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
				nameText = strings.ToUpper(f.text(nameNode))
				isClauseKW = sqlClauseKeywords[nameText]
			case "query_function_name":
				nameText = f.text(nameNode)
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
		f.formatQueryNodeInline(right)

	case "query_comma":
		if *first {
			f.writeIndent()
		}
		nextIsTag := n.NextSibling() != nil && n.NextSibling().Kind() == "cf_selfclose_tag"
		if f.lineLen > f.opts.QueryLineWidth || nextIsTag {
			f.write("\n")
			f.writeIndent()
			f.write(",")
			*first = false
		} else {
			f.write(",")
			*first = false
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
			f.write("\n")
			*first = true
		}

	case "query_assignment_expression":
		// e.g. active = 1 or id = <cfqueryparam ...>
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		f.write(" = ")
		// Right side: emit inline without leading space
		f.formatQueryNodeInline(right)
		// If right side is a self-closing tag, push next content to new line
		if right != nil && right.Kind() == "cf_selfclose_tag" {
			f.write("\n")
			*first = true
		}

	case "query_alias":
		// e.g. u.name or table alias
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		f.write(".")
		f.formatQueryNodeInline(right)

	case "cf_if_tag":
		f.formatQueryCFIf(n)
		*first = true

	case "cf_if_alt":
		f.formatQueryCFIfAlt(n)
		*first = true

	case "cf_tag":
		f.formatQueryCFTag(n)
		*first = true

	case "parenthesized_query_node":
		f.formatQueryParenthesized(n, first)

	default:
		// Other nodes: emit text inline
		text := strings.TrimSpace(f.text(n))
		if text != "" {
			if *first {
				f.writeIndent()
				*first = false
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
	if !f.lastNL {
		f.write("\n")
	}
	f.level--
	f.writeIndent()
	f.write("</cfif>\n")
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
						f.write("\n")
						f.writeIndent()
						f.write(",")
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
		f.write(strings.ToUpper(f.text(n)))
	case "query_identifier":
		text := f.text(n)
		upper := strings.ToUpper(text)
		if sqlClauseKeywords[upper] || sqlKeywords[upper] {
			f.write(upper)
		} else {
			f.write(strings.ToLower(text))
		}
	case "query_function":
		nameNode := n.ChildByFieldName("name")
		argsNode := n.ChildByFieldName("arguments")
		if nameNode != nil {
			f.write(strings.ToUpper(f.text(nameNode)))
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
