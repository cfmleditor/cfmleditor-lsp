package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// A ColdBox view reads args.X, the struct the code rendering it passed:
//
//	#view( view : "_components/content/TableCreationInfo", args : { content : content } )#
//
// ContentBox's admin renders its table partials this way from every content
// listing, and its UI module renders the admin bar from an interceptor. The
// view names no type for args.X, and nothing in it says who renders it.
//
// viewArgs reads every literal view(), renderView() or setView() in the
// view's module (its handlers, views, interceptors and layouts) that names the
// view and passes an args struct literal, and types the value given for X
// where the call is made, as a caller's argument is typed: a variable, a
// dotted name, `new X()`, a single call, or a loop variable at that point. A
// render that passes args other than as a literal leaves it untyped; one that
// omits X says nothing about it. Every render that passes X must type it, and
// they are the alternatives.

var viewRenderNames = []string{"view", "renderview", "setview"}

// viewRenderDirs are a module's directories whose code renders its views.
var viewRenderDirs = []string{"handlers", "views", "interceptors", "layouts"}

// viewArgSite is one render of a view: the file, the call's line, and the
// tokens of the value passed as args, nil when it passes none.
type viewArgSite struct {
	file string
	line int
	args []parser.Token
}

// viewArgs is the component args.X holds in the ColdBox view pr, or "".
func (r *Resolver) viewArgs(variable string, pr *parser.ParseResult, tr *callTrace) string {
	scope, name, ok := strings.Cut(variable, ".")
	if !ok || !strings.EqualFold(scope, "args") || name == "" || strings.ContainsAny(name, ".[") || !pr.URI.IsFile() {
		return ""
	}

	module, view, ok := viewNameOf(pr.URI.Path())
	if !ok {
		return ""
	}

	sites := r.viewArgSites(module)[view]
	if len(sites) == 0 {
		return ""
	}

	var comps []string

	for _, s := range sites {
		if len(s.args) == 0 {
			continue
		}

		value, passed, literal := argsMember(s.args, name)
		if !literal {
			return ""
		}

		if !passed {
			continue
		}

		comp := r.renderedValue(value, s)
		if comp == "" {
			return ""
		}

		for alt := range strings.SplitSeq(comp, "|") {
			if !containsFold(comps, alt) {
				comps = append(comps, alt)
			}
		}
	}

	if len(comps) == 0 {
		return ""
	}

	slices.Sort(comps)

	answer := strings.Join(comps, "|")
	tr.addf("%q is what the %d render(s) of this view pass it: %q", variable, len(sites), answer)

	return answer
}

// renderedValue types value, passed at the render site s.
func (r *Resolver) renderedValue(value []parser.Token, s viewArgSite) string {
	hpr := r.handlerParse(s.file)
	if hpr == nil {
		return ""
	}

	dir := filepath.Dir(s.file)

	caller := ""
	if scope := parser.FindFuncScopeAt(s.line, hpr.Scopes); scope.Start != -1 {
		caller = scope.Name
	}

	value = withoutNullFallback(value)

	comp := r.argumentExprComponent(value, conv.Uint32(s.line), caller, hpr, dir, lookupCtx{})
	if comp == "" && len(value) == 1 && value[0].Kind == parser.TokIdent {
		comp = r.loopElement(&parser.CallSite{Variable: value[0].Value, Line: conv.Uint32(s.line), Caller: caller}, hpr, dir, nil)
	}

	if strings.HasPrefix(comp, "$") {
		return ""
	}

	return r.pathsOf(comp, dir)
}

// withoutNullFallback is value without a trailing `?: javacast( "null", "" )`,
// which passes null when the value is missing: ContentBox's admin bar is
// handed `oContent ?: javacast( "null", "" )`, the content or nothing.
func withoutNullFallback(value []parser.Token) []parser.Token {
	depth := 0

	for i := range len(value) - 1 {
		switch value[i].Kind {
		case parser.TokLParen, parser.TokLBracket, parser.TokLBrace:
			depth++
		case parser.TokRParen, parser.TokRBracket, parser.TokRBrace:
			depth--
		case parser.TokQuestion:
			if depth == 0 && value[i+1].Kind == parser.TokColon && i+2 < len(value) && strings.EqualFold(value[i+2].Value, "javacast") {
				return value[:i]
			}
		default:
		}
	}

	return value
}

// argsMember is the value args (`{ k : v, … }`) gives name, whether it gives
// one, and whether args is a struct literal at all.
func argsMember(args []parser.Token, name string) (value []parser.Token, passed, literal bool) {
	if len(args) < 2 || args[0].Kind != parser.TokLBrace || args[len(args)-1].Kind != parser.TokRBrace {
		return nil, false, false
	}

	for _, piece := range producerSplit(args[1 : len(args)-1]) {
		if len(piece) < 3 || piece[1].Kind != parser.TokColon && piece[1].Kind != parser.TokEquals {
			return nil, false, false
		}

		if strings.EqualFold(strings.Trim(piece[0].Value, `"'`), name) {
			return piece[2:], true, true
		}
	}

	return nil, false, true
}

// viewNameOf is the module directory holding a view and the view's name in
// it, lowercased, as a render call spells it.
func viewNameOf(path string) (module, view string, ok bool) {
	slash := filepath.ToSlash(path)

	before, after, found := strings.CutLast(slash, "/views/")
	if !found || !strings.EqualFold(filepath.Ext(path), ".cfm") {
		return "", "", false
	}

	return filepath.FromSlash(before), strings.ToLower(strings.TrimSuffix(after, filepath.Ext(after))), true
}

// viewArgSites is every render of module's views, by view name, built once
// per module for the life of the resolver, as handoffIndex is.
func (r *Resolver) viewArgSites(module string) map[string][]viewArgSite {
	o := r.owner()
	key := pathKey(module)

	o.mu.RLock()
	sites, ok := o.viewArgCache[key]
	o.mu.RUnlock()

	if ok {
		return sites
	}

	sites = map[string][]viewArgSite{}
	name := strings.ToLower(filepath.Base(module))

	for _, sub := range viewRenderDirs {
		for _, ext := range []string{".cfc", ".cfm"} {
			for _, file := range r.moduleFiles(module, sub, ext) {
				data, err := r.fs().ReadFile(file)
				if err != nil {
					continue
				}

				for view, s := range renderSites(string(data), file, name) {
					sites[view] = append(sites[view], s...)
				}
			}
		}
	}

	o.mu.Lock()
	if o.viewArgCache == nil {
		o.viewArgCache = map[string]map[string][]viewArgSite{}
	}

	o.viewArgCache[key] = sites
	o.mu.Unlock()

	return sites
}

var renderStartRe = regexp.MustCompile(`(?i)\b(?:setView|renderView|view)\s*\(`)

// renderWindow bounds the text tokenized for one render call, so a page is not
// tokenized from each call to its end.
const renderWindow = 8000

// renderSites are the renders in content of a view of the module named
// module, by view name.
func renderSites(content, file, module string) map[string][]viewArgSite {
	out := map[string][]viewArgSite{}

	for _, m := range renderStartRe.FindAllStringIndex(content, -1) {
		window := content[m[0]:min(len(content), m[0]+renderWindow)]
		base := strings.Count(content[:m[0]], "\n")

		tokens := significantTokens(window)
		if len(tokens) < 2 || !slices.Contains(viewRenderNames, strings.ToLower(tokens[0].Value)) || tokens[1].Kind != parser.TokLParen {
			continue
		}

		end := producerGroupEnd(tokens, 1, parser.TokLParen, parser.TokRParen)
		if end < 0 {
			continue
		}

		view, args, ok := renderCall(tokens[2:end], module)
		if !ok {
			continue
		}

		out[view] = append(out[view], viewArgSite{file: file, line: base, args: args})
	}

	return out
}

// renderCall reads a render call's arguments: the view it names, lowercased
// and without a leading slash, and the tokens of its args value. A call
// naming another module, or a view by anything but a literal, is not read.
func renderCall(arguments []parser.Token, module string) (view string, args []parser.Token, ok bool) {
	for i, piece := range producerSplit(arguments) {
		if len(piece) == 1 && i == 0 && piece[0].Kind == parser.TokString {
			view = strings.Trim(piece[0].Value, `"'`)

			continue
		}

		if len(piece) < 3 || piece[0].Kind != parser.TokIdent || piece[1].Kind != parser.TokColon && piece[1].Kind != parser.TokEquals {
			continue
		}

		switch strings.ToLower(piece[0].Value) {
		case "view":
			if len(piece) != 3 || piece[2].Kind != parser.TokString {
				return "", nil, false
			}

			view = strings.Trim(piece[2].Value, `"'`)
		case "module":
			if len(piece) != 3 || piece[2].Kind != parser.TokString || !strings.EqualFold(strings.Trim(piece[2].Value, `"'`), module) {
				return "", nil, false
			}
		case "args":
			args = piece[2:]
		default:
		}
	}

	view = strings.ToLower(strings.TrimPrefix(view, "/"))
	if view == "" || strings.Contains(view, "#") {
		return "", nil, false
	}

	return view, args, true
}
