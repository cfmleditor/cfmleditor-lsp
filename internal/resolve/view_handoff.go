package resolve

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
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
	// setView, and view() or renderView() (ColdBox 7 renamed the second the
	// first): an action calls the latter to render a viewlet it returns, and
	// a view to render a partial.
	renderCallRe  = regexp.MustCompile(`(?i)\b(?:setView|renderView|view)\s*\(\s*(?:view\s*=\s*)?["']/?([\w./-]+)["']([^)]*)`)
	renderModRe   = regexp.MustCompile(`(?i)\bmodule\s*=\s*["']([^"']*)["']`)
	actionStartRe = regexp.MustCompile(`(?i)\bfunction\s+(\w+)\s*\(`)

	// A view named from a variable a subclass sets: ContentBox's base handler
	// renders "#variables.handler#/indexTable", handler being "pages" in one
	// subclass and "entries" in another.
	computedRenderRe = regexp.MustCompile(`(?i)\b(?:setView|renderView|view)\s*\(\s*(?:view\s*=\s*)?["']#variables\.(\w+)#(/[\w./-]*)["']([^)]*)`)
)

// handoffAction is one handler action that renders a view, directly or
// through the views that render it as a partial.
type handoffAction struct {
	handler, name  string   // the handler's path and the action's name
	leaf           string   // the subclass whose literals gave the view its name, or "" for a literal one
	start, setView int      // the lines the action starts on and renders on
	body           string   // the action's source up to the render call
	through        []string // the views between the action and this one, which must not assign the prc member
}

// handoffIndex is every module's handler actions by the view each renders,
// built once for the life of the resolver, as startupCache is, which the
// server drops wherever a path answer may have gone stale. Per index
// generation was tried and rebuilt it all the time, since lazy indexing moves
// the generation during a scan; reading every handler per call made a
// ContentBox scan 60% slower.
type handoffIndex struct {
	modules map[string]map[string]renderers // module dir → lowercased view name → what renders it
}

// renderers is what renders one view: handler actions, and parent views.
type renderers struct {
	actions []handoffAction
	parents []parentView
}

type parentView struct{ name, file string }

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
			r.handoffs.modules = make(map[string]map[string]renderers)
		}

		r.handoffs.modules[module] = byView
		r.mu.Unlock()
	}

	return expandPartials(byView, view, nil)
}

// expandPartials is the actions rendering view: its own, and those of every
// view that renders it as a partial, since prc is the request's and a partial
// reads what its parent's action left there.
func expandPartials(byView map[string]renderers, view string, seen []string) []handoffAction {
	if len(seen) > 4 || slices.Contains(seen, view) {
		return nil
	}

	entry := byView[view]
	out := slices.Clone(entry.actions)

	for _, parent := range entry.parents {
		inherited := expandPartials(byView, parent.name, append(slices.Clone(seen), view))
		for i := range inherited {
			a := inherited[i]
			a.through = append(slices.Clone(a.through), parent.file)
			out = append(out, a)
		}
	}

	return out
}

// moduleActions is what renders each view of module, by its name lowercased:
// every literal setView, view() or renderView() in its handlers, and every
// literal view() or renderView() in its views. A call naming another module
// renders that module's view and is not this one's.
func (r *Resolver) moduleActions(dir string) map[string]renderers {
	out := map[string]renderers{}
	name := strings.ToLower(filepath.Base(dir))

	calls := func(content string, visit func(view string, m []int)) {
		for _, m := range renderCallRe.FindAllStringSubmatchIndex(content, -1) {
			if mod := renderModRe.FindStringSubmatch(content[m[4]:m[5]]); mod != nil && !strings.EqualFold(mod[1], name) {
				continue
			}

			visit(strings.ToLower(content[m[2]:m[3]]), m)
		}
	}

	for _, handler := range r.moduleFiles(dir, "handlers", ".cfc") {
		data, err := r.fs().ReadFile(handler)
		if err != nil {
			continue
		}

		content := string(data)

		for _, m := range computedRenderRe.FindAllStringSubmatchIndex(content, -1) {
			if mod := renderModRe.FindStringSubmatch(content[m[6]:m[7]]); mod != nil && !strings.EqualFold(mod[1], name) {
				continue
			}

			starts := actionStartRe.FindAllStringSubmatchIndex(content[:m[0]], -1)
			if len(starts) == 0 {
				continue
			}

			last := starts[len(starts)-1]

			for _, leaf := range r.leafSubclasses(handler) {
				lit := r.leafLiteral(leaf, handler, content[m[2]:m[3]])
				if lit == "" {
					continue
				}

				view := strings.ToLower(lit + content[m[4]:m[5]])
				entry := out[view]
				entry.actions = append(entry.actions, handoffAction{
					handler: handler,
					leaf:    leaf,
					name:    content[last[2]:last[3]],
					start:   strings.Count(content[:last[0]], "\n"),
					setView: strings.Count(content[:m[0]], "\n"),
					body:    content[last[0]:m[0]],
				})
				out[view] = entry
			}
		}

		calls(content, func(view string, m []int) {
			starts := actionStartRe.FindAllStringSubmatchIndex(content[:m[0]], -1)
			if len(starts) == 0 {
				return
			}

			last := starts[len(starts)-1]
			entry := out[view]
			entry.actions = append(entry.actions, handoffAction{
				handler: handler,
				name:    content[last[2]:last[3]],
				start:   strings.Count(content[:last[0]], "\n"),
				setView: strings.Count(content[:m[0]], "\n"),
				body:    content[last[0]:m[0]],
			})
			out[view] = entry
		})
	}

	views := filepath.Join(dir, "views")

	for _, file := range r.moduleFiles(dir, "views", ".cfm") {
		data, err := r.fs().ReadFile(file)
		if err != nil {
			continue
		}

		rel, err := filepath.Rel(views, file)
		if err != nil {
			continue
		}

		parent := strings.ToLower(strings.TrimSuffix(filepath.ToSlash(rel), filepath.Ext(rel)))

		calls(string(data), func(view string, _ []int) {
			entry := out[view]
			entry.parents = append(entry.parents, parentView{name: parent, file: file})
			out[view] = entry
		})
	}

	return out
}

// moduleFiles are the files with extension ext under module/sub.
func (r *Resolver) moduleFiles(module, sub, ext string) []string {
	dir := filepath.Join(module, sub)

	var out []string

	_ = r.fs().Walk(dir, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // a directory that cannot be read has no handlers to offer
		}

		if !info.IsDir() && strings.EqualFold(filepath.Ext(p), ext) && len(out) < 2048 {
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
		if r.throughAssigns(a.through, name) {
			return ""
		}

		hpr := r.handlerParse(a.handler)
		if hpr == nil {
			return ""
		}

		// An action that never assigns prc.X answers nothing here: prc is
		// its argument, and recordReceiver gives an argument's member only
		// the assignments the function makes to it.
		comp, _ := r.receiverComponentD("prc."+name, conv.Uint32(a.setView), a.name, funcName, hpr, filepath.Dir(a.handler), nil, lookupCtx{leaf: a.leaf})
		if comp == "" || strings.HasPrefix(comp, "$") {
			return ""
		}

		comp = r.pathsOf(comp, filepath.Dir(a.handler))

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
		if r.throughAssigns(a.through, name) {
			return ""
		}

		rhs, ok := lastPrcAssignment(a.body, name)
		if !ok {
			return ""
		}

		hpr := r.handlerParse(a.handler)
		if hpr == nil {
			return ""
		}

		element := r.elementOf(rhs, a.setView, a.start, a.name, hpr, filepath.Dir(a.handler), depth+1, a.leaf)
		if element == "" || answer != "" && answer != element {
			return ""
		}

		answer = element
	}

	return answer
}

// throughAssigns reports whether a view between an action and the partial
// it reaches assigns prc.name itself, so that the action's value is not the
// one the partial reads.
func (r *Resolver) throughAssigns(views []string, name string) bool {
	re := regexp.MustCompile(`(?i)\bprc\.` + regexp.QuoteMeta(name) + `\s*=[^=]`)

	for _, v := range views {
		data, err := r.fs().ReadFile(v)
		if err != nil || re.Match(data) {
			return true
		}
	}

	return false
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

// pathsOf is comp with each alternative (a|b) replaced by its file where one
// resolves: ComponentPath answers for one component, and handed a list would
// answer for its first.
func (r *Resolver) pathsOf(comp, baseDir string) string {
	if !strings.Contains(comp, "|") {
		if p := r.ComponentPath(comp, baseDir); p != "" {
			return p
		}

		return comp
	}

	var alts []string

	for alt := range strings.SplitSeq(comp, "|") {
		if p := r.ComponentPath(alt, baseDir); p != "" {
			alt = p
		}

		alts = append(alts, alt)
	}

	return strings.Join(alts, "|")
}
