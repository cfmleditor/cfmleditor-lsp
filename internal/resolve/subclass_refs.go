package resolve

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

// An abstract component calls what its subclasses supply: ContentBox's
// baseContentHandler uses variables.ormService and never declares it, and each
// concrete handler injects a different service under that name. A receiver is
// looked up in the file and then up its extends chain, never down, so every
// call on it was "has no component ref".
//
// subclassComponent asks the components that extend the file what they hold
// for the name. It answers only when every concrete subclass (a leaf of the
// tree under the file) types it, by a ref of its own or of a component between
// it and the file: one that does not leaves the name to a pre-handler, a mixin
// or a runtime set, and the others' answer would be a guess. The components
// are returned as alternatives, as withSubclasses spells them, so a method any
// of them declares is found.
//
// Only a name the file never types, only variables scope or unscoped, and only
// where the file is extended within the workspace; the walk is capped at
// maxSubclasses.

const maxSubclasses = 16

func (r *Resolver) subclassComponent(variable string, line uint32, pr *parser.ParseResult, tr *callTrace, leaf string) string {
	if r.Index == nil || pr == nil || variable == "" || strings.ContainsAny(variable, "[(") {
		return ""
	}

	name := parser.StripReceiverScope(variable)
	if name == "" || strings.Contains(name, ".") {
		return ""
	}

	root, _, qualified := strings.Cut(variable, ".")
	if qualified && !strings.EqualFold(root, "variables") {
		return ""
	}

	// An unscoped name a function declares is its local, not the component's.
	if !qualified {
		if scope := parser.FindFuncScopeAt(int(line), pr.Scopes); scope.Start != -1 {
			for _, v := range pr.FuncVars(scope.Start, scope.End) {
				if strings.EqualFold(v, name) {
					return ""
				}
			}
		}
	}

	base := pr.URI.Path()
	leaves := r.leafSubclasses(base)

	if leaf != "" {
		leaves = slices.DeleteFunc(leaves, func(l string) bool { return !cfpath.SamePath(l, leaf) })
	}

	if len(leaves) == 0 {
		return ""
	}

	scope := parser.ReceiverRefScope(variable)

	var comps []string

	for _, leaf := range leaves {
		comp := r.subclassRef(leaf, base, name, scope)
		if comp == "" {
			tr.addf("subclass %s does not type %q, so no subclass answer is given", filepath.Base(leaf), name)

			return ""
		}

		if !slices.ContainsFunc(comps, func(c string) bool { return strings.EqualFold(c, comp) }) {
			comps = append(comps, comp)
		}
	}

	slices.Sort(comps)

	answer := strings.Join(comps, "|")

	tr.addf("resolved %q to %q: what every subclass of %s holds for it", variable, answer, filepath.Base(base))

	return answer
}

// leafSubclasses are the components under base, directly or not, that nothing
// extends, in path order.
func (r *Resolver) leafSubclasses(base string) []string {
	if r.Index == nil {
		return nil
	}

	all := []string{base}

	for i := 0; i < len(all) && len(all) <= maxSubclasses; i++ {
		parent := all[i]
		name := strings.TrimSuffix(filepath.Base(parent), filepath.Ext(parent))

		subs := r.Index.FilesExtendingName(name)
		slices.Sort(subs)

		for _, u := range subs {
			sub := cfpath.FromURI(u)

			if slices.ContainsFunc(all, func(p string) bool { return cfpath.SamePath(p, sub) }) {
				continue
			}

			if r.descendsFrom(sub, parent) {
				all = append(all, sub)
			}
		}
	}

	var leaves []string

	for _, c := range all[1:] {
		name := strings.TrimSuffix(filepath.Base(c), filepath.Ext(c))

		extended := false

		for _, u := range r.Index.FilesExtendingName(name) {
			if r.descendsFrom(cfpath.FromURI(u), c) {
				extended = true

				break
			}
		}

		if !extended {
			leaves = append(leaves, c)
		}
	}

	return leaves
}

// subclassRef is the component the file at path, or one of the components
// between it and base, holds for name at file level, nearest first.
func (r *Resolver) subclassRef(path, base, name string, scope parser.RefScope) string {
	for range 16 {
		if cfpath.SamePath(path, base) {
			return ""
		}

		for _, ref := range r.Index.RefsForFile(cfpath.ToURI(path)) {
			if ref != nil && ref.Component != "" && strings.EqualFold(ref.Variable, name) && scope.Admits(ref) {
				return ref.Component
			}
		}

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return ""
		}

		path = r.ComponentPath(ext, filepath.Dir(path))
		if path == "" {
			return ""
		}
	}

	return ""
}

// leafLiteral is the literal string a component assigns `variables.name` in
// its own source, or in the nearest component it extends below base: the
// value a subclass gives a variable its base reads, as ContentBox's handlers
// give entityPlural and handler. "" when none writes one.
func (r *Resolver) leafLiteral(leaf, base, name string) string {
	re := regexp.MustCompile(`(?i)\bvariables\.` + regexp.QuoteMeta(name) + `\s*=\s*["']([^"'#]*)["']`)

	path := leaf

	for range 16 {
		if path == "" || cfpath.SamePath(path, base) {
			return ""
		}

		if data, err := r.fs().ReadFile(path); err == nil {
			if m := re.FindSubmatch(data); m != nil && len(m[1]) > 0 {
				return string(m[1])
			}
		}

		ext, ok := r.extendsOf(path, cfpath.ToURI(path))
		if !ok || ext == "" {
			return ""
		}

		path = r.ComponentPath(ext, filepath.Dir(path))
	}

	return ""
}
