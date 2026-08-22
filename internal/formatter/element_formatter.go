package formatter

// element_formatter.go — pretty-printer for HTML elements.

import (
	"slices"
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

	// Content written tight against both tags in the source stays on one line.
	if f.isTightElement(startTag, endTag, bodyNodes) {
		f.nl()
		f.writeIndent()
		f.write(strings.TrimSpace(f.text(startTag)))

		for _, c := range bodyNodes {
			f.write(f.text(c))
		}

		if endTag != nil {
			f.write(strings.TrimSpace(f.text(endTag)))
		}

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

	inline := func() { f.formatInlineRun(bodyNodes) }

	if f.allSingleLine(bodyNodes) && f.rendersOnOneLine(inline) {
		inline()
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

// isTightElement reports whether an element's content is written flush against
// both of its tags in the source — no whitespace after the start tag's ">" and
// none before the end tag's "<", as in <div class="x">Sorry</div>. Such an
// element is emitted on a single line with its content verbatim, since breaking
// it introduces whitespace the author deliberately left out (and, for inline
// elements, whitespace the browser renders).
//
// Content spanning more than one line is excluded: it cannot be emitted on a
// single line without joining lines, which is a larger change than preserving
// the author's spacing.
func (f *Formatter) isTightElement(startTag, endTag *sitter.Node, bodyNodes []*sitter.Node) bool {
	if startTag == nil || endTag == nil {
		return false
	}

	// <div></div> — nothing between the tags at all.
	if len(bodyNodes) == 0 {
		return startTag.EndByte() == endTag.StartByte()
	}

	first, last := bodyNodes[0], bodyNodes[len(bodyNodes)-1]
	if startTag.EndByte() != first.StartByte() || last.EndByte() != endTag.StartByte() {
		return false
	}

	var raw strings.Builder
	for _, c := range bodyNodes {
		raw.WriteString(f.text(c))
	}

	body := raw.String()
	if body == "" || strings.Contains(body, "\n") {
		return false
	}

	return strings.TrimLeft(body, " \t\r") == body && strings.TrimRight(body, " \t\r") == body
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
	//
	// Whitespace-only nodes are left out. Whether the gap between two tags is
	// captured as a node at all depends on stray trailing spaces in the source,
	// so counting one flipped this answer — and with it the choice between the
	// inline path and the blank-line-grouping path. The format removed the
	// trailing space, the next pass then took the other branch, and the file
	// oscillated between the two layouts.
	var combined strings.Builder

	for _, n := range nodes {
		if strings.TrimSpace(f.text(n)) == "" {
			continue
		}

		combined.WriteString(f.text(n))
	}

	trimmed := strings.TrimSpace(combined.String())
	if trimmed == "" {
		return false
	}

	return !strings.Contains(trimmed, "\n")
}

// hasBlockChild returns true if any node in the list is a block-level tag.
func (f *Formatter) hasBlockChild(nodes []*sitter.Node) bool {
	return slices.ContainsFunc(nodes, f.isBlockTagKind)
}

// formatInlineRun emits a sequence of inline nodes on a single line,
// preserving the original spacing between them.
func (f *Formatter) formatInlineRun(nodes []*sitter.Node) {
	var combined strings.Builder
	for _, n := range nodes {
		combined.WriteString(f.text(n))
	}

	trimmed := strings.TrimSpace(combined.String())
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
	var combined strings.Builder
	for _, n := range nodes {
		combined.WriteString(f.text(n))
	}

	trimmed := strings.TrimSpace(combined.String())
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

	// Computed once over the whole string, not per line: the offsets depend on
	// tag and quote state that a per-line scan of the remaining text cannot
	// see. Slicing off the first line of `<img src="a" alt="b c">` leaves
	// `alt="b c">`, which no longer starts inside a tag.
	breaks := safeBreaks(text)
	bi, start := 0, 0

	for {
		rest := text[start:]
		if len(rest) <= maxContent {
			f.writeIndent()
			f.write(rest)
			f.write("\n")

			return
		}

		for bi < len(breaks) && breaks[bi] <= start {
			bi++
		}

		limit := start + maxContent
		cut, j := -1, bi

		for j < len(breaks) && breaks[j] < limit {
			cut = breaks[j]
			j++
		}

		if cut < 0 {
			// Nothing fits; take the first break past the limit, or if there is
			// none, let the long line stand — LineWidth is a soft limit.
			if j >= len(breaks) {
				f.writeIndent()
				f.write(rest)
				f.write("\n")

				return
			}

			cut = breaks[j]
			j++
		}

		f.writeIndent()
		f.write(text[start:cut])
		f.write("\n")

		start, bi = cut+1, j
	}
}

// safeBreaks returns the offsets of every space in text at which a line break
// is safe.
//
// A space inside a tag's quoted attribute value is not one. writeWrapped is
// handed whole elements verbatim — the "emit this element as-is" path passes
// f.text(n), markup and attributes included — so a plain "last space before the
// limit" search happily broke a line in the middle of an attribute value:
//
//	<img src="x.png" alt="a fairly long alternative text describing the picture">
//
// became three lines with newlines and indentation inside the alt="…". The
// whitespace-only guard cannot catch that, because only whitespace changed; but
// the attribute's value did change, and for a CFML tag whose attribute carries a
// string the runtime uses — a cfhttpparam value, a cfmail subject — the injected
// newline and indent are in the data. It fired on 43 of the 5,504 formattable
// files across the six corpus projects.
//
// Quotes only delimit anything inside a tag. The same text stream carries
// ordinary prose, where an apostrophe is a letter: tracking quotes everywhere
// made "I won't display because…" unbreakable from the apostrophe onward,
// quietly disabling wrapping for the most ordinary English there is. A doubled
// quote is CFML's escape for a quote within a value, so it does not end one.
//
// An unmatched "<" in text — a stray literal rather than a tag — leaves this
// believing it is inside a tag for the rest of the string, so quotes past it
// start counting and fewer spaces qualify. That direction is safe: the cost is
// a wider line, never a break somewhere it does not belong.
func safeBreaks(text string) []int {
	var (
		out   []int
		inTag bool
		quote byte
	)

	for i := 0; i < len(text); i++ {
		c := text[i]

		switch {
		case quote != 0:
			if c != quote {
				continue
			}

			if i+1 < len(text) && text[i+1] == quote {
				i++ // escaped quote; the value continues

				continue
			}

			quote = 0
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case inTag && (c == '"' || c == '\''):
			quote = c
		case c == ' ':
			out = append(out, i)
		}
	}

	return out
}

// formatRawTextElement emits <script> and <style> elements. Their bodies are
// raw text (JavaScript / CSS), not CFML, so the content is never reflowed —
// only its indentation is normalized. The start and end tags are emitted from
// their own source text rather than reassembled from child tokens, because the
// generic child-walking path in formatNode drops the whitespace between the
// tag name and its attributes (producing `<styletype="text/css">`).
func (f *Formatter) formatRawTextElement(n *sitter.Node) {
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

	// No recognizable structure — emit the whole element verbatim.
	if startTag == nil {
		f.nl()
		f.writeIndent()
		f.write(strings.TrimSpace(f.text(n)))
		f.write("\n")

		return
	}

	var raw strings.Builder
	for _, c := range bodyNodes {
		raw.WriteString(f.text(c))
	}

	f.nl()
	f.writeIndent()
	f.write(strings.TrimSpace(f.text(startTag)))

	// A non-empty body goes on its own lines; an empty one keeps
	// <script ...></script> on a single line.
	if strings.TrimSpace(raw.String()) != "" {
		f.write("\n")
		f.write(reindentRawText(raw.String(), f.opts.indent(f.level+1)))
		f.writeIndent()
	}

	if endTag != nil {
		f.write(strings.TrimSpace(f.text(endTag)))
	}

	f.write("\n")
}

// reindentRawText strips the common leading whitespace from every non-blank
// line of raw <script>/<style> content and re-applies indent, preserving the
// relative indentation within the block. The returned text always ends in a
// newline.
func reindentRawText(raw, indent string) string {
	lines := strings.Split(raw, "\n")

	// Drop leading and trailing blank lines.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	common := commonLeadingWhitespace(lines)

	var b strings.Builder

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			b.WriteString("\n")

			continue
		}

		b.WriteString(indent)
		b.WriteString(strings.TrimRight(strings.TrimPrefix(line, common), " \t\r"))
		b.WriteString("\n")
	}

	return b.String()
}

// commonLeadingWhitespace returns the longest whitespace prefix shared by every
// non-blank line.
func commonLeadingWhitespace(lines []string) string {
	common := ""
	first := true

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		ws := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if first {
			common = ws
			first = false

			continue
		}

		common = common[:commonPrefixLen(common, ws)]
	}

	return common
}

func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}

	return n
}
