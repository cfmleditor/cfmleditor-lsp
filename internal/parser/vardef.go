// Package parser extracts local variable declarations from CFML source with
// scope information based on CFML scoping rules.
package parser

import (
	"regexp"
	"strings"
)

// Scope represents the CFML variable scope.
type Scope int

// Scope values for variable declarations.
const (
	ScopeLocal     Scope = iota // var x or local.x — function-local
	ScopeArguments              // arguments.x — function arguments
	ScopeThis                   // this.x — instance/public property
	ScopeVariables              // variables.x or unscoped assignment — page/component scope
)

// VarDef represents a variable declaration in source.
type VarDef struct {
	Name      string
	Scope     Scope
	Line      uint32
	FuncStart int // -1 if file-scope
	FuncEnd   int // -1 if file-scope
}

// FuncScope represents a function's line range.
type FuncScope struct {
	Start int
	End   int
}

// var x = ... or local.x = ...
var localVarRe = regexp.MustCompile(`(?im)(?:var\s+(\w+)|local\.(\w+))\s*=`)

// arguments.x = ...
var argsRe = regexp.MustCompile(`(?im)arguments\.(\w+)\s*=`)

// this.x = ...
var thisRe = regexp.MustCompile(`(?im)this\.(\w+)\s*=`)

// variables.x = ...
var variablesRe = regexp.MustCompile(`(?im)variables\.(\w+)\s*=`)

// Plain: x = ... (at start of line, no scope prefix)
var plainAssignRe = regexp.MustCompile(`(?im)^[ \t]*(\w+)\s*=`)

// Tag variants
var tagLocalRe = regexp.MustCompile(`(?i)<cfset\s+(?:var\s+(\w+)|local\.(\w+))\s*=`)
var tagArgsRe = regexp.MustCompile(`(?i)<cfset\s+arguments\.(\w+)\s*=`)
var tagThisRe = regexp.MustCompile(`(?i)<cfset\s+this\.(\w+)\s*=`)
var tagVariablesRe = regexp.MustCompile(`(?i)<cfset\s+variables\.(\w+)\s*=`)
var tagPlainRe = regexp.MustCompile(`(?i)<cfset\s+(\w+)\s*=`)

// Script function: access? type? function name( ... ) {
var scriptFuncStartRe = regexp.MustCompile(`(?im)(?:(?:public|private|remote|package)[ \t]+)?(?:\w+[ \t]+)?function[ \t]+\w+[ \t]*\([^)]*\)[ \t]*\{`)

// Tag function boundaries
var tagFuncStartRe = regexp.MustCompile(`(?i)<cffunction\s`)
var tagFuncEndRe = regexp.MustCompile(`(?i)</cffunction\s*>`)

// ParseVars extracts variable declarations from content.
func ParseVars(content string) []VarDef {
	scopes := findFuncScopes(content)
	var defs []VarDef
	type key struct {
		name string
		line uint32
	}
	seen := make(map[key]bool)

	add := func(name string, scope Scope, line uint32, funcScopes []FuncScope) {
		k := key{strings.ToLower(name), line}
		if seen[k] {
			return
		}
		seen[k] = true
		fs := findScope(int(line), funcScopes)
		defs = append(defs, VarDef{
			Name:      name,
			Scope:     scope,
			Line:      line,
			FuncStart: fs.Start,
			FuncEnd:   fs.End,
		})
	}

	// local.x and var x
	for _, re := range []*regexp.Regexp{localVarRe, tagLocalRe} {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			var name string
			if m[2] >= 0 {
				name = content[m[2]:m[3]]
			} else if m[4] >= 0 {
				name = content[m[4]:m[5]]
			}
			if name != "" {
				line := uint32(strings.Count(content[:m[0]], "\n"))
				fs := findScope(int(line), scopes)
				s := ScopeLocal
				if fs.Start == -1 {
					s = ScopeVariables
				}
				add(name, s, line, scopes)
			}
		}
	}

	// arguments.x
	for _, re := range []*regexp.Regexp{argsRe, tagArgsRe} {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			add(name, ScopeArguments, uint32(strings.Count(content[:m[0]], "\n")), scopes)
		}
	}

	// this.x
	for _, re := range []*regexp.Regexp{thisRe, tagThisRe} {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			add(name, ScopeThis, uint32(strings.Count(content[:m[0]], "\n")), scopes)
		}
	}

	// variables.x
	for _, re := range []*regexp.Regexp{variablesRe, tagVariablesRe} {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			add(name, ScopeVariables, uint32(strings.Count(content[:m[0]], "\n")), scopes)
		}
	}

	// Plain x = ... (implicitly variables scope)
	for _, re := range []*regexp.Regexp{plainAssignRe, tagPlainRe} {
		for _, m := range re.FindAllStringSubmatchIndex(content, -1) {
			name := content[m[2]:m[3]]
			if isKeyword(name) {
				continue
			}
			add(name, ScopeVariables, uint32(strings.Count(content[:m[0]], "\n")), scopes)
		}
	}

	return defs
}

// FileLayout holds precomputed byte ranges for a file, allowing GlobalVars
// and VarsInFunc to skip recomputation.
type FileLayout struct {
	content        string
	globalSegments []string // non-function, non-comment code segments
	funcByteRanges [][2]int // [start, end) for each function
}

// NewFileLayout computes and caches the structural layout of a file.
func NewFileLayout(content string) *FileLayout {
	fl := &FileLayout{content: content}
	comments := findCommentSpans(content)
	fl.funcByteRanges = findFuncByteRangesWithComments(content, comments)
	fl.globalSegments = computeGlobalSegments(content, fl.funcByteRanges, comments)
	return fl
}

// VarsInFunc returns only local/arguments variable names within the function
// that spans [funcStart, funcEnd].
func VarsInFunc(content string, funcStart, funcEnd int) []string {
	start, end := lineOffsets(content, funcStart, funcEnd)
	if start < 0 {
		return nil
	}
	body := content[start:end]
	return varsInBody(body)
}

// GlobalVars returns this.x and variables.x names declared outside any function.
func GlobalVars(content string) []string {
	return globalVarsFromSegments(nonFuncSegments(content))
}

// GlobalVarsFromLayout returns global vars using a precomputed FileLayout.
func GlobalVarsFromLayout(fl *FileLayout) []string {
	return globalVarsFromSegments(fl.globalSegments)
}

// VarsInFuncFromLayout returns function-local vars using a precomputed FileLayout.
func VarsInFuncFromLayout(fl *FileLayout, funcStart, funcEnd int) []string {
	start, end := lineOffsets(fl.content, funcStart, funcEnd)
	if start < 0 {
		return nil
	}
	return varsInBody(fl.content[start:end])
}

func varsInBody(body string) []string {
	seen := make(map[string]bool)
	var names []string

	// local.x and var x
	for _, re := range []*regexp.Regexp{localVarRe, tagLocalRe} {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			var name string
			if m[2] >= 0 {
				name = body[m[2]:m[3]]
			} else if m[4] >= 0 {
				name = body[m[4]:m[5]]
			}
			if name != "" && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	// arguments.x
	for _, re := range []*regexp.Regexp{argsRe, tagArgsRe} {
		for _, m := range re.FindAllStringSubmatchIndex(body, -1) {
			name := body[m[2]:m[3]]
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	return names
}

func globalVarsFromSegments(segments []string) []string {
	vars := variablesVarsFromSegments(segments)
	vars = append(vars, thisVarsFromSegments(segments)...)
	return vars
}

// VariablesVarsFromLayout returns variables-scoped names from a precomputed FileLayout.
func VariablesVarsFromLayout(fl *FileLayout) []string {
	return variablesVarsFromSegments(fl.globalSegments)
}

// ThisVarsFromLayout returns this-scoped property names from a precomputed FileLayout.
func ThisVarsFromLayout(fl *FileLayout) []string {
	return thisVarsFromSegments(fl.globalSegments)
}

func thisVarsFromSegments(segments []string) []string {
	seen := make(map[string]bool)
	var names []string
	for _, seg := range segments {
		for _, re := range []*regexp.Regexp{thisRe, tagThisRe} {
			for _, m := range re.FindAllStringSubmatchIndex(seg, -1) {
				name := seg[m[2]:m[3]]
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func variablesVarsFromSegments(segments []string) []string {
	seen := make(map[string]bool)
	var names []string

	for _, seg := range segments {
		// variables.x
		for _, re := range []*regexp.Regexp{variablesRe, tagVariablesRe} {
			for _, m := range re.FindAllStringSubmatchIndex(seg, -1) {
				name := seg[m[2]:m[3]]
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}

		// var x / local.x outside functions → variables scope
		for _, re := range []*regexp.Regexp{localVarRe, tagLocalRe} {
			for _, m := range re.FindAllStringSubmatchIndex(seg, -1) {
				var name string
				if m[2] >= 0 {
					name = seg[m[2]:m[3]]
				} else if m[4] >= 0 {
					name = seg[m[4]:m[5]]
				}
				if name != "" && !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}

		// Plain x = ...
		for _, re := range []*regexp.Regexp{plainAssignRe, tagPlainRe} {
			for _, m := range re.FindAllStringSubmatchIndex(seg, -1) {
				name := seg[m[2]:m[3]]
				if !isKeyword(name) && !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}

	return names
}

// computeGlobalSegments returns non-function, non-comment code segments.
func computeGlobalSegments(content string, funcRanges [][2]int, comments []span) []string {
	// Collect all ranges to exclude (functions + comments)
	excluded := make([][2]int, len(funcRanges), len(funcRanges)+len(comments))
	copy(excluded, funcRanges)
	for _, s := range comments {
		excluded = append(excluded, [2]int{s.Start, s.End})
	}

	// Sort by start offset
	for i := 1; i < len(excluded); i++ {
		for j := i; j > 0 && excluded[j][0] < excluded[j-1][0]; j-- {
			excluded[j], excluded[j-1] = excluded[j-1], excluded[j]
		}
	}

	var segments []string
	pos := 0
	for _, r := range excluded {
		if r[0] > pos {
			seg := content[pos:r[0]]
			if strings.TrimSpace(seg) != "" {
				segments = append(segments, seg)
			}
		}
		if r[1] > pos {
			pos = r[1]
		}
	}
	if pos < len(content) {
		seg := content[pos:]
		if strings.TrimSpace(seg) != "" {
			segments = append(segments, seg)
		}
	}
	return segments
}

// nonFuncSegments returns substrings of content that are outside function bodies
// and comments.
func nonFuncSegments(content string) []string {
	comments := findCommentSpans(content)
	return computeGlobalSegments(content, findFuncByteRangesWithComments(content, comments), comments)
}

// findFuncByteRangesWithComments returns [start, end) byte ranges using precomputed comments.
func findFuncByteRangesWithComments(content string, comments []span) [][2]int {
	scopes := findFuncScopesWithComments(content, comments)
	ranges := make([][2]int, 0, len(scopes))
	for _, s := range scopes {
		start, end := lineOffsets(content, s.Start, s.End)
		if start >= 0 {
			ranges = append(ranges, [2]int{start, end})
		}
	}
	return ranges
}

// lineOffsets converts line numbers to byte offsets in content.
// Returns start offset of startLine and end offset (after newline) of endLine.
func lineOffsets(content string, startLine, endLine int) (int, int) {
	start := 0
	line := 0
	for line < startLine {
		idx := strings.IndexByte(content[start:], '\n')
		if idx < 0 {
			return -1, -1
		}
		start += idx + 1
		line++
	}
	end := start
	for line <= endLine {
		idx := strings.IndexByte(content[end:], '\n')
		if idx < 0 {
			end = len(content)
			break
		}
		end += idx + 1
		line++
	}
	return start, end
}

func findFuncScopes(content string) []FuncScope {
	return findFuncScopesWithComments(content, findCommentSpans(content))
}

func findFuncScopesWithComments(content string, comments []span) []FuncScope {
	var scopes []FuncScope
	lines := strings.Split(content, "\n")

	for _, m := range scriptFuncStartRe.FindAllStringIndex(content, -1) {
		if inComment(m[0], comments) {
			continue
		}
		startLine := strings.Count(content[:m[0]], "\n")
		bracePos := m[1] - 1
		endLine := findMatchingBrace(content, bracePos)
		scopes = append(scopes, FuncScope{Start: startLine, End: endLine})
	}

	for _, ms := range tagFuncStartRe.FindAllStringIndex(content, -1) {
		if inComment(ms[0], comments) {
			continue
		}
		startLine := strings.Count(content[:ms[0]], "\n")
		rest := content[ms[1]:]
		endMatch := tagFuncEndRe.FindStringIndex(rest)
		endLine := len(lines) - 1
		if endMatch != nil {
			endLine = strings.Count(content[:ms[1]+endMatch[1]], "\n")
		}
		scopes = append(scopes, FuncScope{Start: startLine, End: endLine})
	}

	return scopes
}

func findMatchingBrace(content string, openPos int) int {
	depth := 1
	i := openPos + 1
	for i < len(content) && depth > 0 {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '"', '\'':
			q := content[i]
			i++
			for i < len(content) && content[i] != q {
				if content[i] == '\\' {
					i++
				}
				i++
			}
		}
		i++
	}
	return strings.Count(content[:i], "\n")
}

func findScope(line int, scopes []FuncScope) FuncScope {
	for _, s := range scopes {
		if line >= s.Start && line <= s.End {
			return s
		}
	}
	return FuncScope{Start: -1, End: -1}
}

func isKeyword(s string) bool {
	switch strings.ToLower(s) {
	case "var", "local", "if", "else", "for", "while", "do", "switch", "case",
		"try", "catch", "finally", "return", "break", "continue", "function",
		"component", "interface", "new", "throw", "import", "true", "false":
		return true
	}
	return false
}
