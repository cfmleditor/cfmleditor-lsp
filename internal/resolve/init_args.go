package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// A component's constructor argument is what every construction passes it.
// cfwheels' CLI services are built as `new services.Templates( helpers = h )`
// and keep it as `variables.helpers = arguments.helpers;`, and ColdBox's
// BoxLangProvider makes its stats with `new BoxLangStats( this )`. Nothing in
// the component says what the argument is, and every call on the variable was
// "has no component ref".
//
// inferInitArgument is argumentFromCallers for init: its callers are found by
// the component's file name, since `init(` is in every file. A `new X( … )`
// whose path resolves to the component is a construction, as is an `init()`
// call the existing caller check places on it (createObject( … ).init( … )).
// Any other mention of the name in a string (getInstance( "X" ),
// createObject without an init on its line) is a construction that cannot be
// read, and leaves the argument untyped.
//
// initArgMember carries the answer into the component: a variables.x whose
// only assignments are `variables.x = arguments.p` inside init holds what p is
// given.

func (r *Resolver) inferInitArgument(fd *parser.FunctionDef, pos int, file string, ctx lookupCtx) string {
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))

	files := r.callerFiles(base)
	for _, f := range r.callerFiles(quotedCallerKey(base)) {
		if !slices.Contains(files, f) {
			files = append(files, f)
		}
	}

	if len(files) == 0 || len(files) > maxArgCallerFiles {
		return ""
	}

	argName := fd.Arguments[pos].Name

	var (
		comps []string
		sites int
	)

	add := func(comp, dir string) bool {
		if comp == "" {
			return false
		}

		for alt := range strings.SplitSeq(r.pathsOf(comp, dir), "|") {
			if !containsFold(comps, alt) {
				comps = append(comps, alt)
			}
		}

		return true
	}

	for _, path := range files {
		hpr := r.handlerParse(path)
		if hpr == nil {
			return ""
		}

		dir := filepath.Dir(path)
		tokens := allTokens(hpr.Content)
		read := map[int]bool{}

		for _, site := range r.constructions(tokens, base, file, dir) {
			read[site.line] = true

			expr, passed := callArgumentAt(tokens, site.at, pos, argName)
			if !passed {
				continue
			}

			sites++
			if sites > maxArgCallSites {
				return ""
			}

			caller := parser.FindFuncScopeAt(site.line, hpr.Scopes).Name
			if !add(r.argumentExprComponent(expr, conv.Uint32(site.line), caller, hpr, dir, ctx), dir) {
				return ""
			}
		}

		calls := hpr.AllCalls()
		for ci := range calls {
			call := &calls[ci]
			if !strings.EqualFold(call.FuncName, "init") {
				continue
			}

			if ours, known := r.callIsTo(call, hpr, path, fd, file, dir, ctx); !ours || !known {
				continue
			}

			read[int(call.Line)] = true

			expr, passed := callArgument(tokens, call.FuncName, int(call.Line), pos, argName)
			if !passed {
				continue
			}

			sites++
			if sites > maxArgCallSites || !add(r.argumentExprComponent(expr, call.Line, call.Caller, hpr, dir, ctx), dir) {
				return ""
			}
		}

		for _, line := range quotedNameLines(hpr.Content, base) {
			if !read[line] {
				return ""
			}
		}
	}

	if sites == 0 {
		return ""
	}

	slices.Sort(comps)

	return strings.Join(comps, "|")
}

type construction struct {
	at   int // the index of the component's name token
	line int
}

// constructions are the `new a.b.Name(` sites in tokens whose path, read from
// dir, is file.
func (r *Resolver) constructions(tokens []parser.Token, base, file, dir string) []construction {
	var out []construction

	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i].Kind != parser.TokIdent || !strings.EqualFold(tokens[i].Value, "new") {
			continue
		}

		j := i + 1

		var dotted strings.Builder

		for j < len(tokens) && tokens[j].Kind == parser.TokIdent {
			dotted.WriteString(tokens[j].Value)

			if j+1 < len(tokens) && tokens[j+1].Kind == parser.TokDot {
				dotted.WriteByte('.')

				j += 2

				continue
			}

			break
		}

		if j+1 >= len(tokens) || tokens[j+1].Kind != parser.TokLParen || !strings.EqualFold(tokens[j].Value, base) {
			continue
		}

		if p := r.ComponentPath(dotted.String(), dir); p != "" && cfpath.SamePath(p, file) {
			out = append(out, construction{at: j, line: tokens[j].Line})
		}
	}

	return out
}

// quotedNameLines are the lines on which name ends a quoted string.
func quotedNameLines(content, name string) []int {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `["']`)
	matches := re.FindAllStringIndex(content, -1)

	out := make([]int, 0, len(matches))
	for _, m := range matches {
		out = append(out, strings.Count(content[:m[0]], "\n"))
	}

	return out
}

var initArgAssignRe = regexp.MustCompile(`(?i)^\s*(?:variables\.)?(\w+)\s*=\s*arguments\.(\w+)\s*;?\s*$`)

// initArgMember is what variables.name holds when its only assignments in the
// file are `variables.name = arguments.p` inside init: what every
// construction passes as p.
func (r *Resolver) initArgMember(variable string, pr *parser.ParseResult, ctx lookupCtx) string {
	name, ok := strings.CutPrefix(strings.ToLower(variable), "variables.")
	if !ok || name == "" || strings.ContainsAny(name, ".[(") || !pr.URI.IsFile() {
		return ""
	}

	scope, ok := funcScope(pr, "init")
	if !ok {
		return ""
	}

	assign := regexp.MustCompile(`(?i)(?:^|[^\w.])(?:variables\.)?` + regexp.QuoteMeta(name) + `\s*=[^=]`)
	param := ""

	for i, line := range strings.Split(pr.Content, "\n") {
		if !assign.MatchString(line) {
			continue
		}

		m := initArgAssignRe.FindStringSubmatch(line)
		if m == nil || !strings.EqualFold(m[1], name) || i < scope.Start || i > scope.End || param != "" && !strings.EqualFold(param, m[2]) {
			return ""
		}

		param = m[2]
	}

	if param == "" {
		return ""
	}

	return r.argumentFromCallers("arguments."+param, "init", pr, ctx)
}
