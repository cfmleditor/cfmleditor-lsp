package formatter

// element_formatter.go — pretty-printer for HTML elements.

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// formatElement re-indents HTML elements, stripping original indentation.
func (f *Formatter) formatElement(n *sitter.Node) {
	// If the element has structured children (start_tag, content, end_tag),
	// recurse into them for proper indentation of nested CF tags.
	var startTag, endTag *sitter.Node
	var bodyNodes []*sitter.Node
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "start_tag", "self_closing_tag":
			startTag = c
		case "end_tag":
			endTag = c
		default:
			bodyNodes = append(bodyNodes, c)
		}
	}

	// Self-closing HTML tag or element with no recognizable structure: emit verbatim
	if startTag == nil || (endTag == nil && len(bodyNodes) == 0) {
		f.nl()
		f.writeIndent()
		f.write(strings.TrimSpace(f.text(n)))
		f.write("\n")
		return
	}

	// Emit start tag
	f.nl()
	f.writeIndent()
	f.write(strings.TrimSpace(f.text(startTag)))
	f.write("\n")

	// Indent and format body children
	f.level++
	for _, c := range bodyNodes {
		f.formatNode(c)
	}
	f.level--

	// Emit end tag
	if endTag != nil {
		f.nl()
		f.writeIndent()
		f.write(strings.TrimSpace(f.text(endTag)))
		f.write("\n")
	}
}
