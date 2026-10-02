package resolve

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/conv"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// A ColdBox view reads the prc its handler action filled: comments.index sets
// prc.comments, calls event.setView( "comments/index" ), and views/comments/
// index.cfm loops over prc.comments. The view names no type for prc.X, and the
// handler that renders it is named nowhere in the view.
//
// The handoff is the setView call. For a view at <module>/views/<name>.cfm,
// every action in <module>/handlers that calls setView with the literal name
// renders it, and prc.X holds what each of those actions last assigned it
// before the call, typed in the handler with the handler's own rules. It is an
// answer only when every rendering action assigns prc.X and they agree: an
// action that does not leaves it to a pre-handler, an interceptor or a layout,
// none of which the view can see.
//
// A view no setView names (a partial rendered by renderView, a pager, a
// sidebar) has no handoff, and its prc stays unknown.

var (
	setViewRe     = regexp.MustCompile(`(?i)\bsetView\s*\(\s*(?:view\s*=\s*)?["']/?([\w./-]+)["']`)
	actionStartRe = regexp.MustCompile(`(?i)\bfunction\s+(\w+)\s*\(`)
)

// handoffAction is one handler action that renders a view.
type handoffAction struct {
	handler, name  string // the handler's path and the action's name
	start, setView int    // the lines the action starts on and calls setView on
	body           string // the action's source up to the setView call
}

// handoffIndex is every module's handler actions by the view each renders,
// built once for the life of the resolver, as startupCache is, which the
// server drops wherever a path answer may have gone stale. Per index
// generation was tried and rebuilt it all the time, since lazy indexing moves
// the generation during a scan; reading every handler per call made a
// ContentBox scan 60% slower.
type handoffIndex struct {
	modules map[string]map[string][]handoffAction // module dir → lowercased view name → actions
}

// viewActions are the handler actions that render the view at path, or nil.
func (r *Resolver) viewActions(path string) []handoffAction {
	slash := filepath.ToSlash(path)

	before, after, found := strings.CutLast(slash, "/views/")
	if !found || !strings.EqualFold(filepath.Ext(path), ".cfm") {
		return nil
	}

	module := filepath.FromSlash(before)
	view := strings.ToLower(strings.TrimSuffix(after, filepath.Ext(after)))

	r.mu.RLock()
	byView, ok := r.handoffs.modules[module]
	r.mu.RUnlock()

	if !ok {
		byView = r.moduleActions(module)

		r.mu.Lock()
		if r.handoffs.modules == nil {
			r.handoffs.modules = make(map[string]map[string][]handoffAction)
		}

		r.handoffs.modules[module] = byView
		r.mu.Unlock()
	}

	return byView[view]
}

// moduleActions is every setView with a literal view name in module's
// handlers, by that name lowercased.
func (r *Resolver) moduleActions(module string) map[string][]handoffAction {
	out := map[string][]handoffAction{}

	for _, handler := range r.moduleHandlers(module) {
		data, err := r.fs().ReadFile(handler)
		if err != nil {
			continue
		}

		content := string(data)

		for _, m := range setViewRe.FindAllStringSubmatchIndex(content, -1) {
			view := strings.ToLower(content[m[2]:m[3]])

			starts := actionStartRe.FindAllStringSubmatchIndex(content[:m[0]], -1)
			if len(starts) == 0 {
				continue
			}

			last := starts[len(starts)-1]
			out[view] = append(out[view], handoffAction{
				handler: handler,
				name:    content[last[2]:last[3]],
				start:   strings.Count(content[:last[0]], "\n"),
				setView: strings.Count(content[:m[0]], "\n"),
				body:    content[last[0]:m[0]],
			})
		}
	}

	return out
}

// moduleHandlers are the handler components under module/handlers.
func (r *Resolver) moduleHandlers(module string) []string {
	dir := filepath.Join(module, "handlers")

	var out []string

	_ = r.fs().Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a directory that cannot be read has no handlers to offer
		}

		if !info.IsDir() && strings.EqualFold(filepath.Ext(p), ".cfc") && len(out) < 512 {
			out = append(out, p)
		}

		return nil
	})

	return out
}

// handlerParse is a handler parsed as a scan parses it, once for the life of
// the resolver, as handoffIndex is.
func (r *Resolver) handlerParse(path string) *parser.ParseResult {
	key := pathKey(path)

	r.mu.RLock()
	pr, ok := r.handlerCache[key]
	r.mu.RUnlock()

	if ok {
		return pr
	}

	data, err := r.fs().ReadFile(path)
	if err != nil {
		return nil
	}

	content := string(data)

	pr = parser.ParseWithOptions(cfpath.ToURI(path), content, &parser.ParseOptions{
		Resolvers: r.Resolvers, FuncLookup: r.FuncLookup(filepath.Dir(path)),
		BeanLookup: r.InjectionBeanLookup(path), PropertyBeanLookup: r.InjectionPropertyLookup(path),
		SetterLookup: r.SetterLookup(path), ConstructorLookup: r.ConstructorLookup(path),
		ExtractCalls: true, ScanAllScopes: true,
	})
	pr.FuncLookup = r.FuncLookup(filepath.Dir(path))

	r.mu.Lock()
	if r.handlerCache == nil || len(r.handlerCache) >= 512 {
		r.handlerCache = make(map[string]*parser.ParseResult)
	}

	r.handlerCache[key] = pr
	r.mu.Unlock()

	return pr
}

// viewPrc is the component prc.X holds in the view pr when every handler
// action rendering it agrees, or "".
func (r *Resolver) viewPrc(variable, funcName string, pr *parser.ParseResult, tr *callTrace) string {
	name, ok := prcMember(variable)
	if !ok {
		return ""
	}

	actions := r.viewActions(pr.URI.Path())
	if len(actions) == 0 {
		return ""
	}

	answer := ""

	for i := range actions {
		a := &actions[i]

		hpr := r.handlerParse(a.handler)
		if hpr == nil {
			return ""
		}

		// An action that never assigns prc.X answers nothing here: prc is
		// its argument, and recordReceiver gives an argument's member only
		// the assignments the function makes to it.
		comp, _ := r.receiverComponent("prc."+name, conv.Uint32(a.setView), a.name, funcName, hpr, filepath.Dir(a.handler), nil)
		if comp == "" || strings.HasPrefix(comp, "$") {
			return ""
		}

		if p := r.ComponentPath(comp, filepath.Dir(a.handler)); p != "" {
			comp = p
		}

		if answer != "" && !cfpath.SamePath(answer, comp) {
			return ""
		}

		answer = comp
	}

	tr.addf("%q is what %d handler action(s) rendering this view assign it: %q", variable, len(actions), answer)

	return answer
}

// viewPrcElement is the element of the collection prc.X holds in the view pr,
// when every handler action rendering it agrees, or "".
func (r *Resolver) viewPrcElement(name string, pr *parser.ParseResult, depth int) string {
	actions := r.viewActions(pr.URI.Path())
	if len(actions) == 0 {
		return ""
	}

	answer := ""

	for i := range actions {
		a := &actions[i]

		rhs, ok := lastPrcAssignment(a.body, name)
		if !ok {
			return ""
		}

		hpr := r.handlerParse(a.handler)
		if hpr == nil {
			return ""
		}

		element := r.elementOf(rhs, a.setView, a.start, a.name, hpr, filepath.Dir(a.handler), depth+1)
		if element == "" || answer != "" && answer != element {
			return ""
		}

		answer = element
	}

	return answer
}

// prcMember is X for a receiver written prc.X.
func prcMember(variable string) (string, bool) {
	scope, name, ok := strings.Cut(variable, ".")
	if !ok || !strings.EqualFold(scope, "prc") || name == "" || strings.ContainsAny(name, ".[") {
		return "", false
	}

	return name, true
}

func prcAssignRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?im)^\s*(?:arguments\.)?prc\.` + regexp.QuoteMeta(name) + `\s*=\s*([^=].*?)\s*;?\s*$`)
}

// lastPrcAssignment is the right-hand side of the last `prc.name = …` in body,
// when it is written on one line and its parentheses balance.
func lastPrcAssignment(body, name string) (string, bool) {
	all := prcAssignRe(name).FindAllStringSubmatch(body, -1)
	if len(all) == 0 {
		return "", false
	}

	rhs := strings.TrimSuffix(strings.TrimSpace(all[len(all)-1][1]), ";")
	if strings.Count(rhs, "(") != strings.Count(rhs, ")") {
		return "", false
	}

	return rhs, true
}
