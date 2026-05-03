
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
	// attributes onto separate lines (default 120).
	LineWidth int
	// AttrBreakThreshold is the number of attributes above which they are
	// always expanded onto separate lines regardless of line width (default 3).
	AttrBreakThreshold int
	// UseTabs uses a single tab character instead of spaces for indentation.
	UseTabs bool
	// ParseScript re-parses cfscript content. If nil, script blocks are
	// emitted verbatim.
	ParseScript ParseFunc
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

func DefaultOptions() Options {
	return Options{
		IndentWidth:        4,
		LineWidth:          120,
		AttrBreakThreshold: 3,
	}
}

// blockTags are CF tags that contain child content and need indented bodies.
var blockTags = map[string]bool{
	"cfif": true, "cfelseif": true, "cfelse": true,
	"cfloop": true, "cfoutput": true, "cfquery": true,
	"cffunction": true, "cfcomponent": true,
	"cftry": true, "cfcatch": true, "cffinally": true,
	"cfswitch": true, "cfcase": true, "cfdefaultcase": true,
	"cftransaction": true, "cfthread": true, "cflock": true,
	"cfmail": true, "cfmailpart": true,
	"cfform": true, "cfgrid": true,
	"cflayout": true, "cflayoutarea": true,
	"cfdiv": true, "cfhtmlhead": true,
	"cfscript": true,
}

// selfClosingTags never have a separate closing tag.
var selfClosingTags = map[string]bool{
	"cfset": true, "cfparam": true, "cfreturn": true,
	"cfthrow": true, "cfabort": true, "cfbreak": true,
	"cfcontinue": true, "cfinvoke": true, "cfargument": true,
	"cfinclude": true, "cflocation": true, "cfcookie": true,
	"cfheader": true, "cfcontent": true, "cfflush": true,
	"cflog": true, "cfsetting": true, "cfprocessingdirective": true,
	"cfdump": true, "cfimage": true, "cfpdf": true,
}

// Formatter holds state during a single formatting pass.
type Formatter struct {
	opts    Options
	src     []byte
	out     bytes.Buffer
	level   int   // current indentation level
	atBOL   bool  // at beginning of line
	lastNL  bool  // last written byte was a newline
	lineLen int   // approximate current line length
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
	return &Formatter{opts: opts, atBOL: true}
}

// Format parses src with the provided tree-sitter parser and returns
// formatted CFML.
func Format(src []byte, tree *sitter.Tree, opts Options) ([]byte, error) {
	f := New(opts)
	f.src = src
	root := tree.RootNode()
	f.formatNode(root)
	return f.out.Bytes(), nil
}

// ─── node text helpers ───────────────────────────────────────────────────────

func (f *Formatter) text(n *sitter.Node) string {
	return string(f.src[n.StartByte():n.EndByte()])
}

func (f *Formatter) childByField(n *sitter.Node, field string) *sitter.Node {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if n.FieldNameForChild(uint32(i)) == field {
			return c
		}
	}
	return nil
}

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

func (f *Formatter) indented() string {
	return f.opts.indent(f.level)
}

func (f *Formatter) writeIndent() {
	if f.atBOL {
		f.write(f.indented())
	}
}

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
		case "cf_start_tag", "cf_tag_attributes":
			f.walkAttrs(c, attrs)
		case "cf_attribute":
			attr := cfAttr{}
			for j := uint(0); j < c.ChildCount(); j++ {
				gc := c.Child(j)
				switch gc.Kind() {
				case "cf_attribute_name":
					attr.name = strings.ToLower(f.text(gc))
				case "quoted_cf_attribute_value":
					attr.value = f.normaliseAttrValue(f.text(gc))
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

	// Multi-line rendering: one attribute per line, aligned after tag name.
	pad := strings.Repeat(" ", len(tagName)+2) // "<" + name + " "
	var sb strings.Builder
	for i, a := range attrs {
		sb.WriteString("\n")
		sb.WriteString(f.indented())
		sb.WriteString(pad)
		if a.value == "" {
			sb.WriteString(a.name)
		} else {
			sb.WriteString(fmt.Sprintf("%s=%s", a.name, a.value))
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
	switch {
	case kind == "program":
		f.formatChildren(n)

	case kind == "cf_tag":
		f.formatCFTag(n)

	case kind == "cf_set_tag", kind == "cf_return_tag":
		f.formatCFSelfClosingTag(n)

	case kind == "cf_selfclose_tag":
		f.formatCFSelfCloseAttrTag(n)

	case kind == "cf_if_tag":
		f.formatCFIfTag(n)

	case kind == "cf_output_tag", kind == "cf_query_tag":
		f.formatCFBlockTag(n)

	case kind == "cf_script_tag":
		f.formatCFScript(n)

	case kind == "html_text":
		f.formatText(n)

	case kind == "comment":
		f.formatComment(n)

	default:
		// HTML elements, expressions, etc. — pass through verbatim.
		if n.ChildCount() == 0 {
			f.write(f.text(n))
		} else {
			f.formatChildren(n)
		}
	}
}

func (f *Formatter) formatChildren(n *sitter.Node) {
	for i := uint(0); i < n.ChildCount(); i++ {
		f.formatNode(n.Child(i))
	}
}

// ─── CF tag formatting ───────────────────────────────────────────────────────

func (f *Formatter) tagName(n *sitter.Node) string {
	kind := n.Kind()

	// Generic cf_tag: look for cf_tag_name in cf_start_tag child.
	if kind == "cf_tag" || kind == "cf_start_tag" || kind == "cf_end_tag" {
		for i := uint(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c.Kind() == "cf_tag_name" {
				return "cf" + strings.ToLower(f.text(c))
			}
			if c.Kind() == "cf_start_tag" {
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
			if end > 0 {
				return "cf" + strings.ToLower(rest[:end])
			}
			return "cf" + strings.ToLower(rest)
		}
	}

	// Specific tags: cf_set_tag → cfset, cf_if_tag → cfif, etc.
	if strings.HasPrefix(kind, "cf_") && strings.HasSuffix(kind, "_tag") {
		inner := kind[3 : len(kind)-4] // strip "cf_" and "_tag"
		return "cf" + strings.ReplaceAll(inner, "_", "")
	}

	return ""
}

// formatCFTag handles generic cf_tag nodes (cffunction, cfloop, etc.)
// Structure: cf_start_tag + body children + cf_end_tag
func (f *Formatter) formatCFTag(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")
	f.write("\n")

	isBlock := blockTags[name]
	if isBlock {
		f.level++
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_end_tag":
			// already handled above/below
		default:
			f.formatNode(c)
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

// formatCFBlockTag handles specific block tags (cf_output_tag, cf_query_tag, etc.)
// that have inline children between <cf...> and </cf...>.
func (f *Formatter) formatCFBlockTag(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")
	f.write("\n")

	isBlock := blockTags[name]
	if isBlock {
		f.level++
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "<cf" || kind == "</cf" || kind == ">" || kind == "cf_tag_name" ||
			kind == "cf_tag_attributes" || kind == "cf_end_tag" ||
			kind == "cf_attribute" || kind == "cf_start_tag" {
			continue
		}
		// Content children (query text, output expressions, etc.)
		f.formatNode(c)
	}

	if isBlock {
		f.level--
	}
	f.nl()
	f.writeIndent()
	f.write("</" + name + ">")
	f.write("\n")
}

// formatCFIfTag handles cf_if_tag with its condition, body, and optional cf_if_alt.
func (f *Formatter) formatCFIfTag(n *sitter.Node) {
	// Collect condition expression (named children before ">")
	var condParts []string
	inBody := false
	var bodyNodes []*sitter.Node
	var altNode *sitter.Node

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
		if !inBody {
			if c.IsNamed() {
				condParts = append(condParts, f.text(c))
			}
		} else {
			bodyNodes = append(bodyNodes, c)
		}
	}

	cond := strings.Join(condParts, " ")
	f.nl()
	f.writeIndent()
	f.write("<cfif " + cond + ">")
	f.write("\n")
	f.level++

	for _, c := range bodyNodes {
		f.formatNode(c)
	}

	f.level--

	if altNode != nil {
		f.formatCFIfAlt(altNode)
	}

	f.nl()
	f.writeIndent()
	f.write("</cfif>")
	f.write("\n")
}

func (f *Formatter) formatCFIfAlt(n *sitter.Node) {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_elseif_tag":
			f.formatCFElseIf(c)
		case "cf_else_tag":
			f.formatCFElse(c)
		}
	}
}

func (f *Formatter) formatCFElseIf(n *sitter.Node) {
	var condParts []string
	var bodyNodes []*sitter.Node
	var altNode *sitter.Node
	inBody := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "<cf" {
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
		if !inBody {
			if c.IsNamed() {
				condParts = append(condParts, f.text(c))
			}
		} else {
			bodyNodes = append(bodyNodes, c)
		}
	}

	cond := strings.Join(condParts, " ")
	f.nl()
	f.writeIndent()
	f.write("<cfelseif " + cond + ">")
	f.write("\n")
	f.level++

	for _, c := range bodyNodes {
		f.formatNode(c)
	}

	f.level--

	if altNode != nil {
		f.formatCFIfAlt(altNode)
	}
}

func (f *Formatter) formatCFElse(n *sitter.Node) {
	f.nl()
	f.writeIndent()
	f.write("<cfelse>")
	f.write("\n")
	f.level++

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "<cf" || kind == ">" {
			continue
		}
		f.formatNode(c)
	}

	f.level--
}

// formatCFSelfCloseAttrTag handles cf_selfclose_tag (cfparam, cfargument, etc.)
// that have cf_attribute children.
func (f *Formatter) formatCFSelfCloseAttrTag(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")
	f.write("\n")
}

func (f *Formatter) formatCFSelfClosingTag(n *sitter.Node) {
	name := f.tagName(n)

	// For specific self-closing tags (cf_set_tag, cf_return_tag),
	// reconstruct from expression children.
	var exprParts []string
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := c.Kind()
		if kind == "<cf" || kind == "cf_selfclose_void_tag_end" || kind == ">" {
			continue
		}
		if c.IsNamed() {
			exprParts = append(exprParts, f.text(c))
		}
	}

	f.nl()
	f.writeIndent()
	body := strings.Join(exprParts, " ")
	if body != "" {
		f.write("<" + name + " " + body + ">")
	} else {
		f.write("<" + name + ">")
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

// ─── text / comment pass-through ─────────────────────────────────────────────

func (f *Formatter) formatText(n *sitter.Node) {
	raw := f.text(n)
	// Trim leading/trailing blank lines, preserve internal whitespace.
	lines := strings.Split(raw, "\n")
	var out []string
	for _, l := range lines {
		out = append(out, l)
	}
	// Strip purely-blank leading / trailing lines
	for len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		out = out[1:]
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		f.write("\n")
		return
	}
	for _, l := range out {
		f.writeIndent()
		f.write(strings.TrimRight(l, " \t"))
		f.write("\n")
	}
}

func (f *Formatter) formatComment(n *sitter.Node) {
	raw := f.text(n)
	f.nl()
	f.writeIndent()
	f.write(raw)
	f.write("\n")
}
