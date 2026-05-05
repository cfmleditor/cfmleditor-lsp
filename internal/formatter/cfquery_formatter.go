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
	case "query_identifier":
		text := f.text(n)
		upper := strings.ToUpper(text)
		if sqlClauseKeywords[upper] {
			if !*first {
				f.write("\n")
			}
			f.writeIndent()
			f.write(text)
			*first = false
		} else {
			if *first {
				f.writeIndent()
				*first = false
			} else {
				f.write(" ")
			}
			f.write(text)
		}

	case "query_comma":
		f.write(",")

	case "cf_selfclose_tag":
		// Embedded CF tag (e.g. <cfqueryparam ...>)
		f.write(" ")
		f.write(f.text(n))

	case "query_assignment_expression":
		// e.g. active = 1 or id = <cfqueryparam ...>
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		f.write(" = ")
		// Right side: emit inline without leading space
		f.formatQueryNodeInline(right)

	case "query_alias":
		// e.g. u.name or table alias
		left := n.ChildByFieldName("left")
		right := n.ChildByFieldName("right")
		f.formatQueryNode(left, first)
		f.write(".")
		f.formatQueryNodeInline(right)

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

// formatQueryNodeInline emits a query node without leading space/newline logic.
func (f *Formatter) formatQueryNodeInline(n *sitter.Node) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "query_identifier":
		f.write(f.text(n))
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
		f.write(f.text(n))
	default:
		f.write(strings.TrimSpace(f.text(n)))
	}
}
