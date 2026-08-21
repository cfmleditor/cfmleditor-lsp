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
	"slices"
	"strings"
	"unicode/utf8"

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

// utf8BOM is the UTF-8 byte-order mark. tree-sitter reports it as trivia
// belonging to no node, so it has to be preserved explicitly.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

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
	// pendingBlockComments holds comments found between a construct's header
	// and its body, to be emitted just inside that body once it opens.
	pendingBlockComments []*sitter.Node
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

	// Refuse a tree the grammar could not parse. The node walk has no
	// meaningful rendering for an ERROR node and falls through to a raw
	// emit that concatenates its children without separators, producing
	// output that is not valid CFML. Bailing out here means a parse
	// failure can never be written over the user's source.
	if root.HasError() {
		return nil, parseErrorAt("document", root, src, 0)
	}

	f.formatNode(root)

	if f.parseErr != nil {
		return nil, f.parseErr
	}

	out = f.out.Bytes()

	// A leading UTF-8 BOM sits outside every CST node, so the walk never
	// emits it. Carry it across verbatim — dropping it rewrites the file's
	// encoding preamble, which some CFML engines are sensitive to.
	if bytes.HasPrefix(src, utf8BOM) && !bytes.HasPrefix(out, utf8BOM) {
		out = append(append([]byte{}, utf8BOM...), out...)
	}

	if opts.WhitespaceOnly {
		if err := checkWhitespaceOnly(src, out, opts.SelfCloseTags, opts.DoubleQuoteAttributes); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// checkWhitespaceOnly walks both byte slices skipping whitespace, comparing
// non-whitespace characters case-insensitively. If allowSelfClose is true,
// a "/" immediately before ">" in either side is also skipped; if allowRequote
// is true, the attribute re-quoting described below is skipped too.
// Returns nil if only whitespace differs, or an error describing the first mismatch.
// isQuoteByte reports whether c delimits an attribute value or string literal.
func isQuoteByte(c byte) bool {
	return c == '"' || c == '\''
}

func checkWhitespaceOnly(a, b []byte, allowSelfClose, allowRequote bool) error {
	i, j := 0, 0

	// The formatter canonicalises cfscript as it goes: a statement written
	// without its optional semicolon gains one, and a single-statement body
	// gains braces. Both are deliberate insertions, so an extra ";", "{" or
	// "}" on the output side is not a defect. The allowance is one-directional
	// — a token the formatter *dropped* is still reported — and the braces are
	// counted, so an unmatched one is still caught below.
	openedAdded, closedAdded := 0, 0

	// Comment bodies are gathered as the walk steps over them and compared
	// once both sides are exhausted. See skipWSAndComments for why comments
	// cannot be compared in line with the surrounding code.
	var commentsA, commentsB []byte

	scriptA, scriptB := scriptRegionsOf(a), scriptRegionsOf(b)

	for {
		i = skipWSAndComments(a, i, &commentsA, scriptA)
		j = skipWSAndComments(b, j, &commentsB, scriptB)

		if j < len(b) && isNormalizationToken(b[j]) &&
			(i >= len(a) || toLower(a[i]) != toLower(b[j])) {
			switch b[j] {
			case '{':
				openedAdded++
			case '}':
				closedAdded++
			}

			j++

			continue
		}

		if i == len(a) && j == len(b) {
			if openedAdded != closedAdded {
				return fmt.Errorf("formatter added %d unmatched brace(s)", openedAdded-closedAdded)
			}

			return compareCommentBodies(a, commentsA, commentsB)
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
		}

		// Attribute re-quoting is a deliberate canonicalisation, and only ever
		// runs in one of two shapes (normaliseAttrValue): an unquoted value
		// gains quotes, or a single-quoted one is upgraded to double. Those are
		// a quote the output has and the source does not, and a quote on each
		// side that differ.
		//
		// A quote the formatter *dropped* is neither, and allowing it is what
		// blinded the guard: the allowance was written as "any mismatched quote
		// on either side", so the formatter stripping the quotes off a CFML
		// string — `<cfset msg = "hello world">` coming back as
		// `<cfset msg = hello world>`, or `SELECT 'a'` as `SELECT a` — passed
		// as whitespace-only. A removal is compared like any other byte now.
		if allowRequote {
			if isQuoteByte(a[i]) && isQuoteByte(b[j]) && a[i] != b[j] {
				i++
				j++

				continue
			}

			if isQuoteByte(b[j]) && !isQuoteByte(a[i]) {
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

// isNormalizationToken reports whether c is one of the tokens the formatter
// inserts as part of canonicalising cfscript.
func isNormalizationToken(c byte) bool {
	return c == ';' || c == '{' || c == '}'
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
	start := max(offset-10, 0)

	end := min(offset+10, len(src))

	return string(src[start:end])
}

func isWS(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// spaceLen returns the byte length of the whitespace character at pos, or 0 if
// there is none. Unlike isWS it decodes multi-byte runes, because indentation
// is not always ASCII: source pasted from a word processor or a browser arrives
// indented with U+2003 EM SPACE and the like. Re-indenting such a line is a
// whitespace change, but measuring it a byte at a time made it look like the
// formatter had deleted content, and the file was refused.
//
// Only *breaking* spaces count. U+00A0 NO-BREAK SPACE and U+202F NARROW NO-BREAK
// SPACE render differently from an ordinary space and are load-bearing in
// markup, so they stay content: losing one has to be reported, not waved through.
func spaceLen(src []byte, pos int) int {
	if pos < 0 || pos >= len(src) {
		return 0
	}

	if c := src[pos]; c < utf8.RuneSelf {
		if isWS(c) || c == '\v' || c == '\f' {
			return 1
		}

		return 0
	}

	r, size := utf8.DecodeRune(src[pos:])

	switch {
	case r >= 0x2000 && r <= 0x200A: // EN QUAD … HAIR SPACE
		return size
	case r == 0x0085, // NEXT LINE
		r == 0x1680, // OGHAM SPACE MARK
		r == 0x2028, // LINE SEPARATOR
		r == 0x2029, // PARAGRAPH SEPARATOR
		r == 0x205F, // MEDIUM MATHEMATICAL SPACE
		r == 0x3000: // IDEOGRAPHIC SPACE
		return size
	}

	return 0
}

// skipSpace advances past a run of whitespace, ASCII or otherwise.
func skipSpace(src []byte, pos int) int {
	for {
		n := spaceLen(src, pos)
		if n == 0 {
			return pos
		}

		pos += n
	}
}

// scriptSpans marks the byte ranges of a document that hold script rather than
// markup. Only inside those does "//" open a comment: in markup the same two
// characters are ordinary content, and appear in places as mundane as a
// DOCTYPE ("-//W3C//DTD XHTML 1.0 Strict//EN"). Reading one of those as a
// comment would swallow the rest of its line and reject a perfectly good
// reformat of it.
type scriptSpans []struct{ start, end int }

func (s scriptSpans) contains(pos int) bool {
	for _, sp := range s {
		if pos >= sp.start && pos < sp.end {
			return true
		}
	}

	return false
}

// scriptRegionsOf locates the <cfscript> and <script> blocks in src, or spans
// the whole file when it is a script-syntax component.
func scriptRegionsOf(src []byte) scriptSpans {
	if isScriptSyntaxComponent(src) {
		return scriptSpans{{0, len(src)}}
	}

	var spans scriptSpans

	lower := bytes.ToLower(src)

	for _, tag := range []string{"cfscript", "script"} {
		open, closeTag := "<"+tag, "</"+tag

		for pos := 0; ; {
			start := indexBytesFrom(lower, pos, open)
			if start < 0 {
				break
			}

			// Require a tag boundary so <script> does not also match
			// <scripting> and <cfscript> is not matched twice.
			after := start + len(open)
			if after < len(lower) && !isTagNameEnd(lower[after]) {
				pos = after

				continue
			}

			body := indexBytesFrom(lower, after, ">")
			if body < 0 {
				break
			}

			end := indexBytesFrom(lower, body+1, closeTag)
			if end < 0 {
				end = len(src)
			}

			spans = append(spans, struct{ start, end int }{body + 1, end})
			pos = end
		}
	}

	return spans
}
func isTagNameEnd(c byte) bool {
	return isWS(c) || c == '>' || c == '/'
}

// isScriptSyntaxComponent reports whether src is a script-syntax component,
// which has no <cfscript> tag to key off because the entire file is script.
func isScriptSyntaxComponent(src []byte) bool {
	// Probe with the whole file marked as script so a leading /** */ doc block
	// is stepped over; only the keyword that follows it matters here.
	all := scriptSpans{{0, len(src)}}

	pos := skipWSAndComments(src, 0, nil, all)

	for _, kw := range []string{"abstract", "final"} {
		if hasWordAt(src, pos, kw) {
			pos = skipWSAndComments(src, pos+len(kw), nil, all)
		}
	}

	return hasWordAt(src, pos, "component") || hasWordAt(src, pos, "interface")
}

// hasWordAt reports whether word sits at pos, case-insensitively, and is not
// merely the prefix of a longer identifier.
func hasWordAt(src []byte, pos int, word string) bool {
	if pos < 0 || pos+len(word) > len(src) {
		return false
	}

	for k := range len(word) {
		if toLower(src[pos+k]) != word[k] {
			return false
		}
	}

	after := pos + len(word)

	return after >= len(src) || !isIdentByte(src[after])
}
func isIdentByte(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= '0' && c <= '9') ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z')
}
func hasBytesAt(src []byte, pos int, lit string) bool {
	return pos >= 0 && pos+len(lit) <= len(src) && string(src[pos:pos+len(lit)]) == lit
}
func indexBytesFrom(src []byte, pos int, lit string) int {
	if pos < 0 || pos > len(src) {
		return -1
	}

	idx := bytes.Index(src[pos:], []byte(lit))
	if idx < 0 {
		return -1
	}

	return pos + idx
}

// headerEnd returns the offset just past the ">" closing the opening tag that
// begins at start. Quoted values and comments are stepped over so that a ">"
// inside either does not end the header early.
//
// Node shape cannot answer this on its own: a tag's attributes and its body
// can be siblings under one node (cf_output_tag holds body comments as direct
// children), so only the position of the ">" separates the two.
func headerEnd(src []byte, start int) int {
	i := start
	for i < len(src) {
		switch {
		case hasBytesAt(src, i, "<!---"):
			end := indexBytesFrom(src, i+5, "--->")
			if end < 0 {
				return len(src)
			}

			i = end + 4
		case src[i] == '"' || src[i] == '\'':
			q := src[i]
			i++

			for i < len(src) && src[i] != q {
				i++
			}

			i++
		case src[i] == '>':
			return i + 1
		default:
			i++
		}
	}

	return len(src)
}

// hasMultiLineComment reports whether any entry is a comment spanning more than
// one line. Such a comment cannot share a line with the attributes around it,
// so the whole list has to go one-per-line.
func hasMultiLineComment(attrs []cfAttr) bool {
	for _, a := range attrs {
		if a.comment && strings.Contains(a.name, "\n") {
			return true
		}
	}

	return false
}

// collectCommentBody appends body's non-whitespace bytes, folded to lower case,
// so that reindenting or rewrapping a comment does not register as a change.
//
// A "/" directly before a ">" is dropped as well. Commented-out markup is
// common, and the self-closing pass rewrites "<cfargument ...>" to
// "<cfargument ... />" wherever it appears, comments included. The code side of
// the comparison already tolerates that same difference.
func collectCommentBody(sink *[]byte, body []byte) {
	if sink == nil {
		return
	}

	for k := 0; k < len(body); {
		if n := spaceLen(body, k); n > 0 {
			k += n

			continue
		}

		c := body[k]
		k++

		if c == '/' && nextNonWS(body, k) == '>' {
			continue
		}

		*sink = append(*sink, toLower(c))
	}
}

// nextNonWS returns the first non-whitespace byte at or after pos, or 0.
func nextNonWS(src []byte, pos int) byte {
	pos = skipSpace(src, pos)
	if pos < len(src) {
		return src[pos]
	}

	return 0
}

// isLineCommentStart reports whether a "//" comment opens at pos. The caller
// has already established that pos sits in script, where "//" is a comment
// rather than the content it would be in markup.
func isLineCommentStart(src []byte, pos int) bool {
	if pos+1 >= len(src) || src[pos] != '/' || src[pos+1] != '/' {
		return false
	}

	// A scheme separator immediately before the slashes marks a URL
	// ("http://host") rather than a comment. Reading it as one would swallow
	// the rest of the line and turn any reflow of that line into a spurious
	// rejection.
	return pos == 0 || src[pos-1] != ':'
}

// compareCommentBodies reports whether the comment text survived the format
// unchanged. Both sides arrive stripped of whitespace and folded to lower case,
// so reindenting and rewrapping are already accounted for; anything left is a
// real edit to what a comment says.
func compareCommentBodies(src, a, b []byte) error {
	if bytes.Equal(a, b) {
		return nil
	}

	at := 0
	for at < len(a) && at < len(b) && a[at] == b[at] {
		at++
	}

	// Locate the divergence in the original so the message points somewhere
	// the reader can actually look.
	line := byteOffsetToLine(src, commentBodyOffset(src, at))

	return fmt.Errorf("formatter changed comment text near line %d (%q became %q)",
		line, commentSnippet(a, at), commentSnippet(b, at))
}

// commentBodyOffset maps an index into the collected comment bytes back to a
// byte offset in src, by re-walking src and counting comment body bytes.
func commentBodyOffset(src []byte, target int) int {
	script := scriptRegionsOf(src)
	seen, pos := 0, 0

	for pos < len(src) {
		var body []byte

		next := skipWSAndComments(src, pos, &body, script)
		if next == pos {
			pos++

			continue
		}

		if seen+len(body) > target {
			return next
		}

		seen += len(body)
		pos = next
	}

	return len(src)
}
func commentSnippet(s []byte, at int) string {
	start := max(at-10, 0)

	end := min(at+20, len(s))

	return string(s[start:end])
}

// skipWSAndComments advances past whitespace and comments, appending the
// non-whitespace body of every comment it skips to sink when that is non-nil.
//
// Comments are stepped over rather than compared character by character
// because a comment's *extent* is what carries meaning. A "//" comment runs to
// the end of its line, so deleting that newline silently pulls whatever
// followed on the next line into the comment. A raw character walk cannot see
// that — joining two lines only removes whitespace, leaving the non-whitespace
// sequence identical — whereas skipping each comment and then requiring the
// surrounding code to line up catches it on the spot. The bodies collected in
// sink are compared separately, so a comment whose text was mangled is still
// reported even though its characters no longer take part in the walk.
func skipWSAndComments(src []byte, pos int, sink *[]byte, script scriptSpans) int {
	for {
		pos = skipSpace(src, pos)

		switch {
		case hasBytesAt(src, pos, "<!---"):
			end := indexBytesFrom(src, pos+5, "--->")
			if end < 0 {
				return pos
			}

			collectCommentBody(sink, src[pos+5:end])
			pos = end + 4
		case hasBytesAt(src, pos, "<!--"):
			// An HTML comment. It has to be recognised in its own right, not
			// left to the "//" rule below, because its body routinely contains
			// slashes ("<!-- // end-of-template -->") that would otherwise be
			// read as a line comment and swallow the rest of the line.
			end := indexBytesFrom(src, pos+4, "-->")
			if end < 0 {
				return pos
			}

			collectCommentBody(sink, src[pos+4:end])
			pos = end + 3
		case script.contains(pos) && hasBytesAt(src, pos, "/*"):
			end := indexBytesFrom(src, pos+2, "*/")
			if end < 0 {
				return pos
			}

			collectCommentBody(sink, src[pos+2:end])
			pos = end + 2
		case script.contains(pos) && isLineCommentStart(src, pos):
			end := pos + 2
			for end < len(src) && src[end] != '\n' {
				end++
			}

			collectCommentBody(sink, src[pos+2:end])
			pos = end
		default:
			return pos
		}
	}
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

	f.parseErr = parseErrorAt(context, root, src, baseRow)
}

// parseErrorAt builds an error describing the first ERROR or MISSING node
// under root, reported against the source line baseRow is offset from.
func parseErrorAt(context string, root *sitter.Node, src []byte, baseRow uint) error {
	errNode := findFirstError(root)
	if errNode == nil {
		return fmt.Errorf("parse error in %s block", context)
	}

	pos := errNode.StartPosition()
	line := baseRow + pos.Row + 1

	snippet := string(src[errNode.StartByte():errNode.EndByte()])
	if len(snippet) > 50 {
		snippet = snippet[:50] + "..."
	}

	return fmt.Errorf("parse error in %s at line %d, col %d near %q", context, line, pos.Column+1, snippet)
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
	if f.lastNL {
		return
	}

	if f.dropWhitespaceOnlyLine() {
		return
	}

	f.write("\n")
}

// dropWhitespaceOnlyLine removes a partial trailing line holding nothing but
// whitespace, reporting whether it removed one.
//
// Content emitted verbatim — a <cfxml> or <cfscript> body, say — ends with the
// indentation that preceded its closing tag. Starting a new line after it
// committed that indentation as a whitespace-only line, and the next format did
// the same to the line it had just created. Files grew by one line every time
// they were formatted, without limit.
func (f *Formatter) dropWhitespaceOnlyLine() bool {
	b := f.out.Bytes()

	cut := len(b)
	for cut > 0 && b[cut-1] != '\n' {
		if !isWS(b[cut-1]) {
			return false
		}

		cut--
	}

	if cut == len(b) {
		return false
	}

	f.out.Truncate(cut)
	f.lineLen = 0
	f.lastNL = true
	f.atBOL = true

	return true
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
	// comment marks an entry that is a CFML comment sitting among the
	// attributes rather than an attribute of its own. It carries its whole
	// text in name and is re-emitted verbatim.
	comment bool
}

// collectAttrs gathers all cf_attribute children from a tag node,
// searching through cf_start_tag and cf_tag_attributes.
func (f *Formatter) collectAttrs(tag *sitter.Node) []cfAttr {
	var attrs []cfAttr

	f.walkAttrs(tag, &attrs, headerEnd(f.src, int(tag.StartByte())))

	return attrs
}

func (f *Formatter) walkAttrs(n *sitter.Node, attrs *[]cfAttr, hdrEnd int) {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_start_tag_with_selfclose", "cf_tag_attributes":
			f.walkAttrs(c, attrs, hdrEnd)
		case "cf_comment":
			// A comment among the attributes. Commenting an attribute out is
			// how a setting gets parked without losing it, so dropping the
			// comment throws that away silently. Carry it through in place.
			if int(c.StartByte()) < hdrEnd {
				*attrs = append(*attrs, cfAttr{name: f.text(c), comment: true})
			}
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
//
// The quote characters *inside* the value are never rewritten. Swapping them
// corrupts the value whenever the delimiter it would swap to already appears
// within: `to="#listLen(temp,"'")#"` became `to="#listLen(temp,”')#"`, an
// unbalanced literal the grammar then rejects. Re-quoting is therefore only
// applied when the value carries no double quote of its own; otherwise the
// original delimiters are kept as-is.
func (f *Formatter) normaliseAttrValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		q := v[0]
		if (q == '"' || q == '\'') && v[len(v)-1] == q {
			if q == '"' {
				return v
			}

			inner := v[1 : len(v)-1]
			if strings.Contains(inner, `"`) {
				// Converting would need the inner quotes rewritten. Leave the
				// single quotes in place rather than change the value.
				return v
			}

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
	var inline strings.Builder
	inline.WriteString(" ")

	for i, a := range attrs {
		if i > 0 {
			inline.WriteString(" ")
		}

		if a.value == "" {
			inline.WriteString(a.name)
		} else {
			fmt.Fprintf(&inline, "%s=%s", a.name, a.value)
		}
	}

	// Should we expand?
	oneLiner := "<" + tagName + inline.String()
	expand := len(attrs) > f.opts.AttrBreakThreshold ||
		f.lineLen+len(oneLiner) > f.opts.LineWidth ||
		hasMultiLineComment(attrs)

	if !expand {
		return inline.String()
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

	case "doctype", "xml_decl":
		f.formatDoctype(n)

	case "style_element", "script_element":
		f.formatRawTextElement(n)

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

// formatterState is the mutable output state a trial render has to leave
// exactly as it found it.
type formatterState struct {
	outLen           int
	level            int
	atBOL            bool
	lastNL           bool
	lineLen          int
	lastTagMultiLine bool
	pendingComma     bool
	parseErr         error
	pendingComments  []*sitter.Node
}

func (f *Formatter) saveState() formatterState {
	return formatterState{
		outLen:           f.out.Len(),
		level:            f.level,
		atBOL:            f.atBOL,
		lastNL:           f.lastNL,
		lineLen:          f.lineLen,
		lastTagMultiLine: f.lastTagMultiLine,
		pendingComma:     f.pendingComma,
		parseErr:         f.parseErr,
		pendingComments:  slices.Clone(f.pendingBlockComments),
	}
}

func (f *Formatter) restoreState(s formatterState) {
	f.out.Truncate(s.outLen)
	f.level = s.level
	f.atBOL = s.atBOL
	f.lastNL = s.lastNL
	f.lineLen = s.lineLen
	f.lastTagMultiLine = s.lastTagMultiLine
	f.pendingComma = s.pendingComma
	f.parseErr = s.parseErr
	f.pendingBlockComments = s.pendingComments
}

// rendersOnOneLine reports whether what emit writes lands on a single line
// inside LineWidth, by running it, measuring the result and discarding it.
//
// The question has to be asked of the text that will actually be emitted, not
// of the source. The formatter rewrites as it goes — a tag written ">" comes out
// " />", attribute values gain quotes, spacing is normalised — so measuring the
// source ran a couple of characters per tag short of the truth. Once a file had
// been formatted the measurement changed, borderline bodies crossed the limit
// and took the other branch, and formatting alternated between two layouts.
// Approximating the difference textually does not help: it only moves the
// boundary, so a different set of borderline cases flips instead.
//
// Note that the width alone cannot answer this. The emitters soft-wrap, so a
// body that is too long comes back inside LineWidth by being split across
// lines — the tell is the split itself, not the width. Both are checked.
func (f *Formatter) rendersOnOneLine(emit func()) bool {
	saved := f.saveState()
	col := f.lineLen

	emit()

	rendered := f.out.Bytes()[saved.outLen:]

	// The emitters open their own line; that first newline is expected, and
	// resets the column. Any further one means the content was wrapped.
	if len(rendered) > 0 && rendered[0] == '\n' {
		rendered = rendered[1:]
		col = 0
	}

	fits := !bytes.ContainsRune(rendered, '\n') && col+len(rendered) <= f.opts.LineWidth

	f.restoreState(saved)

	return fits
}

// isWhitespaceNode reports whether n contributes nothing but whitespace.
//
// Whitespace between two siblings is not itself a sibling. Letting one update
// the grouping state made trailing spaces after a comment look like a
// non-comment neighbour, so the tag after it gained a blank line — which the
// same format then stripped, flipping the answer on the next pass. Files
// oscillated between the two forms forever.
func (f *Formatter) isWhitespaceNode(n *sitter.Node) bool {
	return strings.TrimSpace(f.text(n)) == ""
}

func (f *Formatter) formatChildren(n *sitter.Node) {
	prevTagKind := ""
	prevWasComment := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		kind := f.nodeTagKind(c)

		if kind != "" && prevTagKind != "" && !prevWasComment && !f.nodeIsComment(c) &&
			(kind != prevTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
			f.write("\n")
		}

		f.formatNode(c)

		if f.isWhitespaceNode(c) {
			continue
		}

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

// nodeIsComment returns true if the node is a comment. Comments are transparent
// to blank-line grouping in both directions: they neither end the run they sit
// in nor start a new one.
//
// The symmetry is what makes the output stable. Whether the grammar hands a
// comment to a tag's body or to the siblings after it depends on stray trailing
// whitespace, and only the body is grouped — so a comment gained a blank line
// before it, the format removed the whitespace, the re-parse moved the comment
// out of the body, and the blank line went away again.
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
//
// Whitespace-only text counts as nothing. Trailing spaces after a tag's ">"
// become a text node, so `<cffunction ...>   ` followed by a <cfargument> used
// to answer false and gain a blank line — which the format then removed,
// leaving the file to answer true and lose the blank line on the next pass.
// Formatting oscillated between the two forever.
func (f *Formatter) firstBodyChildIsArg(n *sitter.Node) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)

		kind := c.Kind()
		if kind == "<cf" || kind == "</cf" || kind == ">" || kind == "cf_tag_name" ||
			kind == "cf_tag_attributes" || kind == "cf_end_tag" ||
			kind == "cf_attribute" || kind == "cf_start_tag" {
			continue
		}

		if strings.TrimSpace(f.text(c)) == "" {
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
// hasCFBodyContent reports whether a cf_tag has any body child with actual
// content, ignoring the start/end tags emitted around the body loop.
func (f *Formatter) hasCFBodyContent(n *sitter.Node) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_end_tag", "implicit_cf_end_tag":
			continue
		default:
			if strings.TrimSpace(f.text(c)) != "" {
				return true
			}
		}
	}

	return false
}

// hasRealCFEndTag reports whether the source actually closed this cf_tag. A
// tag written without one (`<cfmodule ...>`, `<cffeed ...>`, `<cfadmin ...>`)
// has either no end-tag child at all or only the grammar's synthetic
// implicit_cf_end_tag marker — inventing a `</name>` for it re-parents every
// following sibling into the tag's body.
func (f *Formatter) hasRealCFEndTag(n *sitter.Node) bool {
	for i := uint(0); i < n.ChildCount(); i++ {
		if n.Child(i).Kind() == "cf_end_tag" {
			return true
		}
	}

	return false
}

// isCFTryBranch reports whether n is a <cfcatch> or <cffinally> block, the// isCFTryBranch reports whether n is a <cfcatch> or <cffinally> block, the
// tags that are indented with their enclosing <cftry> rather than inside it.
func (f *Formatter) isCFTryBranch(n *sitter.Node) bool {
	if n.Kind() != "cf_tag" {
		return false
	}

	switch strings.ToLower(f.tagName(n)) {
	case "cfcatch", "cffinally":
		return true
	}

	return false
}

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

	closed := f.hasRealCFEndTag(n)
	isBlock := closed // only a genuinely closed tag brackets an indented body

	// An empty block (e.g. <cfcatch type="any"></cfcatch>) otherwise emits the
	// padding blank line from both ends, leaving two blank lines around nothing.
	hasBody := f.hasCFBodyContent(n)

	if isBlock {
		f.level++

		if hasBody {
			f.write("\n")
		}
	}

	prevCFTagKind := ""
	prevCFTagWasComment := false

	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "cf_start_tag", "cf_end_tag", "implicit_cf_end_tag":
			// Start and end tags are emitted around this loop. An unclosed tag
			// yields implicit_cf_end_tag, a synthetic marker whose text is the
			// trailing whitespace; emitting it added a stray blank line that
			// disappeared once the closing tag existed, making the output
			// differ between the first and second format.
		default:
			tagKind := f.nodeTagKind(c)
			if tagKind != "" && prevCFTagKind != "" && !prevCFTagWasComment && !f.nodeIsComment(c) &&
				(tagKind != prevCFTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
				f.write("\n")
			}

			// <cfcatch>/<cffinally> line up with their enclosing <cftry>
			// rather than sitting inside its body, matching the conventional
			// CFML style (and how <cfelse> is emitted inside <cfif>).
			dedent := name == "cftry" && f.level > 0 && f.isCFTryBranch(c)
			if dedent {
				f.level--
			}

			f.formatNode(c)

			if dedent {
				f.level++
			}

			if f.isWhitespaceNode(c) {
				continue
			}

			if tagKind != "" && !f.nodeIsComment(c) {
				prevCFTagKind = tagKind
			}

			prevCFTagWasComment = f.nodeIsComment(c)
		}
	}

	if isBlock {
		f.level--
	}

	if hasBody {
		f.write("\n")
	}

	// Only close what the source closed. Tags legal without a body — cfmodule,
	// cfhttp, cfinvoke, cffeed, cfadmin — were given a synthesised closing tag,
	// which swallowed everything after them into the tag's body.
	if closed {
		f.nl()
		f.writeIndent()
		f.write("</" + name + ">")
	}

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
	inline := func() { f.emitBodyInline(bodyNodes) }
	if f.allSingleLine(bodyNodes) && !f.hasBlockChild(bodyNodes) && f.rendersOnOneLine(inline) {
		inline()
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
			if tagKind != "" && prevTagKind != "" && !prevWasComment && !f.nodeIsComment(c) &&
				(tagKind != prevTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
				f.write("\n")
			}

			if kind == "comment" { //nolint:gocritic // ifElseChain: intentional for readability
				f.formatComment(c)
			} else {
				f.formatNode(c)
			}

			if f.isWhitespaceNode(c) {
				i++

				continue
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

// emitBodyInline writes a block tag's body without blank-line grouping: CF tags
// go through formatNode, and runs of text stay inline.
func (f *Formatter) emitBodyInline(bodyNodes []*sitter.Node) {
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
}

// formatCFSavecontent emits the opening tag with proper indentation
// but preserves everything between (and including) the closing tag verbatim.
func (f *Formatter) formatCFSavecontent(n *sitter.Node) {
	name := f.tagName(n)
	attrs := f.collectAttrs(n)

	f.nl()
	f.writeIndent()
	f.write("<" + name + f.renderAttrs(name, attrs) + ">")

	// Emit everything between the tags verbatim. Collecting only the known
	// cf_savecontent_body* node kinds missed content the grammar places
	// outside them: a body consisting purely of comments parses as an *empty*
	// cf_savecontent_body with the comments following it as siblings, so the
	// entire body was dropped. Slicing the source spans whatever shape the
	// grammar chose.
	f.write(f.savecontentBody(n, name))

	// Emit closing tag
	f.write("</" + name + ">")
	f.write("\n")
}

// savecontentBody returns the raw source between a savecontent tag's opening
// ">" and its closing tag.
func (f *Formatter) savecontentBody(n *sitter.Node, name string) string {
	start := headerEnd(f.src, int(n.StartByte()))

	end := int(n.EndByte())
	if end > len(f.src) {
		end = len(f.src)
	}

	if closeAt := lastIndexFold(f.src[:end], "</"+name); closeAt >= start {
		end = closeAt
	}

	if start < 0 || start > end {
		return ""
	}

	return string(f.src[start:end])
}

// lastIndexFold returns the offset of the final case-insensitive occurrence of
// lit in src, or -1.
func lastIndexFold(src []byte, lit string) int {
	for i := len(src) - len(lit); i >= 0; i-- {
		if hasBytesAtFold(src, i, lit) {
			return i
		}
	}

	return -1
}

func hasBytesAtFold(src []byte, pos int, lit string) bool {
	if pos < 0 || pos+len(lit) > len(src) {
		return false
	}

	for k := range len(lit) {
		if toLower(src[pos+k]) != toLower(lit[k]) {
			return false
		}
	}

	return true
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

	// Break at logical operators, indenting by paren depth.
	baseIndent := f.indented() + f.opts.indent(1)

	// A "//" comment runs to the end of its line, so folding the following
	// line up onto it turns that code into part of the comment. Collapsing a
	// condition like
	//
	//	if ( a          // first
	//	    or b )      // second
	//
	// onto one line leaves everything after the first "//" commented out, and
	// because joining lines only removes whitespace the change slips past a
	// character-level comparison. Keep the author's line structure instead and
	// re-indent it.
	if anyHasLineComment(parts) {
		return strings.Join(parts, "\n"+baseIndent)
	}

	single := strings.Join(parts, " ")

	// Check if it fits on one line.
	tagPrefix := f.lineLen
	if tagPrefix+len(single) <= f.opts.LineWidth {
		return single
	}

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

// anyHasLineComment reports whether any part carries a "//" comment, which
// makes the remainder of that part's line uncollapsible.
func anyHasLineComment(parts []string) bool {
	for _, p := range parts {
		if hasLineComment(p) {
			return true
		}
	}

	return false
}

// hasLineComment reports whether s opens a "//" comment outside a string
// literal. Unlike the document-wide scan the guard performs, s here is a single
// expression, so its quotes are balanced and tracking them is reliable.
func hasLineComment(s string) bool {
	var quote byte

	for i := range len(s) {
		c := s[i]

		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '/' && i+1 < len(s) && s[i+1] == '/':
			return true
		}
	}

	return false
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
		if tagKind != "" && prevTagKind2 != "" && !prevWasComment2 && !f.nodeIsComment(c) &&
			(tagKind != prevTagKind2 || f.isBlockTagKind(c) || f.lastTagMultiLine) {
			f.write("\n")
		}

		f.formatNode(c)

		if f.isWhitespaceNode(c) {
			continue
		}

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
		if tagKind != "" && prevAltTagKind != "" && !prevAltWasComment && !f.nodeIsComment(c) &&
			(tagKind != prevAltTagKind || f.isBlockTagKind(c) || f.lastTagMultiLine) {
			f.write("\n")
		}

		f.formatNode(c)

		if f.isWhitespaceNode(c) {
			continue
		}

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

// formatDoctype emits a <!DOCTYPE ...> or <?xml ...?> declaration verbatim on its
// own line.
//
// The grammar models the doctype body (everything between "DOCTYPE" and ">") as an
// unnamed pattern token that is not exposed as a child node, so the generic
// child-walking path in formatNode would silently reconstruct the node as
// "<!DOCTYPE>" and discard the body. An xml_decl fails there differently but just as
// badly: its parts *are* children ("<?", "xml", tag_attributes..., "?>"), and the
// generic path concatenates them with no separator, turning
// <?xml version="1.0" encoding="utf-8"?> into <?xmlversion="1.0"encoding="utf-8"?>.
// The whitespaceOnly guard cannot catch that — only whitespace was removed — but it
// destroys the declaration and leaves the file unparseable.
//
// Emitting the node's own source text avoids both, and is correct regardless:
// neither declaration has internal structure worth reformatting, and an XML
// declaration is specified down to its spacing.
func (f *Formatter) formatDoctype(n *sitter.Node) {
	raw := strings.TrimSpace(f.text(n))
	if raw == "" {
		return
	}

	f.writeIndent()
	f.write(raw)
	f.write("\n")
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
			sb.WriteString(baseIndent)
			sb.WriteString(f.opts.indent(1))
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}

	sb.WriteString(baseIndent)
	sb.WriteString("--->")

	return sb.String()
}
