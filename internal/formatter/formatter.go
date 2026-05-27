// Package formatter walks a tree-sitter concrete syntax tree produced by the
// cfml grammar and re-emits well-formatted CFML source.
//
// Formatting rules applied
//   - Consistent 4-space indentation inside block-level CF tags.
//   - One attribute per line when there are more than [AttrBreakThreshold]
//     attributes, or when the tag would exceed [LineWidth] columns.
//   - Attribute values are always double-quoted.
//   - CF tag and attribute names are lower-cased.
//   - Blank lines inside CFScript blocks are preserved but capped at one.
//   - The closing </cfXxx> tag matches the indentation of the opening tag.
//   - HTML content and non-CF tags are re-emitted verbatim (pass-through).
package formatter

import (
	"bytes"
	"fmt"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseFunc parses source code and returns a tree-sitter tree.
// Used to re-parse cfscript content with the cfscript sub-grammar.
type ParseFunc func(src []byte) *sitter.Tree

// Options controls formatting behaviour.
type Options struct {
	// IndentWidth is the number of spaces per indentation level (default 4).
	IndentWidth int
	// LineWidth is the soft column limit used to decide whether to expand
	// attributes onto separate lines (default 100).
	LineWidth int
	// QueryLineWidth is the soft column limit for SQL inside cfquery (default 70).
	QueryLineWidth int
	// AttrBreakThreshold is the number of attributes above which they are
	// always expanded onto separate lines regardless of line width (default 4).
	AttrBreakThreshold int
	// UseTabs uses a single tab character instead of spaces for indentation.
	UseTabs bool
	// LowercaseTags lowercases CF tag names (default true).
	LowercaseTags bool
	// LowercaseAttributes lowercases attribute names (default true).
	LowercaseAttributes bool
	// DoubleQuoteAttributes normalizes attribute values to double quotes (default true).
	DoubleQuoteAttributes bool
	// QueryUppercaseKeywords uppercases SQL keywords in cfquery (default true).
	QueryUppercaseKeywords bool
	// QueryFormat controls whether cfquery content is formatted.
	// When false, query content is emitted verbatim. Default false.
	QueryFormat bool
	// ScopeCase controls the case of CFML scope names (variables, arguments, etc.).
	// Valid values: "upper", "lower", "leave" (default "leave").
	ScopeCase string
	// CommaPosition controls where commas appear in multi-line argument lists.
	// Valid values: "after" (trailing, default), "before" (leading).
	CommaPosition string
	// QueryCommaPosition controls where commas appear in SQL SELECT lists.
	// Defaults to the value of CommaPosition if empty.
	QueryCommaPosition string
	// ParseScript re-parses cfscript content. If nil, script blocks are
	// emitted verbatim.
	ParseScript ParseFunc
	// ParseQuery re-parses cfquery content. If nil, query blocks are
	// emitted verbatim.
	ParseQuery ParseFunc
	// ParseCFML re-parses CFML source after pre-formatting. If nil,
	// pre-formatting (e.g. converting implicit end tags to self-closing) is skipped.
	ParseCFML ParseFunc
	// SelfCloseTags controls whether void/implicit-end HTML tags are
	// converted to self-closing form (e.g. <br> → <br />). Default true.
	SelfCloseTags bool
	// WhitespaceOnly when true causes Format to return an error if the
	// output differs from the input in non-whitespace content.
	WhitespaceOnly bool
}

func (o Options) indent(level int) string {
	if o.UseTabs {
		return strings.Repeat("\t", level)
	}

	w := o.IndentWidth
	if w == 0 {
		w = 4
	}

	return strings.Repeat(" ", w*level)
}

func (o Options) queryCommaLeading() bool {
	return o.QueryCommaPosition == "before"
}

func (o Options) queryCommaPreserve() bool {
	if o.QueryCommaPosition != "" {
		return o.QueryCommaPosition == "preserve"
	}

	return true // default: preserve comma placement in SQL
}

// DefaultOptions returns Options with sensible defaults (4-space indent, 120 col width).
func DefaultOptions() Options {
	return Options{
		IndentWidth:            4,
		LineWidth:              100,
		QueryLineWidth:         70,
		AttrBreakThreshold:     4,
		UseTabs:                true,
		LowercaseTags:          true,
		LowercaseAttributes:    true,
		DoubleQuoteAttributes:  true,
		QueryUppercaseKeywords: true,
		QueryFormat:            false,
		SelfCloseTags:          true,
	}
}

// selfClosingTags never have a separate closing tag.
// var selfClosingTags = map[string]bool{
// 	"cfset": true, "cfparam": true, "cfreturn": true,
// 	"cfthrow": true, "cfabort": true, "cfbreak": true,
// 	"cfcontinue": true, "cfinvoke": true, "cfargument": true,
// 	"cfinclude": true, "cflocation": true, "cfcookie": true,
// 	"cfheader": true, "cfcontent": true, "cfflush": true,
// 	"cflog": true, "cfsetting": true, "cfprocessingdirective": true,
// 	"cfdump": true, "cfimage": true, "cfpdf": true,
// }

// Formatter holds state during a single formatting pass.
type Formatter struct {
	opts             Options
	src              []byte
	out              bytes.Buffer
	level            int   // current indentation level
	atBOL            bool  // at beginning of line
	lastNL           bool  // last written byte was a newline
	lineLen          int   // approximate current line length
	lastTagMultiLine bool  // last emitted tag had expanded (multi-line) attributes
	parseErr         error // first sub-parse error encountered
	pendingComma     bool  // deferred trailing comma to emit after next query item
}

// New creates a Formatter with the given options.
func New(opts Options) *Formatter {
	if opts.IndentWidth == 0 {
		opts.IndentWidth = 4
	}

	if opts.LineWidth == 0 {
		opts.LineWidth = 120
	}

	if opts.AttrBreakThreshold == 0 {
		opts.AttrBreakThreshold = 3
	}

	return &Formatter{opts: opts, atBOL: true, lastNL: true}
}

// Format parses src with the provided tree-sitter parser and returns
// formatted CFML.
func Format(src []byte, tree *sitter.Tree, opts Options) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = nil
			err = fmt.Errorf("formatter panic: %v", r)
		}
	}()

	if opts.SelfCloseTags && opts.ParseCFML != nil {
		src, tree = preformat(src, tree, opts.ParseCFML)
	}

	f := New(opts)
	f.src = src
	root := tree.RootNode()
	f.formatNode(root)

	if f.parseErr != nil {
		return nil, f.parseErr
	}

	out = f.out.Bytes()
	if opts.WhitespaceOnly {
		if err := checkWhitespaceOnly(src, out, opts.SelfCloseTags); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// checkWhitespaceOnly walks both byte slices skipping whitespace, comparing
// non-whitespace characters case-insensitively. If allowSelfClose is true,
// a "/" immediately before ">" in either side is also skipped.
// Returns nil if only whitespace differs, or an error describing the first mismatch.
func checkWhitespaceOnly(a, b []byte, allowSelfClose bool) error {
	i, j := 0, 0

	for {
		i = skipWSAndComments(a, i)
		j = skipWSAndComments(b, j)

		if i == len(a) && j == len(b) {
			return nil
		}

		if i == len(a) || j == len(b) {
			line := byteOffsetToLine(a, i)

			return fmt.Errorf("formatter made non-whitespace changes near line %d (content length mismatch)", line)
		}

		if allowSelfClose {
			if a[i] == '/' && i+1 < len(a) && a[i+1] == '>' && b[j] == '>' {
				i++

				continue
			}

			if b[j] == '/' && j+1 < len(b) && b[j+1] == '>' && a[i] == '>' {
				j++

				continue
			}
			// Allow added/removed quotes around attribute values
			if a[i] == '"' && b[j] != '"' {
				i++

				continue
			}

			if b[j] == '"' && a[i] != '"' {
				j++

				continue
			}

			if a[i] == '\'' && b[j] != '\'' {
				i++

				continue
			}

			if b[j] == '\'' && a[i] != '\'' {
				j++

				continue
			}
		}

		if toLower(a[i]) != toLower(b[j]) {
			line := byteOffsetToLine(a, i)
			ctx := snippetAt(a, i)

			return fmt.Errorf("formatter made non-whitespace changes at line %d near %q", line, ctx)
		}

		i++
		j++
	}
}

func byteOffsetToLine(src []byte, offset int) int {
	line := 1

	for k := 0; k < offset && k < len(src); k++ {
		if src[k] == '\n' {
			line++
		}
	}

	return line
}

func snippetAt(src []byte, offset int) string {
	start := offset - 10
	if start < 0 {
		start = 0
	}

	end := offset + 10
	if end > len(src) {
		end = len(src)
	}

	return string(src[start:end])
}

func isWS(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// skipWSAndComments advances past whitespace and CFML comments (<!--- ... --->).
func skipWSAndComments(src []byte, pos int) int {
	for {
		for pos < len(src) && isWS(src[pos]) {
			pos++
		}

		if pos+4 < len(src) && src[pos] == '<' && src[pos+1] == '!' && src[pos+2] == '-' && src[pos+3] == '-' && src[pos+4] == '-' {
			end := pos + 5
			for end+2 < len(src) {
				if src[end] == '-' && src[end+1] == '-' && src[end+2] == '-' && end+3 < len(src) && src[end+3] == '>' {
					pos = end + 4

					break
				}

				end++
			}

			if end+2 >= len(src) {
				break
			}

			continue
		}

		break
	}

	return pos
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}

	return c
}

// cfmlScopes are the built-in CFML scope names.
var cfmlScopes = map[string]bool{
	"variables":   true,
	"arguments":   true,
	"local":       true,
	"request":     true,
	"session":     true,
	"application": true,
	"server":      true,
	"form":        true,
	"url":         true,
	"cgi":         true,
	"cookie":      true,
	"client":      true,
	"this":        true,
	"super":       true,
}

// recordParseError stores the first parse error encountered during sub-parsing.
func (f *Formatter) recordParseError(context string, root *sitter.Node, src []byte, baseRow uint) {
	if f.parseErr != nil {
		return
	}

	errNode := findFirstError(root)
	if errNode == nil {
		f.parseErr = fmt.Errorf("parse error in %s block", context)

		return
	}

	pos := errNode.StartPosition()
	line := baseRow + pos.Row + 1

	snippet := string(src[errNode.StartByte():errNode.EndByte()])
	if len(snippet) > 50 {
		snippet = snippet[:50] + "..."
	}

	f.parseErr = fmt.Errorf("parse error in %s at line %d, col %d near %q", context, line, pos.Column+1, snippet)
}

func findFirstError(n *sitter.Node) *sitter.Node {
	if n.IsError() || n.IsMissing() {
		return n
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		if found := findFirstError(n.Child(i)); found != nil {
			return found
		}
	}

	return nil
}

// applyScopeCase transforms a CFML scope name based on the ScopeCase option.
func (f *Formatter) applyScopeCase(text string) string {
	if f.opts.ScopeCase == "" || f.opts.ScopeCase == "leave" {
		return text
	}

	if !cfmlScopes[strings.ToLower(text)] {
		return text
	}

	if f.opts.ScopeCase == "upper" {
		return strings.ToUpper(text)
	}

	return strings.ToLower(text)
}

// ─── node text helpers ───────────────────────────────────────────────────────

func (f *Formatter) text(n *sitter.Node) string {
	return string(f.src[n.StartByte():n.EndByte()])
}

// func (f *Formatter) childByField(n *sitter.Node, field string) *sitter.Node {
// 	for i := uint(0); i < n.ChildCount(); i++ {
// 		c := n.Child(i)
// 		if n.FieldNameForChild(uint32(i)) == field {
// 			return c
// 		}
// 	}
// 	return nil
// }

// ─── output helpers ──────────────────────────────────────────────────────────

func (f *Formatter) write(s string) {
	if s == "" {
		return
	}

	f.out.WriteString(s)
	// track approximate line length for soft-wrap decisions
	if idx := strings.LastIndexByte(s, '\n'); idx >= 0 {
		f.lineLen = len(s) - idx - 1
		f.lastNL = s[len(s)-1] == '\n'
		f.atBOL = f.lastNL
	} else {
		f.lineLen += len(s)
		f.lastNL = false
		f.atBOL = false
	}
}

func (f *Formatter) nl() {
	if !f.lastNL {
		f.write("\n")
	}
}

// appendTrailingComma inserts a comma before the trailing newline(s) in the
// output buffer. Used in trailing-comma mode when a source comma appears at
// the start of a new line — it gets moved to the end of the previous line.
// Returns true if the comma was successfully appended.
func (f *Formatter) appendTrailingComma() bool { //nolint:unparam // return used for future callers
	b := f.out.Bytes()
	// Walk backwards past trailing whitespace/newlines to find the last content line
	i := len(b) - 1
	for i >= 0 && (b[i] == '\n' || b[i] == '\r' || b[i] == ' ' || b[i] == '\t') {
		i--
	}

	if i < 0 {
		return false
	}
	// Insert comma after position i (after the last non-whitespace char)
	insertPos := i + 1

	f.out.Reset()
	f.out.Write(b[:insertPos])
	f.out.WriteByte(',')
	f.out.Write(b[insertPos:])

	return true
}

// flushPendingComma appends a deferred trailing comma after the last emitted
// content. Called after an item has been fully emitted.
// func (f *Formatter) flushPendingComma() {
// 	if !f.pendingComma {
// 		return
// 	}
// 	f.pendingComma = false
// 	f.appendTrailingComma()
// }

func (f *Formatter) indented() string {
	return f.opts.indent(f.level)
}

func (f *Formatter) writeIndent() {
	if f.atBOL {
		f.write(f.indented())
	}
}

// countIndentLevel counts the indentation level of a line, treating each tab
// as one level and each indentWidth spaces as one level.
// func (f *Formatter) countIndentLevel(line string, indentWidth int) int {
// 	level := 0
// 	spaces := 0
// 	for _, ch := range line {
// 		switch ch {
// 		case '\t':
// 			level++
// 			spaces = 0
// 		case ' ':
// 			spaces++
// 			if spaces >= indentWidth {
// 				level++
// 				spaces = 0
// 			}
// 		default:
// 			return level
// 		}
// 	}
// 	return level
// }

// ─── attribute formatting ────────────────────────────────────────────────────

type cfAttr struct {
	name  string
	value string // empty = boolean attribute
}

// collectAttrs gathers all cf_attribute children from a tag node,
// searching through cf_start_tag and cf_tag_attributes.
func (f *Formatter) collectAttrs(tag *sitter.Node) []cfAttr {
	var attrs []cfAttr

	f.walkAttrs(tag, &attrs)

	return attrs
}

func (f *Formatter) walkAttrs(n *sitter.Node, attrs *[]cfAttr) {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_start_tag_with_selfclose", "cf_tag_attributes":
			f.walkAttrs(c, attrs)
		case "cf_attribute":
			attr := cfAttr{}

			for j := uint(0); j < c.ChildCount(); j++ {
				gc := c.Child(j)
				switch gc.Kind() {
				case "cf_attribute_name":
					name := f.text(gc)
					if f.opts.LowercaseAttributes {
						name = strings.ToLower(name)
					}

					attr.name = name
				case "quoted_cf_attribute_value":
					if f.opts.DoubleQuoteAttributes {
						attr.value = f.normaliseAttrValue(f.text(gc))
					} else {
						attr.value = strings.TrimSpace(f.text(gc))
					}
				case "cf_attribute_value":
					val := strings.TrimSpace(f.text(gc))
					if f.opts.DoubleQuoteAttributes {
						attr.value = `"` + val + `"`
					} else {
						attr.value = val
					}
				}
			}

			*attrs = append(*attrs, attr)
		}
	}
}

// normaliseAttrValue ensures the value is wrapped in double quotes.
func (f *Formatter) normaliseAttrValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		q := v[0]
		if (q == '"' || q == '\'') && v[len(v)-1] == q {
			// already quoted — normalise to double quotes
			inner := v[1 : len(v)-1]
			inner = strings.ReplaceAll(inner, `"`, `'`)

			return `"` + inner + `"`
		}
	}

	return `"` + v + `"`
}

func (f *Formatter) renderAttrs(tagName string, attrs []cfAttr) string {
	if len(attrs) == 0 {
		return ""
	}

	// Inline rendering
	inline := " "

	for i, a := range attrs {
		if i > 0 {
			inline += " "
		}

		if a.value == "" {
			inline += a.name
		} else {
			inline += fmt.Sprintf("%s=%s", a.name, a.value)
		}
	}

	// Should we expand?
	oneLiner := "<" + tagName + inline
	expand := len(attrs) > f.opts.AttrBreakThreshold ||
		f.lineLen+len(oneLiner) > f.opts.LineWidth

	if !expand {
		return inline
	}

	// Multi-line rendering: one attribute per line, indented one level.
	indent := f.opts.indent(1)

	var sb strings.Builder
	for i, a := range attrs {
		sb.WriteString("\n")
		sb.WriteString(f.indented())
		sb.WriteString(indent)

		if a.value == "" {
			sb.WriteString(a.name)
		} else {
			sb.WriteString(fmt.Sprintf("%s=%s", a.name, a.value)) //nolint:staticcheck // QF1012: intentional for readability
		}

		if i < len(attrs)-1 {
			sb.WriteString(" ")
		}
	}

	return sb.String()
}

// ─── core traversal ──────────────────────────────────────────────────────────

func (f *Formatter) formatNode(n *sitter.Node) {
	kind := n.Kind()
	switch kind {
	case "program", "component_file":
		f.formatChildren(n)

	case "cf_component_content":
		f.formatComponentContent(n)

	case "cf_component_open_tag":
		f.formatCFComponentOpen(n)

	case "cf_component_close_tag":
		f.formatCFComponentClose(n)

	case "cf_tag":
		f.formatCFTag(n)

	case "cf_set_tag", "cf_return_tag":
		f.formatCFSelfClosingTag(n)

	case "cf_selfclose_tag":
		f.formatCFSelfCloseAttrTag(n)

	case "cf_if_tag":
		f.formatCFIfTag(n)

	case "cf_if_alt":
		f.formatCFIfAlt(n)

	case "cf_elseif_tag":
		f.formatCFElseIf(n)

	case "cf_else_tag":
		f.formatCFElse(n)

	case "cf_output_tag",
		"cf_function_tag",
		"cf_xml_tag":
		f.formatCFBlockTag(n)

	case "cf_savecontent_tag":
		f.formatCFSavecontent(n)

	case "cf_query_tag":
		f.formatCFQuery(n)

	case "cf_script_tag":
		f.formatCFScript(n)

	case "hash_expression":
		f.formatHashExpression(n)

	case "element":
		f.formatElement(n)

	case "html_text", "text":
		f.formatText(n)

	case "comment", "cf_comment":
		f.formatComment(n)

	case "cf_selfclose_void_tag_end":
		// Handled by parent (formatCFSelfClosingTag / formatCFSelfCloseAttrTag).
		// Emit verbatim if reached directly.
		f.write(f.text(n))

	case "implicit_end_tag":
		// Whitespace between tags — suppress since the formatter handles spacing.

	case "assignment_expression", "binary_expression",
		"unary_expression", "ternary_expression",
		"elvis_expression", "update_expression",
		"call_expression", "member_expression",
		"subscript_expression", "new_expression",
		"sequence_expression", "augmented_assignment_expression",
		"parenthesized_expression":
		// Expression nodes — normalize spacing via expr().
		f.write(f.expr(n))

	default:
		// Leaf nodes emit verbatim; non-leaf recurse into children.
		if n.ChildCount() == 0 {
			f.write(f.text(n))
		} else {
			f.formatChildren(n)
		}
	}
}

func (f *Formatter) formatChildren(n *sitter.Node) {
	prevTagKind := ""
	prevWasComment := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := f.nodeTagKind(c)

		if kind != "" && prevTagKind != "" && !prevWasComment &&
			(kind != prevTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
			f.write("\n")
		}

		f.formatNode(c)

		if kind != "" && !f.nodeIsComment(c) {
			prevTagKind = kind
		}

		prevWasComment = f.nodeIsComment(c)
	}
}

// nodeTagKind returns a string identifying the "tag group" of a node for
// blank-line-between-groups logic. Returns "" for non-tag nodes.
func (f *Formatter) nodeTagKind(n *sitter.Node) string {
	kind := n.Kind()
	switch kind {
	case "cf_set_tag":
		// Distinguish <cfset var ...> from <cfset ...> for grouping.
		text := f.text(n)
		if strings.Contains(text, " var ") || strings.Contains(text, "\tvar ") {
			return "cf_set_var_tag"
		}

		return kind
	case "cf_return_tag", "cf_selfclose_tag", "cf_tag",
		"cf_if_tag", "cf_output_tag", "cf_function_tag", "cf_query_tag",
		"cf_script_tag", "cf_xml_tag", "cf_savecontent_tag",
		"comment", "cf_comment":
		return kind
	default:
		return ""
	}
}

// nodeIsComment returns true if the node is a comment (used to avoid
// updating prevTagKind so comments don't cause blank lines after them).
func (f *Formatter) nodeIsComment(n *sitter.Node) bool {
	kind := n.Kind()

	return kind == "comment" || kind == "cf_comment"
}

// isBlockTagKind returns true if the node is a block-level CF tag that
// contains indented children (cfif, cfloop, cfoutput, etc.).
func (f *Formatter) isBlockTagKind(n *sitter.Node) bool {
	kind := n.Kind()
	switch kind {
	case "cf_if_tag", "cf_output_tag", "cf_function_tag", "cf_query_tag",
		"cf_script_tag", "cf_xml_tag", "cf_savecontent_tag":
		return true
	case "cf_tag":
		// A cf_tag with an end tag is a block tag.
		for i := uint(0); i < n.ChildCount(); i++ {
			if n.Child(i).Kind() == "cf_end_tag" {
				return true
			}
		}

		return false
	}

	return false
}

// firstBodyChildIsArg returns true if the first meaningful body child of a
// block tag is a cfargument tag.
func (f *Formatter) firstBodyChildIsArg(n *sitter.Node) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		if kind == "<cf" || kind == "</cf" || kind == ">" || kind == "cf_tag_name" ||
			kind == "cf_tag_attributes" || kind == "cf_end_tag" ||
			kind == "cf_attribute" || kind == "cf_start_tag" {
			continue
		}

		if kind == "cf_selfclose_tag" {
			return f.tagName(c) == "cfargument"
		}

		return false
	}

	return false
}

// ─── CF tag formatting ───────────────────────────────────────────────────────

func (f *Formatter) tagName(n *sitter.Node) string {
	kind := n.Kind()

	// Generic cf_tag: look for cf_tag_name in cf_start_tag child.
	if kind == "cf_tag" || kind == "cf_start_tag" || kind == "cf_end_tag" || kind == "cf_start_tag_with_selfclose" {
		for i := uint(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Kind() == "cf_tag_name" {
				name := f.text(c)
				if f.opts.LowercaseTags {
					name = strings.ToLower(name)
				}

				return "cf" + name
			}

			if c.Kind() == "cf_start_tag" || c.Kind() == "cf_start_tag_with_selfclose" {
				return f.tagName(c)
			}
		}
	}

	// cf_selfclose_tag: extract name from source text after "<cf"
	if kind == "cf_selfclose_tag" {
		raw := f.text(n)
		if strings.HasPrefix(strings.ToLower(raw), "<cf") {
			rest := raw[3:]
			end := strings.IndexAny(rest, " \t\r\n/>")
			name := rest

			if end > 0 {
				name = rest[:end]
			}

			if f.opts.LowercaseTags {
				name = strings.ToLower(name)
			}

			return "cf" + name
		}
	}

	// Specific tags: cf_set_tag → cfset, cf_if_tag → cfif, etc.
	if strings.HasPrefix(kind, "cf_") && strings.HasSuffix(kind, "_tag") && len(kind) > 7 {
		inner := kind[3 : len(kind)-4] // strip "cf_" and "_tag"

		return "cf" + strings.ReplaceAll(inner, "_", "")
	}

	return ""
}

// formatCFTag handles generic cf_tag nodes (cffunction, cfloop, etc.)
// Structure: cf_start_tag + body children + cf_end_tag
func (f *Formatter) formatCFTag(n *sitter.Node) {
	// Check if this is a self-closing cf_tag (e.g. <cftransaction action="commit" />)
	for i := uint(0); i < n.ChildCount(); i++ {
		if n.Child(i).Kind() == "cf_start_tag_with_selfclose" {
			f.formatCFSelfCloseAttrTag(n)

			return
		}
	}

	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")
	f.write("\n")

	isBlock := true // cf_tag with start+end is always a block
	if isBlock {
		f.level++
		f.write("\n")
	}

	prevCFTagKind := ""
	prevCFTagWasComment := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_end_tag":
			// already handled above/below
		default:
			tagKind := f.nodeTagKind(c)
			if tagKind != "" && prevCFTagKind != "" && !prevCFTagWasComment &&
				(tagKind != prevCFTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
				f.write("\n")
			}

			f.formatNode(c)

			if tagKind != "" && !f.nodeIsComment(c) {
				prevCFTagKind = tagKind
			}

			prevCFTagWasComment = f.nodeIsComment(c)
		}
	}

	if isBlock {
		f.level--
	}

	f.write("\n")
	f.nl()
	f.writeIndent()
	f.write("</" + name + ">")
	f.write("\n")
}

// formatCFComponentOpen handles cf_component_open_tag (a sibling node in the tree).
func (f *Formatter) formatCFComponentOpen(n *sitter.Node) {
	attrs := f.collectAttrs(n)
	f.nl()
	f.writeIndent()
	f.write("<cfcomponent" + f.renderAttrs("cfcomponent", attrs) + ">")
	f.write("\n\n")

	f.level++
}

// formatCFComponentClose handles cf_component_close_tag (a sibling node in the tree).
func (f *Formatter) formatCFComponentClose(_ *sitter.Node) {
	f.level--
	f.write("\n")
	f.nl()
	f.writeIndent()
	f.write("</cfcomponent>")
	f.write("\n")
}

// formatCFBlockTag handles specific block tags (cf_output_tag, cf_query_tag, etc.)
// that have inline children between <cf...> and </cf...>.
func (f *Formatter) formatCFBlockTag(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")
	f.write("\n")

	isBlock := true // specific block tag types are always blocks
	if isBlock {
		f.level++
		if name != "cfoutput" && (name != "cffunction" || !f.firstBodyChildIsArg(n)) {
			f.write("\n")
		}
	}

	prevTagKind := ""
	prevWasComment := false

	// Collect body nodes (skip syntax tokens).
	var bodyNodes []*sitter.Node

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		if kind == "<cf" || kind == "</cf" || kind == ">" || kind == "cf_tag_name" ||
			kind == "cf_tag_attributes" || kind == "cf_end_tag" ||
			kind == "cf_attribute" || kind == "cf_start_tag" {
			continue
		}

		bodyNodes = append(bodyNodes, c)
	}

	// If all body nodes are on a single line, emit them inline.
	if f.allSingleLine(bodyNodes) && !f.hasBlockChild(bodyNodes) && f.fitsOnLine(bodyNodes) {
		// Separate CF tags from inline text — CF tags get formatNode, text stays inline
		var textRun []*sitter.Node

		for _, c := range bodyNodes {
			if strings.HasPrefix(c.Kind(), "cf_") {
				if len(textRun) > 0 {
					f.formatInlineRun(textRun)
					textRun = nil
				}

				f.formatNode(c)
			} else {
				textRun = append(textRun, c)
			}
		}

		if len(textRun) > 0 {
			f.formatInlineRun(textRun)
		}
	} else {
		for i := 0; i < len(bodyNodes); {
			c := bodyNodes[i]
			kind := c.Kind()

			if f.isInlineNode(c) {
				// Collect consecutive inline nodes into a run.
				run := []*sitter.Node{c}
				j := i + 1

				for j < len(bodyNodes) && f.isInlineNode(bodyNodes[j]) {
					run = append(run, bodyNodes[j])
					j++
				}

				f.formatTextRun(run)

				i = j

				continue
			}

			// Insert blank line between groups of different tag types.
			tagKind := f.nodeTagKind(c)
			if tagKind != "" && prevTagKind != "" && !prevWasComment &&
				(tagKind != prevTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
				f.write("\n")
			}

			if kind == "comment" { //nolint:gocritic // ifElseChain: intentional for readability
				f.formatComment(c)
			} else {
				f.formatNode(c)
			}

			if tagKind != "" && kind != "comment" && kind != "cf_comment" {
				prevTagKind = tagKind
			}

			prevWasComment = kind == "comment" || kind == "cf_comment"
			i++
		}
	}

	if isBlock {
		f.level--
	}

	f.nl()
	f.writeIndent()
	f.write("</" + name + ">")
	f.write("\n")
}

// formatCFSavecontent emits the opening tag with proper indentation
// but preserves everything between (and including) the closing tag verbatim.
func (f *Formatter) formatCFSavecontent(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")

	// Find the body node and emit it verbatim
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		switch kind {
		case "cf_savecontent_body", "cf_savecontent_body_html",
			"cf_savecontent_body_script", "cf_savecontent_body_css",
			"cf_savecontent_body_xml", "cf_savecontent_body_sql",
			"cf_savecontent_body_raw":
			f.write(string(f.src[c.StartByte():c.EndByte()]))
		}
	}

	// Emit closing tag
	f.write("</" + name + ">")
	f.write("\n")
}

// normalizeCond collapses internal newlines and leading whitespace in a
// multi-line condition expression into properly indented continuation lines.
// Long conditions are broken at logical operators, with indentation reflecting
// parenthesis depth for readability.
func (f *Formatter) normalizeCond(raw string) string {
	// Collapse to single line first.
	lines := strings.Split(raw, "\n")

	var parts []string

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}

	single := strings.Join(parts, " ")

	// Check if it fits on one line.
	tagPrefix := f.lineLen
	if tagPrefix+len(single) <= f.opts.LineWidth {
		return single
	}

	// Break at logical operators, indenting by paren depth.
	baseIndent := f.indented() + f.opts.indent(1)

	var result strings.Builder

	i := 0
	for i < len(single) {
		ch := single[i]

		if i > 0 {
			for _, op := range condBreakOperators {
				if matchWord(single, i, op) {
					result.WriteString("\n")
					result.WriteString(baseIndent)

					break
				}
			}
		}

		result.WriteByte(ch)

		i++
	}

	return result.String()
}

// condBreakOperators are CFML logical operators where long conditions should break.
var condBreakOperators = []string{"&&", "||", "AND", "OR", "XOR", "EQV", "IMP"}

// matchWord checks if s[pos:] starts with word preceded and followed by a space.
func matchWord(s string, pos int, word string) bool {
	if pos == 0 || s[pos-1] != ' ' {
		return false
	}

	if pos+len(word) > len(s) {
		return false
	}

	if s[pos:pos+len(word)] != word {
		return false
	}

	if pos+len(word) < len(s) && s[pos+len(word)] != ' ' {
		return false
	}

	return true
}

// formatCFIfTag handles cf_if_tag with its condition, body, and optional cf_if_alt.
func (f *Formatter) formatCFIfTag(n *sitter.Node) {
	// Collect condition expression (named children before ">")
	var condParts []string

	inBody := false

	var bodyNodes []*sitter.Node

	var altNode *sitter.Node

	var trailingNodes []*sitter.Node

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		if kind == "<cf" || kind == "</cf" {
			continue
		}

		if kind == ">" {
			inBody = true

			continue
		}

		if kind == "cf_if_alt" {
			altNode = c

			continue
		}

		switch {
		case !inBody:
			if c.IsNamed() {
				condParts = append(condParts, f.expr(c))
			}
		case altNode != nil:
			// Nodes after cf_if_alt belong to the last branch (tree-sitter
			// sometimes places trailing comments as siblings of cf_if_alt).
			trailingNodes = append(trailingNodes, c)
		default:
			bodyNodes = append(bodyNodes, c)
		}
	}

	cond := f.normalizeCond(strings.Join(condParts, " "))
	f.nl()
	f.writeIndent()
	f.write("<cfif " + cond + ">")
	f.write("\n")

	f.level++
	f.write("\n")

	prevTagKind2 := ""
	prevWasComment2 := false

	for _, c := range bodyNodes {
		tagKind := f.nodeTagKind(c)
		if tagKind != "" && prevTagKind2 != "" && !prevWasComment2 &&
			(tagKind != prevTagKind2 || f.isBlockTagKind(c) || f.lastTagMultiLine) {
			f.write("\n")
		}

		f.formatNode(c)

		if tagKind != "" && !f.nodeIsComment(c) {
			prevTagKind2 = tagKind
		}

		prevWasComment2 = f.nodeIsComment(c)
	}

	f.level--

	if altNode != nil {
		f.formatCFIfAlt(altNode)
	}

	// Emit nodes that tree-sitter placed after cf_if_alt (e.g. trailing
	// comments) — they belong to the last branch.
	if len(trailingNodes) > 0 {
		f.level++
		for _, c := range trailingNodes {
			f.formatNode(c)
		}

		f.level--
	}

	f.write("\n")
	f.nl()
	f.writeIndent()
	f.write("</cfif>")
	f.write("\n")
}

func (f *Formatter) formatCFIfAlt(n *sitter.Node) {
	// cf_if_alt structure:
	//   <cf (anonymous), cf_elseif_tag (condition + ">"), body nodes..., cf_if_alt (nested)
	//   OR: <cf (anonymous), cf_else_tag (">"), body nodes...
	// Body nodes are siblings of cf_elseif_tag/cf_else_tag, not children.
	var condParts []string

	var bodyNodes []*sitter.Node

	var altNode *sitter.Node

	isElse := false
	inBody := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		switch kind {
		case "<cf":
			continue
		case "cf_elseif_tag":
			// Extract condition from inside cf_elseif_tag
			for j := uint(0); j < c.ChildCount(); j++ {
				gc := c.Child(j)
				if gc.Kind() == ">" {
					break
				}

				if gc.IsNamed() {
					condParts = append(condParts, f.expr(gc))
				}
			}

			inBody = true
		case "cf_else_tag":
			isElse = true
			inBody = true
		case "cf_if_alt":
			altNode = c
		default:
			if inBody {
				bodyNodes = append(bodyNodes, c)
			}
		}
	}

	if isElse {
		f.write("\n")
		f.nl()
		f.writeIndent()
		f.write("<cfelse>")
		f.write("\n")
	} else {
		cond := f.normalizeCond(strings.Join(condParts, " "))
		f.write("\n")
		f.nl()
		f.writeIndent()
		f.write("<cfelseif " + cond + ">")
		f.write("\n")
	}

	f.level++
	f.write("\n")

	prevAltTagKind := ""
	prevAltWasComment := false

	for _, c := range bodyNodes {
		tagKind := f.nodeTagKind(c)
		if tagKind != "" && prevAltTagKind != "" && !prevAltWasComment &&
			(tagKind != prevAltTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
			f.write("\n")
		}

		f.formatNode(c)

		if tagKind != "" && !f.nodeIsComment(c) {
			prevAltTagKind = tagKind
		}

		prevAltWasComment = f.nodeIsComment(c)
	}

	f.level--

	if altNode != nil {
		f.formatCFIfAlt(altNode)
	}
}

func (f *Formatter) formatCFElseIf(n *sitter.Node) {
	// Called when cf_elseif_tag is visited directly (not via formatCFIfAlt).
	// In practice, formatCFIfAlt handles this inline, but keep for safety.
	f.write(f.text(n))
}

func (f *Formatter) formatCFElse(n *sitter.Node) {
	// Called when cf_else_tag is visited directly (not via formatCFIfAlt).
	// In practice, formatCFIfAlt handles this inline, but keep for safety.
	f.write(f.text(n))
}

// formatCFSelfCloseAttrTag handles cf_selfclose_tag (cfparam, cfargument, etc.)
// that have cf_attribute children.
func (f *Formatter) formatCFSelfCloseAttrTag(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	rendered := f.renderAttrs(name, attrs)
	f.lastTagMultiLine = strings.Contains(rendered, "\n")
	f.nl()
	f.writeIndent()
	f.write("<" + name + rendered + " />")
	f.write("\n")
}

func (f *Formatter) formatCFSelfClosingTag(n *sitter.Node) {
	f.lastTagMultiLine = false
	name := f.tagName(n)

	// For specific self-closing tags (cf_set_tag, cf_return_tag),
	// reconstruct from expression children with normalized spacing.
	var exprParts []string

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		if kind == "<cf" || kind == "cf_selfclose_void_tag_end" || kind == ">" {
			continue
		}

		if c.IsNamed() {
			exprParts = append(exprParts, f.expr(c))
		}
	}

	body := strings.Join(exprParts, " ")

	// If the body spans multiple lines (e.g. multi-line function args),
	// emit with re-indentation based on the normalized expression.
	if strings.Contains(body, "\n") {
		f.lastTagMultiLine = true
		lines := strings.Split(body, "\n")

		f.nl()
		f.writeIndent()
		f.write("<" + name + " " + lines[0])
		f.write("\n")

		for i, l := range lines[1:] {
			if strings.TrimSpace(l) == "" {
				f.write("\n")

				continue
			}

			f.write(l)

			if i == len(lines)-2 {
				f.write(" />")
			}

			f.write("\n")
		}

		return
	}

	f.nl()
	f.writeIndent()

	if body != "" {
		f.write("<" + name + " " + body + " />")
	} else {
		f.write("<" + name + " />")
	}

	f.write("\n")
}

// ─── CFScript formatting ─────────────────────────────────────────────────────

// formatCFScript pretty-prints the contents of a <cfscript>…</cfscript> block
// by recursing into the cfscript sub-grammar nodes via cfscript_formatter.go.
func (f *Formatter) formatCFScript(n *sitter.Node) {
	f.nl()
	f.writeIndent()
	f.write("<cfscript>\n")

	f.level++

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() == "cf_script_content" && f.opts.ParseScript != nil {
			scriptSrc := f.src[c.StartByte():c.EndByte()]

			tree := f.opts.ParseScript(scriptSrc)
			if tree != nil {
				if tree.RootNode().HasError() {
					f.recordParseError("cfscript", tree.RootNode(), scriptSrc, c.StartPosition().Row)
				}
				defer tree.Close()

				origSrc := f.src
				f.src = scriptSrc
				f.formatScriptChildren(tree.RootNode())
				f.src = origSrc
			}
		}
	}

	f.level--
	f.nl()
	f.writeIndent()
	f.write("</cfscript>\n")
}

func (f *Formatter) formatScriptChildren(n *sitter.Node) {
	prevEndRow := int(n.StartPosition().Row)

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		startRow := int(c.StartPosition().Row)
		if startRow-prevEndRow > 1 {
			f.write("\n")
		}

		prevEndRow = int(c.EndPosition().Row)
		f.formatScriptNode(c)
	}
}

// ─── hash expression ─────────────────────────────────────────────────────────

// formatHashExpression emits #expr# preserving the delimiters.
// The opening # is an external token not exposed as a child node.
func (f *Formatter) formatHashExpression(n *sitter.Node) {
	f.write("#")

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.Kind() == "#" {
			f.write("#")
		} else {
			f.write(f.text(c))
		}
	}
}

// ─── component file ──────────────────────────────────────────────────────────

// formatComponentContent formats a script-based component file by re-parsing
// the content with the CFScript grammar.
func (f *Formatter) formatComponentContent(n *sitter.Node) {
	if f.opts.ParseScript != nil {
		scriptSrc := f.src[n.StartByte():n.EndByte()]

		tree := f.opts.ParseScript(scriptSrc)
		if tree != nil {
			if tree.RootNode().HasError() {
				f.recordParseError("cfscript", tree.RootNode(), scriptSrc, n.StartPosition().Row)
			}
			defer tree.Close()

			origSrc := f.src
			f.src = scriptSrc
			f.formatScriptChildren(tree.RootNode())
			f.src = origSrc

			return
		}
	}

	f.write(f.text(n))
}

// ─── text / comment pass-through ─────────────────────────────────────────────

func (f *Formatter) formatText(n *sitter.Node) {
	raw := f.text(n)
	// Suppress whitespace-only text nodes (spacing between tags is managed by the formatter).
	if strings.TrimSpace(raw) == "" {
		return
	}
	// Collapse all whitespace to single spaces (HTML whitespace rules).
	f.writeWrapped(collapseWhitespace(strings.TrimSpace(raw)))
}

func (f *Formatter) formatComment(n *sitter.Node) {
	raw := f.normalizeCFComment(strings.TrimSpace(f.text(n)))
	if raw == "" {
		return
	}

	f.nl()
	f.writeIndent()
	f.write(raw)
	f.write("\n")
}

// normalizeCFComment collapses extra internal whitespace in <!--- ... ---> comments.
func (f *Formatter) normalizeCFComment(raw string) string {
	if !strings.HasPrefix(raw, "<!---") || !strings.HasSuffix(raw, "--->") {
		return raw
	}

	inner := raw[5 : len(raw)-4]

	lines := strings.Split(inner, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) == 0 {
		return "<!--- --->"
	}

	if len(lines) == 1 {
		return "<!--- " + strings.TrimSpace(lines[0]) + " --->"
	}
	// Multi-line: strip common leading whitespace
	minIndent := -1

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent > 0 {
		for i, line := range lines {
			if len(line) >= minIndent {
				lines[i] = line[minIndent:]
			}
		}
	}

	baseIndent := f.opts.indent(f.level)

	var sb strings.Builder

	sb.WriteString("<!---\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			sb.WriteString("\n")
		} else {
			sb.WriteString(baseIndent + f.opts.indent(1) + line + "\n")
		}
	}

	sb.WriteString(baseIndent + "--->")

	return sb.String()
}
