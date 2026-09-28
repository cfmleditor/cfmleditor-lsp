package parser

import "strings"

// tagFrame is one open tag in tagStructure's stack.
type tagFrame struct {
	name string
	// branches are the lines of a `<cfif>`'s `<cfelse>` and `<cfelseif>`.
	branches []int
	// line is the line of the tag's `<`.
	line int
}

// maxTagSearch bounds how far down the stack a closing tag looks for its
// opener. Unclosed tags pile up — a `<p>` or `<li>` whose close is implied —
// and a close with no opener at all would otherwise search all of them, once
// per close.
const maxTagSearch = 64

// tagStructure appends the spans of the markup in content to spans: every
// comment, every element or CF tag spanning lines, every opening tag whose
// attributes wrap, and every `<cfelse>` and `<cfelseif>` branch. script lists
// the byte ranges of the script regions, which scriptStructure has read and
// this walk steps over.
//
// Tags pair on a stack. A closing tag pops to the nearest opener of the same
// name, case-insensitively, and whatever is left open above it is discarded:
// that is how `<cfset>`, `<br>` and an unclosed `<p>` fall out without a list
// of which tags have bodies. isBodilessTag is only there to keep the stack
// short.
func tagStructure(content string, script [][2]int, idx []int32, spans []Span) []Span {
	var stack []tagFrame

	for pos := 0; pos < len(content); {
		if len(script) > 0 && pos >= script[0][0] {
			pos = max(pos, script[0][1])
			script = script[1:]

			continue
		}

		k := strings.IndexByte(content[pos:], '<')
		if k < 0 {
			break
		}

		lt := pos + k
		if len(script) > 0 && lt >= script[0][0] {
			pos = lt

			continue
		}

		rest := content[lt:]

		switch {
		case strings.HasPrefix(rest, "<!--"):
			end := len(content)

			if strings.HasPrefix(rest, "<!---") {
				end = skipCFMLComment(content, lt)
			} else if c := strings.Index(rest[4:], "-->"); c >= 0 {
				end = lt + 4 + c + 3
			}

			if start, last := lineAtOffset(idx, lt), lineAtOffset(idx, end-1); last > start {
				spans = append(spans, Span{Start: start, End: last, Kind: SpanComment})
			}

			pos = end
		case len(rest) > 1 && rest[1] == '/':
			name := tagName(rest[2:])
			if name == "" {
				pos = lt + 2

				continue
			}

			pos = tagEnd(content, lt+2+len(name))
			stack = closeTag(stack, name, lineAtOffset(idx, lt), lineAtOffset(idx, lastContent(content, lt)), &spans)
		default:
			name := tagName(rest[1:])
			if name == "" {
				pos = lt + 1

				continue
			}

			gt := tagEnd(content, lt+1+len(name))
			pos = gt
			line := lineAtOffset(idx, lt)

			// An opening tag whose attributes wrap folds by itself, to the
			// line before its `>`, whether or not it has a body: `<button`
			// with an attribute a line, a twenty-line `<cfset x = {`.
			if last := lineAtOffset(idx, gt-1); last > line {
				spans = append(spans, Span{Start: line, End: last, Kind: SpanBlock})
			}

			if selfClosing(content, lt, gt) || isBodilessTag(name) {
				continue
			}

			if isBranchTag(name) {
				branchCFIf(stack, line)

				continue
			}

			if raw := rawTextEnd(content, gt, name); raw >= 0 {
				// <script> and <style> hold JavaScript and CSS, where
				// `a<b` is not a tag: step to the closing tag, which the
				// next turn of the walk pairs.
				pos = raw
			}

			stack = append(stack, tagFrame{name: name, line: line})
		}
	}

	return spans
}

// tagName returns the name at the start of s: a letter or underscore, then
// letters, digits, `_`, `-`, `.` and `:` (a cfimport prefix, `cf_` custom
// tags). Empty when s does not start a tag — `a < b`, `<=`, `<#`.
func tagName(s string) string {
	if s == "" || !isTagNameStart(s[0]) {
		return ""
	}

	i := 1
	for i < len(s) && isTagNamePart(s[i]) {
		i++
	}

	return s[:i]
}

func isTagNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

func isTagNamePart(c byte) bool {
	return isTagNameStart(c) || c >= '0' && c <= '9' || c == '-' || c == '.' || c == ':'
}

// tagEnd returns the offset just past the `>` ending the tag whose attributes
// begin at from, stepping over quoted values; the end of content when the
// tag is never closed. A `<` starting a tag before that `>` ends the tag where
// it stands, so the walk reads what it opens: `<input <cfif a>checked</cfif>>`
// puts a CF tag inside an HTML one, and swallowing it left the `</cfif>`
// unpaired.
func tagEnd(content string, from int) int {
	gt := tagEndIndex(content[from:])
	if gt < 0 {
		gt = len(content) - from - 1
	}

	if lt := strings.IndexByte(content[from:from+gt], '<'); lt >= 0 && startsTag(content[from+lt+1:]) {
		return from + lt
	}

	return from + gt + 1
}

// startsTag reports whether s, the text after a `<`, begins an opening or
// closing tag.
func startsTag(s string) bool {
	if s != "" && s[0] == '/' {
		s = s[1:]
	}

	return s != "" && isTagNameStart(s[0])
}

// lastContent returns the offset of the last non-space byte before lt, or lt
// when there is none.
func lastContent(content string, lt int) int {
	for i := lt - 1; i >= 0; i-- {
		switch content[i] {
		case ' ', '\t', '\r', '\n':
		default:
			return i
		}
	}

	return lt
}

// selfClosing reports whether the tag from lt to just past its `>` at gt ends
// in `/>`.
func selfClosing(content string, lt, gt int) bool {
	return gt-2 > lt && content[gt-1] == '>' && content[gt-2] == '/'
}

// closeTag pops stack to the nearest open tag named name, folding it to the
// closing tag on line. With no opener within maxTagSearch, the close is
// ignored and the stack left alone.
//
// A `<cfif>`'s branches end at contentLine, the line of the last thing before
// the `</cfif>`. A branch has no closing tag of its own, so, like a switch
// case, it keeps its last line on screen; and each runs to the `</cfif>`,
// since a `<cfelseif>` holds the branches after it, as tree-sitter read it.
func closeTag(stack []tagFrame, name string, line, contentLine int, spans *[]Span) []tagFrame {
	for i := len(stack) - 1; i >= 0 && i >= len(stack)-maxTagSearch; i-- {
		f := &stack[i]
		if !strings.EqualFold(f.name, name) {
			continue
		}

		if line > f.line {
			*spans = append(*spans, Span{Start: f.line, End: line, Kind: SpanBlock})
		}

		for _, b := range f.branches {
			if contentLine > b {
				*spans = append(*spans, Span{Start: b, End: contentLine, Kind: SpanCase})
			}
		}

		return stack[:i]
	}

	return stack
}

// branchCFIf records a `<cfelse>` or `<cfelseif>` branch, on line, of the
// nearest open `<cfif>`, for closeTag to fold. The `<cfif>`'s own first
// branch is not folded apart from the whole `<cfif>`.
func branchCFIf(stack []tagFrame, line int) {
	for i := len(stack) - 1; i >= 0 && i >= len(stack)-maxTagSearch; i-- {
		if f := &stack[i]; strings.EqualFold(f.name, "cfif") {
			f.branches = append(f.branches, line)

			return
		}
	}
}

// rawTextEnd returns, for a <script> or <style> whose open tag ends at from,
// the offset of its closing tag, or the end of content; -1 for any other tag,
// and for a block holding a CF tag, which ClassifyRegions does not treat as
// raw either: its `<cfif>`s and `<cfoutput>`s are walked like any others.
func rawTextEnd(content string, from int, name string) int {
	var closing string

	switch {
	case strings.EqualFold(name, "script"):
		closing = "</script"
	case strings.EqualFold(name, "style"):
		closing = "</style"
	default:
		return -1
	}

	end := len(content)
	if i := indexFoldASCII(content[from:], closing); i >= 0 {
		end = from + i
	}

	if indexFoldASCII(content[from:end], "<cf") >= 0 || indexFoldASCII(content[from:end], "</cf") >= 0 {
		return -1
	}

	return end
}

// indexFoldASCII is strings.Index, ASCII case-insensitive, for a needle
// starting with `<`.
func indexFoldASCII(s, needle string) int {
	for i := 0; i+len(needle) <= len(s); i++ {
		j := strings.IndexByte(s[i:], '<')
		if j < 0 {
			return -1
		}

		i += j
		if i+len(needle) <= len(s) && strings.EqualFold(s[i:i+len(needle)], needle) {
			return i
		}
	}

	return -1
}

// isBranchTag reports whether name starts a branch of a `<cfif>`.
func isBranchTag(name string) bool {
	return strings.EqualFold(name, "cfelse") || strings.EqualFold(name, "cfelseif")
}

// isBodilessTag reports whether name never has a closing tag: an HTML void
// element, or a CF tag that takes no body. Pushing one would only lengthen
// the stack until something popped past it.
func isBodilessTag(name string) bool {
	if len(name) > len("cfprocessingdirective") {
		return false
	}

	var buf foldScratch

	switch string(buf.lowerFold(name)) {
	case "br", "img", "input", "meta", "link", "hr", "area", "base", "col", "embed",
		"param", "source", "track", "wbr",
		"cfset", "cfreturn", "cfparam", "cfargument", "cfproperty", "cfinclude",
		"cfabort", "cfbreak", "cfcontinue", "cflocation", "cfqueryparam", "cfdump",
		"cfthrow", "cfrethrow", "cfheader", "cfcontent", "cfcookie", "cfinvokeargument",
		"cflog", "cfsetting", "cfprocparam", "cfprocresult", "cfexit", "cfflush",
		"cfimport", "cfprocessingdirective":
		return true
	default:
		return false
	}
}
