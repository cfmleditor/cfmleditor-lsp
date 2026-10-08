// Package frameworkapi serves the API of the frameworks the presets name
// when their source is not in the workspace.
//
// A preset types `event` as coldbox.system.web.context.RequestContext, and
// with ColdBox checked out that is a real file: calls on it are checked,
// completion lists its methods and hover shows them. Without it, every call
// on it was accepted as dynamic and the editor knew nothing about it. The
// stubs here are that API, generated from each framework's own source
// (cmd/cfstubgen) — every method's signature and doc comment, with empty
// bodies — and served to the resolver from a virtual directory, Root, only
// when nothing on disk answers the dot-path. So the editor completes, checks
// and documents a framework it cannot see, and a workspace with the framework
// checked out resolves to the real files exactly as before.
//
// A stub is not something to jump to: IsStub is how go-to-definition and the
// other location answers leave them out.
package frameworkapi

import (
	"embed"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

//go:embed all:stubs
var stubs embed.FS

// Root is the virtual directory the stubs are served from. Nothing on disk
// lives there; Wrap intercepts every path beneath it.
var Root = func() string {
	if runtime.GOOS == "windows" {
		return `C:\__cfmleditor_frameworks__`
	}

	return "/__cfmleditor_frameworks__"
}()

func init() {
	// Resolution relative to a stub — a bare-word base or return type beside
	// it — goes through the path package's filesystem, not a resolver's.
	cfpath.DefaultFS = Wrap(cfpath.DefaultFS)
}

// index maps each framework to its stubs, keyed by the lowercased path under
// stubs/<framework>/, since a dot-path's case need not match the file's.
var index = sync.OnceValue(func() map[string]map[string]string {
	out := map[string]map[string]string{}

	_ = fs.WalkDir(stubs, "stubs", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cfc") {
			return err
		}

		fw, rel, ok := strings.Cut(strings.TrimPrefix(p, "stubs/"), "/")
		if !ok {
			return nil
		}

		if out[fw] == nil {
			out[fw] = map[string]string{}
		}

		out[fw][strings.ToLower(rel)] = rel

		return nil
	})

	return out
})

// Frameworks lists the frameworks that have stubs.
func Frameworks() []string {
	out := make([]string, 0, len(index()))
	for fw := range index() {
		out = append(out, fw)
	}

	slices.Sort(out)

	return out
}

// Set is the stubs for the frameworks one configuration names.
type Set struct {
	frameworks []string
}

// For is the stubs for the named frameworks, or nil when none of them has
// any — so a resolver without presets pays nothing.
func For(frameworks []string) *Set {
	var names []string

	for _, f := range frameworks {
		for _, f := range append([]string{strings.ToLower(f)}, implied[strings.ToLower(f)]...) {
			if _, ok := index()[f]; ok && !slices.Contains(names, f) {
				names = append(names, f)
			}
		}
	}

	if len(names) == 0 {
		return nil
	}

	return &Set{frameworks: names}
}

// Path is the stub for a component dot-path, or "" when none of the set's
// frameworks has one.
func (s *Set) Path(dotted string) string {
	if s == nil || dotted == "" || strings.ContainsAny(dotted, "/\\#$ ") {
		return ""
	}

	key := strings.ToLower(strings.ReplaceAll(dotted, ".", "/")) + ".cfc"

	for _, fw := range s.frameworks {
		if rel, ok := index()[fw][key]; ok {
			return filepath.Join(Root, fw, filepath.FromSlash(rel))
		}
	}

	return ""
}

// Packages lists the dot-path packages the set's stubs hold components in.
func (s *Set) Packages() []string {
	if s == nil {
		return nil
	}

	seen := map[string]bool{}

	var out []string

	for _, fw := range s.frameworks {
		for rel := range index()[fw] {
			if i := strings.LastIndexByte(rel, '/'); i > 0 {
				if pkg := strings.ReplaceAll(rel[:i], "/", "."); !seen[pkg] {
					seen[pkg] = true
					out = append(out, pkg)
				}
			}
		}
	}

	slices.Sort(out)

	return out
}

// HelperPaths are the stubs of the helper templates the set's frameworks mix
// into handlers and views (Helpers), which the resolver offers last.
func (s *Set) HelperPaths() []string {
	if s == nil {
		return nil
	}

	var out []string

	for _, fw := range s.frameworks {
		i := slices.IndexFunc(Sources, func(src Source) bool { return src.Framework == fw })
		if i < 0 {
			continue
		}

		for _, h := range Helpers[fw] {
			name := strings.TrimSuffix(path.Base(h.Template), path.Ext(h.Template))
			if rel, ok := index()[fw][strings.ToLower(Sources[i].Prefix)+"/helpers/"+strings.ToLower(name)+".cfc"]; ok {
				out = append(out, filepath.Join(Root, fw, filepath.FromSlash(rel)))
			}
		}
	}

	return out
}

// IDPackages lists the packages the set's frameworks map by file name, in
// which a bare WireBox id is looked for (idPackages).
func (s *Set) IDPackages() []string {
	if s == nil {
		return nil
	}

	var out []string
	for _, fw := range s.frameworks {
		out = append(out, idPackages[fw]...)
	}

	return out
}

// namespaces are the dot-path prefixes that can only mean one framework,
// with the framework whose stubs hold them. framework.* (FW/1) and wheels.*
// are left to their presets: a project may well have a folder of that name.
var namespaces = []struct{ prefix, framework string }{
	{"coldbox.system.", "coldbox"},
	{"testbox.system.", "testbox"},
	{"commandbox.system.", "commandbox"},
	{"qb.models.", "cfmigrations"},
	{"contentbox.models.", "contentbox"},
	{"cborm.models.", "cborm"},
	{"cbmessagebox.models.", "cbmessagebox"},
	{"cbvalidation.models.", "cbvalidation"},
	{"cbsecurity.models.", "cbsecurity"},
}

const mxunitPrefix = "mxunit."

// Namespaced is the stub for a dot-path in one of namespaces, whatever the
// configuration names, or "". The resolver asks it after the preset's own
// set, and after everything on disk.
func Namespaced(dotted string) string {
	// MXUnit is TestBox's compatibility layer: TestBox documents mapping
	// /mxunit to testbox/system/compat, so mxunit.framework.TestCase is
	// testbox.system.compat.framework.TestCase.
	if len(dotted) > len(mxunitPrefix) && strings.EqualFold(dotted[:len(mxunitPrefix)], mxunitPrefix) {
		dotted = "testbox.system.compat." + dotted[len(mxunitPrefix):]
	}

	for _, n := range namespaces {
		if len(dotted) > len(n.prefix) && strings.EqualFold(dotted[:len(n.prefix)], n.prefix) {
			return (&Set{frameworks: []string{n.framework}}).Path(dotted)
		}
	}

	return ""
}

// NamespaceOf is the framework whose namespace dotted is in, or "".
func NamespaceOf(dotted string) string {
	for _, n := range namespaces {
		if len(dotted) > len(n.prefix) && strings.EqualFold(dotted[:len(n.prefix)], n.prefix) {
			return n.framework
		}
	}

	return ""
}

// IsStub reports whether a path is one of the stubs.
func IsStub(p string) bool {
	return p == Root || strings.HasPrefix(p, Root+string(filepath.Separator))
}

// IsStubURI reports whether a file URI names one of the stubs.
func IsStubURI(u string) bool {
	return strings.Contains(u, "__cfmleditor_frameworks__") && IsStub(cfpath.FromURI(u))
}

// embedded is the embed path for a path under Root.
func embedded(p string) (string, bool) {
	p = filepath.Clean(p)
	if !IsStub(p) {
		return "", false
	}

	rest := strings.TrimPrefix(strings.TrimPrefix(p, Root), string(filepath.Separator))
	if rest == "" {
		return "stubs", true
	}

	return path.Join("stubs", filepath.ToSlash(rest)), true
}

// StubText is a stub's text, for a path under Root.
func StubText(p string) (string, bool) {
	e, ok := embedded(p)
	if !ok {
		return "", false
	}

	data, err := stubs.ReadFile(e)
	if err != nil {
		return "", false
	}

	return string(data), true
}
