package route

import (
	"path/filepath"
	"strings"
)

// Lookups are the questions resolution asks about the workspace. They are hooks
// rather than a dependency on internal/resolve and internal/index, because this
// package is used from the code-map builder, from the LSP handlers and from
// tests, and each has a different idea of what "the workspace" is.
type Lookups struct {
	// ComponentPath turns a dot-path into an absolute .cfc path, or "".
	ComponentPath func(component string) string

	// HasMethod reports whether the component file declares the method. Matching
	// must be case-insensitive.
	HasMethod func(cfcPath, method string) bool

	// DirExists reports whether a slash-separated path, relative to the workspace
	// root, is a directory.
	DirExists func(rel string) bool

	// FindFile returns the absolute path of a slash-separated file relative to the
	// workspace root, or "".
	FindFile func(rel string) string
}

// Resolver applies a Config to routes.
type Resolver struct {
	Config  Config
	Lookups Lookups
}

// Resolve returns every target a route reaches, best first.
//
// More than one is a real answer, not a failure to decide: a view shared between
// products resolves into whichever product is serving the page, and which that is
// cannot be known statically. The caller decides whether to take the first, take
// them all, or mark the edge as a guess — on one workspace 497 of 523 resolvable
// routes were unambiguous and 26 genuinely reached several products.
//
// Controllers are tried before views because a route that names a real method is
// a call, and the view of the same name is usually the template that method
// renders — reporting the file and not the method would lose the edge that
// matters.
func (r *Resolver) Resolve(route string) []Target {
	segs := Split(route)
	if len(segs) == 0 {
		return nil
	}

	var (
		out  []Target
		seen = make(map[string]bool)
	)

	add := func(t Target) {
		key := string(t.Kind) + "\x00" + t.Component + "\x00" + strings.ToLower(t.Method) + "\x00" + t.Path
		if seen[key] {
			return
		}

		seen[key] = true

		out = append(out, t)
	}

	for _, exp := range r.Config.expansions(segs) {
		for i, rule := range r.Config.Controllers {
			if t, ok := r.controller(rule, exp.segs); ok {
				t.Rule = i
				t.Alias = exp.alias
				add(t)
			}
		}
	}

	for _, exp := range r.Config.expansions(segs) {
		for i, rule := range r.Config.Views {
			if t, ok := r.view(rule, exp.segs); ok {
				t.Rule = i
				t.Alias = exp.alias
				add(t)
			}
		}
	}

	return out
}

func (r *Resolver) controller(rule ControllerRule, segs []string) (Target, bool) {
	if r.Lookups.ComponentPath == nil || r.Lookups.HasMethod == nil {
		return Target{}, false
	}

	component, ok := expand(rule.Component, segs)
	if !ok || component == "" {
		return Target{}, false
	}

	methods := expandAll(rule.Method, segs)
	if len(methods) == 0 {
		return Target{}, false
	}

	path := r.Lookups.ComponentPath(component)
	if path == "" {
		return Target{}, false
	}

	// The method check is not optional. A component template built from the first
	// segment or two matches far more routes than it should — every route starting
	// "tassweb." resolves tassweb.cfc — so without it a rule would claim routes it
	// has no method for and bury the rules below it that do. It is also what makes
	// :search safe: a start point that names nothing simply does not match.
	for _, method := range methods {
		if r.Lookups.HasMethod(path, method) {
			return Target{Kind: KindController, Component: component, Method: method, Path: path}, true
		}
	}

	return Target{}, false
}

func (r *Resolver) view(rule ViewRule, segs []string) (Target, bool) {
	exts := rule.Ext
	if len(exts) == 0 {
		exts = []string{".cfm"}
	}

	if rule.LongestDir {
		return r.longestDir(rule, segs, exts)
	}

	if rule.Path == "" {
		return Target{}, false
	}

	// The template controls its own separators: literal "/" between directories,
	// and whatever join the segment reference asks for inside a name. Rewriting
	// every dot to a slash here — which this did — made a dotted file name
	// impossible to express, and a dotted file name is exactly what a convention
	// like "webroot/${2}/${3+}" needs: webroot/assessment plus
	// dialog.objectivegroup.setup.cfm, not a directory called dialog.
	rel, ok := expand(rule.Path, segs)
	if !ok || rel == "" {
		return Target{}, false
	}

	for _, ext := range exts {
		if p := r.findFile(rule.Root, rel+ext); p != "" {
			return Target{Kind: KindView, Path: p}, true
		}
	}

	return Target{}, false
}

// longestDir walks the split point between directory and file name from the right,
// taking the first one that names a directory that exists.
//
// From the right rather than the left: the file name carries dots of its own
// ("popup.lookup.filter.cfm"), so the leftmost directory that exists is almost
// never the one meant. "ui.web.general.popup.lookup.filter" has ui, ui/web and
// ui/web/general all real, and only the longest gives a file.
func (r *Resolver) longestDir(rule ViewRule, segs, exts []string) (Target, bool) {
	if r.Lookups.DirExists == nil || r.Lookups.FindFile == nil {
		return Target{}, false
	}

	for cut := len(segs) - 1; cut >= 1; cut-- {
		dir := strings.Join(segs[:cut], "/")
		if rule.Root != "" {
			dir = strings.TrimSuffix(rule.Root, "/") + "/" + dir
		}

		if !r.Lookups.DirExists(dir) {
			continue
		}

		name := strings.Join(segs[cut:], ".")
		for _, ext := range exts {
			if p := r.Lookups.FindFile(dir + "/" + name + ext); p != "" {
				return Target{Kind: KindView, Path: p}, true
			}
		}
	}

	return Target{}, false
}

func (r *Resolver) findFile(root, rel string) string {
	if r.Lookups.FindFile == nil {
		return ""
	}

	if root != "" {
		rel = strings.TrimSuffix(root, "/") + "/" + rel
	}

	return r.Lookups.FindFile(filepath.ToSlash(rel))
}

// Enabled reports whether the config does anything.
func (c Config) Enabled() bool {
	hasSource := len(c.Attributes) > 0 || len(c.QueryParams) > 0 ||
		len(c.Properties) > 0 || len(c.Functions) > 0

	return hasSource && (len(c.Controllers) > 0 || len(c.Views) > 0)
}

func normalise(names []string) []string {
	out := make([]string, 0, len(names))

	for _, n := range names {
		if n = strings.ToLower(strings.TrimSpace(n)); n != "" {
			out = append(out, n)
		}
	}

	return out
}
