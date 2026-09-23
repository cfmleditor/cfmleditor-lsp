package parser

import (
	"sort"
	"strings"

	"go.lsp.dev/uri"
)

// ParseFunctionDefs extracts function definitions from CFC content.
func ParseFunctionDefs(fileURI uri.URI, content string) []FunctionDef {
	regions := ClassifyRegions(content)

	var defs []FunctionDef

	for _, r := range regions {
		if r.Kind == RegionScript {
			sp := newScriptParser(r.Text, string(fileURI), r.StartLine, nil).asCFScript()
			sp.parse()
			defs = append(defs, sp.funcs...)
		} else {
			tp := newTagParser(r.Text, string(fileURI))
			tp.parse()
			// Adjust lines by region start
			for i := range tp.funcs {
				tp.funcs[i].Line += uint32(r.StartLine)
			}

			defs = append(defs, tp.funcs...)
		}
	}

	return defs
}

// ParseComponentRefs extracts component references from source content.
func ParseComponentRefs(fileURI uri.URI, content string) []ComponentRef {
	regions := ClassifyRegions(content)

	var refs []ComponentRef

	for _, r := range regions {
		if r.Kind == RegionScript {
			sp := newScriptParser(r.Text, string(fileURI), r.StartLine, nil).asCFScript()
			sp.parse()
			refs = append(refs, sp.componentRefs...)
		} else {
			tp := newTagParser(r.Text, string(fileURI))
			tp.parse()

			for i := range tp.componentRefs {
				tp.componentRefs[i].Line += uint32(r.StartLine)
			}

			refs = append(refs, tp.componentRefs...)
		}
	}

	sort.Slice(refs, func(i, j int) bool { return refs[i].Line < refs[j].Line })

	return refs
}

// ParseVars extracts variable declarations from content.
func ParseVars(content string) []VarDef {
	regions := ClassifyRegions(content)
	scopes := findFuncScopesIn(regions)

	var defs []VarDef

	type key struct {
		name string
		line uint32
	}

	seen := make(map[key]bool)

	for _, r := range regions {
		var regionVars []VarDef

		if r.Kind == RegionScript {
			sp := newScriptParser(r.Text, "", r.StartLine, nil).asCFScript()
			sp.parse()
			regionVars = sp.vars
		} else {
			tp := newTagParser(r.Text, "")
			tp.parse()

			for i := range tp.vars {
				tp.vars[i].Line += uint32(r.StartLine)
			}

			regionVars = tp.vars
		}

		for _, v := range regionVars {
			k := key{strings.ToLower(v.Name), v.Line}
			if seen[k] {
				continue
			}

			seen[k] = true

			fs := findFuncScope(int(v.Line), scopes)
			// var/local outside a function → variables scope
			if fs.Start == -1 && v.Scope == ScopeLocal {
				v.Scope = ScopeVariables
			}

			v.FuncStart = fs.Start
			v.FuncEnd = fs.End
			defs = append(defs, v)
		}
	}

	return defs
}

// GlobalVars returns this.x and variables.x names declared outside any function.
func GlobalVars(content string) []string {
	scopes := FindFuncScopes(content)
	allVars := ParseVars(content)
	seen := make(map[string]bool)

	var names []string

	for _, v := range allVars {
		if v.FuncStart != -1 {
			continue
		}

		_ = scopes

		switch v.Scope { //nolint:exhaustive
		case ScopeVariables, ScopeThis:
			if !seen[v.Name] {
				seen[v.Name] = true

				names = append(names, v.Name)
			}
		}
	}

	return names
}

// VarsInFunc returns local/arguments variable names within the function spanning [funcStart, funcEnd].
func VarsInFunc(content string, funcStart, funcEnd int) []string {
	allVars := ParseVars(content)
	seen := make(map[string]bool)

	var names []string

	for _, v := range allVars {
		if v.FuncStart != funcStart || v.FuncEnd != funcEnd {
			continue
		}

		switch v.Scope { //nolint:exhaustive
		case ScopeLocal, ScopeArguments:
			if !seen[v.Name] {
				seen[v.Name] = true

				names = append(names, v.Name)
			}
		}
	}

	return names
}

// FindFuncScopes returns function line ranges in the content.
func FindFuncScopes(content string) []FuncScope {
	return findFuncScopesIn(ClassifyRegions(content))
}

// findFuncScopesIn is FindFuncScopes for a caller that has already classified
// the content. ClassifyRegions is not cheap — it builds a line index over the
// whole file — and ParseVars was paying for it twice, once itself and once
// inside FindFuncScopes, on every call.
func findFuncScopesIn(regions []Region) []FuncScope {
	var scopes []FuncScope

	for _, r := range regions {
		if r.Kind == RegionScript {
			scopes = append(scopes, findScriptFuncScopes(r.Text, r.StartLine)...)
		} else {
			scopes = append(scopes, findTagFuncScopes(r.Text, r.StartLine)...)
		}
	}

	return scopes
}

// findScriptFuncScopes finds function boundaries in script source.
func findScriptFuncScopes(src string, baseLine int) []FuncScope {
	var scopes []FuncScope

	sc := NewScanner(src)

	for {
		tok := sc.NextSkipComments()
		if tok.Kind == TokEOF {
			break
		}

		if tok.Kind != TokIdent {
			continue
		}

		isFuncKeyword := identEq(tok.Value, "function")
		if !isFuncKeyword {
			lower := strings.ToLower(tok.Value)
			if lower == "public" || lower == "private" || lower == "remote" || lower == "package" {
				// Look ahead for [type] function
				next := sc.PeekSkipComments()
				if next.Kind == TokIdent && identEq(next.Value, "function") {
					sc.NextSkipComments()

					isFuncKeyword = true
				} else if next.Kind == TokIdent {
					sc.NextSkipComments() // type

					next2 := sc.PeekSkipComments()
					if next2.Kind == TokIdent && identEq(next2.Value, "function") {
						sc.NextSkipComments()

						isFuncKeyword = true
					}
				}
			}
		}

		if !isFuncKeyword {
			continue
		}

		startLine := baseLine + tok.Line

		// Skip name
		nameTok := sc.NextSkipComments()
		if nameTok.Kind != TokIdent {
			continue
		}
		// Skip (args)
		lp := sc.NextSkipComments()
		if lp.Kind != TokLParen {
			continue
		}

		depth := 1
		for depth > 0 {
			t := sc.NextSkipComments()
			if t.Kind == TokEOF {
				break
			}

			if t.Kind == TokLParen {
				depth++
			}

			if t.Kind == TokRParen {
				depth--
			}
		}

		// Find body end
		brace := sc.PeekSkipComments()
		if brace.Kind == TokSemicolon {
			sc.NextSkipComments()

			scopes = append(scopes, FuncScope{Name: nameTok.Value, Start: startLine, End: baseLine + brace.Line})

			continue
		}

		if brace.Kind != TokLBrace {
			continue
		}

		sc.NextSkipComments()

		braceDepth := 1

		var lastTok Token

		for braceDepth > 0 {
			lastTok = sc.NextSkipComments()
			if lastTok.Kind == TokEOF {
				break
			}

			if lastTok.Kind == TokLBrace {
				braceDepth++
			}

			if lastTok.Kind == TokRBrace {
				braceDepth--
			}
		}

		endLine := baseLine + lastTok.Line
		scopes = append(scopes, FuncScope{Name: nameTok.Value, Start: startLine, End: endLine})
	}

	return scopes
}

// findTagFuncScopes finds <cffunction>...</cffunction> boundaries.
func findTagFuncScopes(src string, baseLine int) []FuncScope {
	return findTagFuncScopesIdx(src, baseLine, nil)
}

// findTagFuncScopesIdx is findTagFuncScopes for a caller holding a line index
// for exactly this src. ClassifyRegions builds one over the whole file on its
// way to the regions, and extractSignatures then asked for the scopes of that
// same whole file — so the index was built twice over one string, every parse.
// A nil idx means build one.
func findTagFuncScopesIdx(src string, baseLine int, idx []int32) []FuncScope {
	// A modest capacity rather than a counted one. Counting <cffunction first
	// sizes the slice exactly, but it is a second pass over the whole source and
	// it runs on script components too, where it scans everything to find
	// nothing — measurably, at +1.6% on script parsing for a slice that append
	// grows well enough on its own.
	scopes := make([]FuncScope, 0, 8)

	if idx == nil {
		idx = buildLineIdx(src)
	}

	pos := 0
	commentDepth := 0

	for {
		i := indexCFTag(src[pos:], "cffunction")
		if i < 0 {
			break
		}

		i += pos

		// Update comment depth from pos to i
		for j := pos; j < i; j++ {
			if j+4 < len(src) && src[j:j+5] == "<!---" {
				commentDepth++
				j += 4
			} else if j+3 < len(src) && src[j:j+4] == "--->" {
				commentDepth--
				if commentDepth < 0 {
					commentDepth = 0
				}

				j += 3
			}
		}

		// Skip if inside a CFML comment block
		if commentDepth > 0 {
			if closeIdx := strings.Index(src[i:], "--->"); closeIdx >= 0 {
				pos = i + closeIdx + 4
				commentDepth--
			} else {
				break
			}

			continue
		}

		startLine := baseLine + lineAtOffset(idx, i)

		end := min(i+200, len(src))

		funcName := getAttr(src[i:end], "name")
		funcAccess := getAttr(src[i:end], "access")
		funcReturn := getAttr(src[i:end], "returntype")

		closeIdx := indexCFTag(src[i+11:], "/cffunction")
		if closeIdx < 0 {
			scopes = append(scopes, FuncScope{Name: funcName, Access: funcAccess, ReturnType: funcReturn, Start: startLine, End: baseLine + len(idx) - 1})

			break
		}

		closeEnd := i + 11 + closeIdx
		gt := strings.IndexByte(src[closeEnd:], '>')

		if gt >= 0 {
			closeEnd += gt + 1
		}

		endLine := baseLine + lineAtOffset(idx, closeEnd)
		scopes = append(scopes, FuncScope{Name: funcName, Access: funcAccess, ReturnType: funcReturn, Start: startLine, End: endLine})
		pos = closeEnd
	}

	return scopes
}

// buildLineIdx returns byte offsets of each line start.
//
// int32 rather than int: this is one entry per line of every file parsed, built
// several times over, and it was the single largest allocation site in a tag
// parse. A CFML file large enough to overflow int32 is 2GB of source, which no
// engine would load and this parser holds entirely in memory anyway.
//
// Both passes go through the stdlib's byte scanners rather than a loop over
// every byte: strings.Count and strings.IndexByte are vectorised, and this runs
// once per parsed file over the whole source, which made it 3.9% of tag parsing
// in a profile. The count pass stays — sizing the slice up front is what keeps
// the fill pass from reallocating a dozen times on a large file.
func buildLineIdx(src string) []int32 {
	idx := make([]int32, 1, strings.Count(src, "\n")+1)

	for off := 0; off < len(src); {
		i := strings.IndexByte(src[off:], '\n')
		if i < 0 {
			break
		}

		off += i + 1

		idx = append(idx, int32(off))
	}

	return idx
}

// lineAtOffset returns the 0-based line number for a byte offset.
func lineAtOffset(idx []int32, offset int) int {
	lo, hi := 0, len(idx)
	for lo < hi {
		mid := (lo + hi) / 2
		if int(idx[mid]) <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	return lo - 1
}

// ClassifyRegions segments CFC content into script and tag regions.
func ClassifyRegions(content string) []Region {
	regions, _ := ClassifyRegionsIdx(content)

	return regions
}

// ClassifyRegionsIdx is ClassifyRegions plus the line index it built on the way,
// so a caller that also needs line numbers for this content does not build a
// second one. The index is nil for a script file, which never reaches the code
// that wants it.
func ClassifyRegionsIdx(content string) ([]Region, []int32) {
	if isScriptFile(content) {
		return []Region{{Kind: RegionScript, StartLine: 0, Text: content}}, nil
	}

	return splitCFScriptBlocks(content)
}

// isScriptFile checks whether the first non-whitespace, non-comment token
// indicates a script-based CFC, or whether the content has no CF tags.
func isScriptFile(content string) bool {
	sc := NewScanner(content)

	tok := sc.NextSkipComments()
	if tok.Kind == TokIdent {
		lower := strings.ToLower(tok.Value)
		if lower == "component" || lower == "interface" || lower == "property" || lower == "import" {
			return true
		}
	}

	// A literal <script> tag (HTML/JavaScript, not <cfscript>) is strong
	// evidence this is an HTML page with embedded JS rather than a pure
	// CFScript component — even one with zero CF tags of its own (e.g. a
	// plain HTML dialog template). Without this check, "no CF tags found at
	// all" below would misclassify the whole file as CFScript and feed raw
	// JavaScript into the CFML scanner.
	if hasScriptTag(content) {
		return false
	}

	// If no CF tags found at all, treat as script
	return !containsCFTag(content)
}

// hasScriptTag reports whether content contains a literal <script ...> or
// <script> tag (case-insensitive), as opposed to <cfscript>.
// hasScriptTag reports whether content holds an HTML <script> tag.
//
// Walked with strings.IndexByte and gated on one folded byte, for the reason
// indexCFTag is: almost no '<' in a page starts the tag being looked for, so
// EqualFold should not be reached for most of them. This runs over whole files
// during region classification.
func hasScriptTag(content string) bool {
	for off := 0; off < len(content); {
		i := strings.IndexByte(content[off:], '<')
		if i < 0 {
			return false
		}

		at := off + i
		off = at + 1

		if at+7 >= len(content) {
			return false
		}

		if lowerASCII(content[at+1]) != 's' {
			continue
		}

		if !strings.EqualFold(content[at+1:at+7], "script") {
			continue
		}

		switch content[at+7] {
		case '>', '/', ' ', '\t', '\n', '\r':
			return true
		}
	}

	return false
}

// containsCFTag checks if content has any <cf (case-insensitive) without allocating.
func containsCFTag(s string) bool {
	for off := 0; off+2 < len(s); {
		i := strings.IndexByte(s[off:], '<')
		if i < 0 {
			break
		}

		at := off + i
		off = at + 1

		if at+2 < len(s) && toLowerByte(s[at+1]) == 'c' && toLowerByte(s[at+2]) == 'f' {
			return true
		}
	}

	return false
}

// scriptSkipSpan is the byte range [start, end) of a literal <script>...
// </script> block (not <cfscript>) that contains no "<cf" tag of its own.
type scriptSkipSpan struct {
	start, end int
}

// findScriptSkipSpans scans content for <script>...</script> blocks (case-
// insensitive) that contain no "<cf" anywhere inside, returning their byte
// ranges in order. CFML comments are skipped so a <script> tag inside a
// <!--- ... ---> comment doesn't produce a spurious span. A <script> block
// that DOES contain a "<cf" tag is deliberately left out of the result, so
// real CFML mixed into it (a stray <cfoutput>, a nested <cfscript>) is still
// parsed normally rather than being treated as opaque.
func findScriptSkipSpans(content string) []scriptSkipSpan {
	var spans []scriptSkipSpan

	pos := 0

	for pos < len(content) {
		i := strings.IndexByte(content[pos:], '<')
		if i < 0 {
			break
		}

		i += pos

		if i+4 < len(content) && content[i:i+5] == "<!---" {
			depth := 1
			j := i + 5

			for j < len(content) && depth > 0 {
				switch {
				case j+4 < len(content) && content[j:j+5] == "<!---":
					depth++
					j += 5
				case j+3 < len(content) && content[j:j+4] == "--->":
					depth--
					j += 4
				default:
					j++
				}
			}

			pos = j

			continue
		}

		if i+7 >= len(content) || !strings.EqualFold(content[i+1:i+7], "script") {
			pos = i + 1

			continue
		}

		switch content[i+7] {
		case '>', '/', ' ', '\t', '\n', '\r':
			// tag boundary — proceed
		default:
			pos = i + 1

			continue
		}

		gt := strings.IndexByte(content[i:], '>')
		if gt < 0 {
			break
		}

		bodyStart := i + gt + 1

		closeIdx := indexCFTag(content[bodyStart:], "/script>")

		var bodyEnd, spanEnd int

		if closeIdx < 0 {
			bodyEnd = len(content)
			spanEnd = len(content)
		} else {
			bodyEnd = bodyStart + closeIdx
			spanEnd = bodyEnd + len("</script>")
		}

		if !containsCFTag(content[bodyStart:bodyEnd]) {
			spans = append(spans, scriptSkipSpan{start: i, end: spanEnd})
		}

		pos = spanEnd
	}

	return spans
}

// splitCFScriptBlocks splits tag-based content into tag, script, and skip
// regions. CFML comments (<!--- ... --->) are skipped so that <cfscript>
// blocks inside comments do not produce spurious script regions. Literal
// <script>...</script> blocks with no CFML inside (see findScriptSkipSpans)
// are emitted as RegionSkip so their JavaScript is never scanned as CFML.
func splitCFScriptBlocks(content string) ([]Region, []int32) {
	idx := buildLineIdx(content)
	skipSpans := findScriptSkipSpans(content)
	skipIdx := 0

	var regions []Region

	pos := 0

	for {
		// Find the next <cfscript> while skipping CFML comments.
		openIdx := -1
		scanPos := pos

		for scanPos < len(content) {
			i := strings.IndexByte(content[scanPos:], '<')
			if i < 0 {
				break
			}

			i += scanPos

			// Skip CFML comment (<!--- ... --->) with nesting support.
			if i+4 < len(content) && content[i:i+5] == "<!---" {
				depth := 1
				j := i + 5

				for j < len(content) && depth > 0 {
					switch {
					case j+4 < len(content) && content[j:j+5] == "<!---":
						depth++
						j += 5
					case j+3 < len(content) && content[j:j+4] == "--->":
						depth--
						j += 4
					default:
						j++
					}
				}

				scanPos = j

				continue
			}

			if i+10 <= len(content) && strings.EqualFold(content[i:i+10], "<cfscript>") {
				openIdx = i

				break
			}

			scanPos = i + 1
		}

		// Find the next script-skip span at or after pos.
		for skipIdx < len(skipSpans) && skipSpans[skipIdx].end <= pos {
			skipIdx++
		}

		var nextSkip *scriptSkipSpan
		if skipIdx < len(skipSpans) {
			nextSkip = &skipSpans[skipIdx]
		}

		// A skip span starting before the next <cfscript> (or with no more
		// <cfscript> left at all) is handled first.
		if nextSkip != nil && (openIdx < 0 || nextSkip.start < openIdx) {
			if nextSkip.start > pos {
				text := content[pos:nextSkip.start]
				if strings.TrimSpace(text) != "" {
					regions = append(regions, Region{Kind: RegionTag, StartLine: lineAtOffset(idx, pos), Text: text, Offset: pos})
				}
			}

			regions = append(regions, Region{Kind: RegionSkip, StartLine: lineAtOffset(idx, nextSkip.start), Text: content[nextSkip.start:nextSkip.end], Offset: nextSkip.start})

			pos = nextSkip.end
			skipIdx++

			continue
		}

		if openIdx < 0 {
			break
		}

		if openIdx > pos {
			text := content[pos:openIdx]
			if strings.TrimSpace(text) != "" {
				regions = append(regions, Region{Kind: RegionTag, StartLine: lineAtOffset(idx, pos), Text: text, Offset: pos})
			}
		}

		bodyStart := openIdx + 10 // len("<cfscript>")
		closeIdx := indexCFTag(content[bodyStart:], "/cfscript>")

		if closeIdx < 0 {
			text := content[bodyStart:]
			if strings.TrimSpace(text) != "" {
				regions = append(regions, Region{Kind: RegionScript, StartLine: lineAtOffset(idx, bodyStart), Text: text, Offset: bodyStart})
			}

			pos = len(content)

			break
		}

		closeIdx += bodyStart

		text := content[bodyStart:closeIdx]
		if strings.TrimSpace(text) != "" {
			regions = append(regions, Region{Kind: RegionScript, StartLine: lineAtOffset(idx, bodyStart), Text: text, Offset: bodyStart})
		}

		pos = closeIdx + 11 // len("</cfscript>")
	}

	if pos < len(content) {
		text := content[pos:]
		if strings.TrimSpace(text) != "" {
			regions = append(regions, Region{Kind: RegionTag, StartLine: lineAtOffset(idx, pos), Text: text, Offset: pos})
		}
	}

	return regions, idx
}

// indexCFTag finds "<" followed by suffix (case-insensitive) in s.
// Returns the index of '<' or -1.
//
// The walk between candidates is strings.IndexByte rather than a byte-at-a-time
// loop, and the suffix's first byte is folded by hand before EqualFold is called
// at all. Both matter because this is the search the whole tag scanner is built
// on — it was 6% of tag parsing in a profile — and because the ratio is brutal:
// a page is full of '<' and almost none of them start the tag being looked for,
// so the common case should be a vector scan and one byte compare, not a
// function call per angle bracket.
func indexCFTag(s, suffix string) int {
	if suffix == "" {
		return strings.IndexByte(s, '<')
	}

	first := lowerASCII(suffix[0])

	for off := 0; off < len(s); {
		i := strings.IndexByte(s[off:], '<')
		if i < 0 {
			break
		}

		at := off + i
		off = at + 1

		if at+1+len(suffix) > len(s) {
			break
		}

		if lowerASCII(s[at+1]) != first {
			continue
		}

		if strings.EqualFold(s[at+1:at+1+len(suffix)], suffix) {
			return at
		}
	}

	return -1
}

func findFuncScope(line int, scopes []FuncScope) FuncScope {
	for _, s := range scopes {
		if line >= s.Start && line <= s.End {
			return s
		}
	}

	return FuncScope{Start: -1, End: -1}
}

// FindFuncScopeAt returns the FuncScope containing the given line, or {-1,-1}.
func FindFuncScopeAt(line int, scopes []FuncScope) FuncScope {
	return findFuncScope(line, scopes)
}

// StripReceiverScope strips a receiver expression down to the text after its last
// top-level "." (e.g. "VARIABLES.donorObj" -> "donorObj"), matching how a ComponentRef
// is stored without its scope prefix. "Top-level" skips any "." inside a "[...]"
// subscript: for "linkMap[arguments.startSource]", the only "." is inside the brackets
// (separating "arguments" from "startSource", not a scope prefix on "linkMap"), so it
// must NOT be stripped there — a naive strings.LastIndexByte(s, '.') would cut the
// receiver down to the nonsensical fragment "startSource]", losing the "linkMap["
// prefix entirely. Returns s unchanged if it has no top-level ".".
func StripReceiverScope(s string) string {
	depth := 0
	last := -1

	for i, c := range s {
		switch c {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '.':
			if depth == 0 {
				last = i
			}
		}
	}

	if last < 0 {
		return s
	}

	return s[last+1:]
}

func uriFromString(s string) uri.URI {
	return uri.URI(s)
}
