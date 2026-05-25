package formatter

// element_formatter.go — pretty-printer for HTML elements.

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// htmlVoidElements are HTML elements that never have children.
var htmlVoidElements = map[string]bool{
	"br": true, "hr": true, "img": true, "input": true,
	"meta": true, "link": true, "area": true, "base": true,
	"col": true, "embed": true, "source": true, "track": true,
	"wbr": true,
}

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

	// Self-closing HTML tag, void element, or element with no recognizable structure: emit verbatim
	if startTag == nil || (endTag == nil && len(bodyNodes) == 0) || f.isVoidElement(startTag) {
		f.nl()
		f.writeWrapped(collapseWhitespace(strings.TrimSpace(f.text(n))))

		return
	}

	// Emit start tag
	f.nl()
	f.writeIndent()
	f.write(strings.TrimSpace(f.text(startTag)))
	f.write("\n")

	// Indent and format body children
	f.level++
	if f.allSingleLine(bodyNodes) && f.fitsOnLine(bodyNodes) {
		f.formatInlineRun(bodyNodes)
	} else {
		f.formatBodyRuns(bodyNodes)
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

// isVoidElement checks if a start_tag is for an HTML void element.
func (f *Formatter) isVoidElement(startTag *sitter.Node) bool {
	if startTag == nil {
		return false
	}

	for i := uint(0); i < startTag.ChildCount(); i++ {
		c := startTag.Child(i)
		if c.Kind() == "tag_name" {
			return htmlVoidElements[strings.ToLower(f.text(c))]
		}
	}

	return false
}

// allSingleLine returns true if the content nodes, after trimming surrounding
// whitespace, all fit on a single line (no embedded newlines in content).
func (f *Formatter) allSingleLine(nodes []*sitter.Node) bool {
	if len(nodes) == 0 {
		return false
	}
	// Concatenate the text of all nodes and check if the trimmed content is single-line.
	var combined string
	for _, n := range nodes {
		combined += f.text(n)
	}

	trimmed := strings.TrimSpace(combined)
	if trimmed == "" {
		return false
	}

	return !strings.Contains(trimmed, "\n")
}

// fitsOnLine returns true if the collapsed content of nodes fits within the line width.
func (f *Formatter) fitsOnLine(nodes []*sitter.Node) bool {
	var combined string
	for _, n := range nodes {
		combined += f.text(n)
	}

	collapsed := collapseWhitespace(strings.TrimSpace(combined))
	indent := len(f.opts.indent(f.level))

	return indent+len(collapsed) <= f.opts.LineWidth
}

// hasBlockChild returns true if any node in the list is a block-level tag.
func (f *Formatter) hasBlockChild(nodes []*sitter.Node) bool {
	for _, c := range nodes {
		if f.isBlockTagKind(c) {
			return true
		}
	}

	return false
}

// formatInlineRun emits a sequence of inline nodes on a single line,
// preserving the original spacing between them.
func (f *Formatter) formatInlineRun(nodes []*sitter.Node) {
	var combined string
	for _, n := range nodes {
		combined += f.text(n)
	}

	trimmed := strings.TrimSpace(combined)
	if trimmed == "" {
		return
	}

	f.writeWrapped(collapseWhitespace(trimmed))
}

// formatBodyRuns processes body nodes by grouping consecutive inline nodes
// (html_text, hash_expression) that should stay together, emitting each
// group on one line while delegating block-level nodes to formatNode.
func (f *Formatter) formatBodyRuns(nodes []*sitter.Node) {
	for i := 0; i < len(nodes); {
		c := nodes[i]
		if f.isInlineNode(c) {
			// Collect consecutive inline nodes into a run.
			run := []*sitter.Node{c}
			j := i + 1

			for j < len(nodes) && f.isInlineNode(nodes[j]) {
				run = append(run, nodes[j])
				j++
			}

			f.formatTextRun(run)

			i = j
		} else {
			f.formatNode(c)

			i++
		}
	}
}

// isInlineNode returns true for nodes that represent inline content
// (text, hash expressions) rather than block-level elements.
func (f *Formatter) isInlineNode(n *sitter.Node) bool {
	switch n.Kind() {
	case "html_text", "text", "hash_expression", "hash_single", "implicit_end_tag":
		return true
	}

	return false
}

// formatTextRun emits a run of inline nodes on a single indented line,
// collapsing all internal whitespace to single spaces (HTML whitespace rules).
func (f *Formatter) formatTextRun(nodes []*sitter.Node) {
	var combined string
	for _, n := range nodes {
		combined += f.text(n)
	}

	trimmed := strings.TrimSpace(combined)
	if trimmed == "" {
		return
	}

	f.writeWrapped(collapseWhitespace(trimmed))
}

// collapseWhitespace replaces runs of whitespace with a single space.
func collapseWhitespace(s string) string {
	var b strings.Builder

	b.Grow(len(s))

	inWS := false

	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !inWS {
				b.WriteByte(' ')

				inWS = true
			}
		} else {
			b.WriteRune(r)

			inWS = false
		}
	}

	return b.String()
}

// writeWrapped writes text with word-wrapping at the configured line width.
// Each line is indented at the current level.
func (f *Formatter) writeWrapped(text string) {
	indent := f.opts.indent(f.level)

	maxContent := f.opts.LineWidth - len(indent)
	if maxContent <= 0 || len(text) <= maxContent {
		f.writeIndent()
		f.write(text)
		f.write("\n")

		return
	}

	for len(text) > 0 {
		if len(text) <= maxContent {
			f.writeIndent()
			f.write(text)
			f.write("\n")

			break
		}
		// Find last space at or before maxContent.
		cut := strings.LastIndexByte(text[:maxContent], ' ')
		if cut <= 0 {
			// No space found; find next space after maxContent.
			cut = strings.IndexByte(text[maxContent:], ' ')
			if cut < 0 {
				f.writeIndent()
				f.write(text)
				f.write("\n")

				break
			}

			cut += maxContent
		}

		f.writeIndent()
		f.write(text[:cut])
		f.write("\n")

		text = text[cut+1:]
	}
}
