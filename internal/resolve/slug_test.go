package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, src := range files {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// TestAPackageNamesItselfByItsSlug: a CommandBox package's code spells its own
// components under its box.json slug, the name it is installed as —
// coldbox-platform's handlers extend coldbox.system.EventHandler, its own
// system/EventHandler.cfc — and a checkout has no mapping for that. The slug
// is found above the calling file; a path under another first segment is not
// claimed; and a configured mapping still comes first.
func TestAPackageNamesItselfByItsSlug(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"pkg/box.json":              `{ "name": "My Lib", "slug": "mylib" }`,
		"pkg/system/Base.cfc":       `component { function base() {} }`,
		"pkg/tests/specs/Spec.cfc":  `component extends="mylib.system.Base" {}`,
		"elsewhere/system/Base.cfc": `component {}`,
	})

	from := filepath.Join(dir, "pkg", "tests", "specs")
	r := &Resolver{FS: vfs.OS{}, Index: index.New()}

	if got, want := r.ComponentPath("mylib.system.Base", from), filepath.Join(dir, "pkg", "system", "Base.cfc"); got != want {
		t.Errorf("mylib.system.Base: got %q, want %q", got, want)
	}

	if got := r.ComponentPath("otherlib.system.Base", from); got != "" {
		t.Errorf("otherlib.system.Base: got %q, want nothing", got)
	}

	mapped := &Resolver{FS: vfs.OS{}, Index: index.New(), Mappings: map[string]string{"mylib": filepath.Join(dir, "elsewhere")}}
	if got, want := mapped.ComponentPath("mylib.system.Base", from), filepath.Join(dir, "elsewhere", "system", "Base.cfc"); got != want {
		t.Errorf("with a mapping: got %q, want the mapping's %q", got, want)
	}
}

// TestACallChainedOnABareCallIsCheckedOnWhatItReturns: `make().go()` is a call
// to go on what make returns. It was resolved as a bare call to go — looked
// for among this file's own methods — which reported every TestBox matcher
// chained on expect() "not found in extends chain" once the chain resolved.
func TestACallChainedOnABareCallIsCheckedOnWhatItReturns(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Widget.cfc": `component { function go() {} }`,
		"Page.cfc": `component {
	Widget function make() { return new Widget(); }
	any function untyped() { return 1; }
	function f() {
		make().go();
		make().nope();
		untyped().go();
		missing().go();
	}
}`,
	})

	file := filepath.Join(dir, "Page.cfc")

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true})
	r := &Resolver{FS: vfs.OS{}, Index: index.New()}
	got := map[string]string{}

	for _, c := range pr.AllCalls() {
		if len(c.Chain) > 0 {
			got[c.Chain[0]+"."+c.FuncName] = r.CanResolveCall(&c, pr, dir)
		}
	}

	for call, want := range map[string]string{
		"make.go":    "",
		"make.nope":  "method 'nope' not found in Widget",
		"untyped.go": "method 'untyped' has no component return type (chain to 'go')",
		"missing.go": "chained on 'missing', which is not found (calling 'go')",
	} {
		if g, ok := got[call]; !ok {
			t.Errorf("%s: not recorded as a chain; got %v", call, got)
		} else if g != want {
			t.Errorf("%s: %q, want %q", call, g, want)
		}
	}
}
