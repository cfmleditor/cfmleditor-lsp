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

// VarsAt returns variable names visible at the given line position,
// respecting CFML scope visibility rules:
//   - local/arguments: only inside the declaring function
//   - this/variables: anywhere in the file
//   - file-scope vars: visible everywhere (lexical scoping)
func VarsAt(content string, line int) []string {
	defs := ParseVars(content)
	scopes := findFuncScopes(content)
	curScope := findScope(line, scopes)

	seen := make(map[string]bool)
	var names []string

	for _, d := range defs {
		if d.Line > uint32(line) {
			continue
		}
		visible := false
		switch d.Scope {
		case ScopeLocal, ScopeArguments:
			// Only visible inside the same function
			visible = d.FuncStart == curScope.Start && d.FuncEnd == curScope.End
		case ScopeThis, ScopeVariables:
			// Visible anywhere in the file
			visible = true
		}
		if visible && !seen[d.Name] {
			seen[d.Name] = true
			names = append(names, d.Name)
		}
	}
	return names
}

func findFuncScopes(content string) []FuncScope {
	var scopes []FuncScope
	lines := strings.Split(content, "\n")

	for _, m := range scriptFuncStartRe.FindAllStringIndex(content, -1) {
		startLine := strings.Count(content[:m[0]], "\n")
		bracePos := m[1] - 1
		endLine := findMatchingBrace(content, bracePos)
		scopes = append(scopes, FuncScope{Start: startLine, End: endLine})
	}

	for _, ms := range tagFuncStartRe.FindAllStringIndex(content, -1) {
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
