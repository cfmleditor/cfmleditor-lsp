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
			sp := newScriptParser(r.Text, string(fileURI), r.StartLine)
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
			sp := newScriptParser(r.Text, string(fileURI), r.StartLine)
			sp.parse()
			refs = append(refs, sp.refs...)
		} else {
			tp := newTagParser(r.Text, string(fileURI))
			tp.parse()
			for i := range tp.refs {
				tp.refs[i].Line += uint32(r.StartLine)
			}
			refs = append(refs, tp.refs...)
		}
	}

	sort.Slice(refs, func(i, j int) bool { return refs[i].Line < refs[j].Line })
	return refs
}

// ParseVars extracts variable declarations from content.
func ParseVars(content string) []VarDef {
	regions := ClassifyRegions(content)
	scopes := FindFuncScopes(content)
	var defs []VarDef
	type key struct {
		name string
		line uint32
	}
	seen := make(map[key]bool)

	for _, r := range regions {
		var regionVars []VarDef
		if r.Kind == RegionScript {
			sp := newScriptParser(r.Text, "", r.StartLine)
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
		switch v.Scope {
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
		switch v.Scope {
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
	regions := ClassifyRegions(content)
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
	var scopes []FuncScope
	idx := buildLineIdx(src)
	pos := 0

	for {
		i := indexCFTag(src[pos:], "cffunction")
		if i < 0 {
			break
		}
		i += pos
		startLine := baseLine + lineAtOffset(idx, i)
		end := i + 200; if end > len(src) { end = len(src) }
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
func buildLineIdx(src string) []int {
	n := 1
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			n++
		}
	}
	idx := make([]int, 1, n)
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

// lineAtOffset returns the 0-based line number for a byte offset.
func lineAtOffset(idx []int, offset int) int {
	lo, hi := 0, len(idx)
	for lo < hi {
		mid := (lo + hi) / 2
		if idx[mid] <= offset {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}

// ClassifyRegions segments CFC content into script and tag regions.
func ClassifyRegions(content string) []Region {
	if isScriptFile(content) {
		return []Region{{Kind: RegionScript, StartLine: 0, Text: content}}
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
		if lower == "component" || lower == "interface" || lower == "property" {
			return true
		}
	}
	// If no CF tags found at all, treat as script
	return !containsCFTag(content)
}

// containsCFTag checks if content has any <cf (case-insensitive) without allocating.
func containsCFTag(s string) bool {
	for i := 0; i < len(s)-2; i++ {
		if s[i] == '<' && toLowerByte(s[i+1]) == 'c' && toLowerByte(s[i+2]) == 'f' {
			return true
		}
	}
	return false
}

// splitCFScriptBlocks splits tag-based content into tag and script regions.
func splitCFScriptBlocks(content string) []Region {
	idx := buildLineIdx(content)
	var regions []Region
	pos := 0

	for {
		openIdx := indexCFTag(content[pos:], "cfscript>")
		if openIdx < 0 {
			break
		}
		openIdx += pos
		if openIdx > pos {
			text := content[pos:openIdx]
			if strings.TrimSpace(text) != "" {
				regions = append(regions, Region{Kind: RegionTag, StartLine: lineAtOffset(idx, pos), Text: text})
			}
		}
		bodyStart := openIdx + 10 // len("<cfscript>")
		closeIdx := indexCFTag(content[bodyStart:], "/cfscript>")
		if closeIdx < 0 {
			text := content[bodyStart:]
			if strings.TrimSpace(text) != "" {
				regions = append(regions, Region{Kind: RegionScript, StartLine: lineAtOffset(idx, bodyStart), Text: text})
			}
			pos = len(content)
			break
		}
		closeIdx += bodyStart
		text := content[bodyStart:closeIdx]
		if strings.TrimSpace(text) != "" {
			regions = append(regions, Region{Kind: RegionScript, StartLine: lineAtOffset(idx, bodyStart), Text: text})
		}
		pos = closeIdx + 11 // len("</cfscript>")
	}
	if pos < len(content) {
		text := content[pos:]
		if strings.TrimSpace(text) != "" {
			regions = append(regions, Region{Kind: RegionTag, StartLine: lineAtOffset(idx, pos), Text: text})
		}
	}
	return regions
}

// indexCFTag finds "<" followed by suffix (case-insensitive) in s.
// Returns the index of '<' or -1.
func indexCFTag(s, suffix string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1+len(suffix) <= len(s) && strings.EqualFold(s[i+1:i+1+len(suffix)], suffix) {
			return i
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

func uriFromString(s string) uri.URI {
	return uri.URI(s)
}
