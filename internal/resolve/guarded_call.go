package resolve

import (
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A call the code checks for before making it is not a missing method:
//
//	if ( structKeyExists( variables.config, "onShutdown" ) ) {
//		variables.config.onShutdown( this );
//	}
//
// is LogBox calling a convention its config may or may not follow. The guard
// is structKeyExists( receiver, "method" ), receiver.keyExists( "method" ) or
// isDefined( "receiver.method" ), not negated, in the function making the
// call, and the call must sit in what the guard controls: the block of the if
// it opens (a brace group, or a <cfif> up to its close), or the rest of the
// statement when there is no block.

// guardedByExistsCheck reports whether the call to funcName on variable at
// line (0-based) is made only after its containing code checked that the
// method exists.
func guardedByExistsCheck(pr *parser.ParseResult, variable, funcName string, line int) bool {
	if pr == nil || variable == "" || funcName == "" {
		return false
	}

	from := 0
	if scope := parser.FindFuncScopeAt(line, pr.Scopes); scope.Start != -1 {
		from = scope.Start
	}

	start, ok := lineOffset(pr.Content, from)
	if !ok {
		return false
	}

	callLine, ok := lineOffset(pr.Content, line)
	if !ok {
		return false
	}

	at := callLine
	if i := indexFoldStr(lineOfContent(pr.Content, line), variable+"."+funcName); i >= 0 {
		at += i
	}

	text := pr.Content[start:at]

	for _, m := range existsGuard(variable, funcName).FindAllStringIndex(text, -1) {
		if negatedGuard(text[:m[0]]) {
			continue
		}

		if controls(text[:m[0]], text[m[1]:]) {
			return true
		}
	}

	return false
}

func existsGuard(variable, funcName string) *regexp.Regexp {
	v, f := regexp.QuoteMeta(variable), regexp.QuoteMeta(funcName)

	return regexp.MustCompile(`(?i)structKeyExists\(\s*` + v + `\s*,\s*["']` + f + `["']\s*\)` +
		`|\b` + v + `\.keyExists\(\s*["']` + f + `["']\s*\)` +
		`|isDefined\(\s*["']` + v + `\.` + f + `["']\s*\)`)
}

// negatedGuard reports whether the text before a guard ends in a negation.
func negatedGuard(before string) bool {
	before = strings.TrimRight(before, " \t")
	if strings.HasSuffix(before, "!") {
		return true
	}

	l := strings.ToLower(before)

	return strings.HasSuffix(l, " not") || strings.HasSuffix(l, "(not")
}

// controls reports whether the code the guard opens is still open at the end
// of after, the text from the guard to the call.
func controls(before, after string) bool {
	if isTagGuard(before) {
		depth := 1

		for l := strings.ToLower(after); ; {
			o, c := strings.Index(l, "<cfif"), strings.Index(l, "</cfif")
			switch {
			case c < 0 && o < 0:
				return depth > 0
			case c < 0 || (o >= 0 && o < c):
				depth++
				l = l[o+5:]
			default:
				depth--
				if depth == 0 {
					return false
				}

				l = l[c+6:]
			}
		}
	}

	depth, opened := 0, false

	for i := range len(after) {
		switch after[i] {
		case '{':
			depth++
			opened = true
		case '}':
			depth--
			if depth <= 0 {
				return false
			}
		case ';':
			if !opened {
				return false
			}
		}
	}

	return opened && depth > 0 || !opened && strings.Count(after, "\n") <= 1
}

// isTagGuard reports whether the guard is in a <cfif> or <cfelseif>.
func isTagGuard(before string) bool {
	l := strings.ToLower(before)
	tag, script := strings.LastIndex(l, "<cf"), strings.LastIndexAny(l, ">;{}")

	return tag >= 0 && tag > script
}

func lineOffset(content string, n int) (int, bool) {
	off := 0

	for range n {
		i := strings.IndexByte(content[off:], '\n')
		if i < 0 {
			return 0, false
		}

		off += i + 1
	}

	return off, true
}

func indexFoldStr(s, sub string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sub))
}
