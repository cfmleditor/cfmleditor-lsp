package frameworkapi

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/config"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/vfs"
)

// TestEveryPresetHasItsStubs: a preset types variables as its framework's
// components and implies its bases, and each of those must have a stub, or a
// workspace without the framework gets nothing for it. A preset added without
// a Sources entry, or a component the generator did not reach, fails here.
func TestEveryPresetHasItsStubs(t *testing.T) {
	for _, fw := range config.KnownFrameworks() {
		idx := slices.IndexFunc(Sources, func(s Source) bool { return s.Framework == fw })
		if idx < 0 {
			t.Errorf("%s: no Sources entry, so no stubs", fw)

			continue
		}

		set := For([]string{fw})
		for _, c := range slices.Concat(config.PresetComponents(fw), Sources[idx].Extra) {
			if !strings.HasPrefix(strings.ToLower(c), strings.ToLower(Sources[idx].Prefix)+".") {
				continue
			}

			if pkg, ok := strings.CutSuffix(c, ".*"); ok {
				if !slices.ContainsFunc(set.Packages(), func(p string) bool { return strings.EqualFold(p, pkg) }) {
					t.Errorf("%s: no stubs in %s", fw, pkg)
				}

				continue
			}

			if set.Path(c) == "" {
				t.Errorf("%s: no stub for %s", fw, c)
			}
		}
	}
}

// TestEveryInjectionHasItsStub: a property injected with the WireBox DSL is
// typed as one of ColdBox's own classes, and without ColdBox in the workspace
// that class comes from the stubs, or the property's methods are unchecked.
func TestEveryInjectionHasItsStub(t *testing.T) {
	cb := For([]string{"coldbox"})

	for _, c := range parser.InjectedFrameworkComponents() {
		if cb.Path(c) == "" {
			t.Errorf("no stub for %s", c)
		}
	}
}

// TestAStubReturnsWhatItsDocSays: ColdBox declares few return types and
// documents most, so the generator reads `@return` — and a test's
// `var event = execute( "main.index" )` is a RequestContext. The doc's path is
// sometimes wrong (execute's names coldbox.system.context.RequestContext) and
// the class is found by its file name then; an interface is left out, since a
// cache typed as ICacheProvider would have no getOrSet().
func TestAStubReturnsWhatItsDocSays(t *testing.T) {
	cb := For([]string{"coldbox"})

	for _, tc := range []struct{ component, method, want string }{
		{"coldbox.system.testing.BaseTestCase", "execute", "coldbox.system.web.context.RequestContext"},
		{"coldbox.system.testing.BaseTestCase", "getMockController", "coldbox.system.testing.mock.web.MockController"},
		{"coldbox.system.cache.CacheFactory", "getCache", ""},
	} {
		text, ok := StubText(cb.Path(tc.component))
		if !ok {
			t.Fatalf("no stub for %s", tc.component)
		}

		pr := parser.Parse("file:///stub.cfc", text)

		i := slices.IndexFunc(pr.Funcs, func(f parser.FunctionDef) bool { return strings.EqualFold(f.Name, tc.method) })
		if i < 0 {
			t.Fatalf("%s has no %s", tc.component, tc.method)
		}

		if got := pr.Funcs[i].ReturnType; got != tc.want {
			t.Errorf("%s.%s returns %q, want %q", tc.component, tc.method, got, tc.want)
		}
	}
}

// TestStubsParseAndTheirBasesResolve: every stub parses into functions or
// properties, and a base named with the framework's own prefix is a stub
// too, so an inherited method is found through the chain.
func TestStubsParseAndTheirBasesResolve(t *testing.T) {
	all := For(Frameworks())

	err := fs.WalkDir(stubs, "stubs", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cfc") {
			return err
		}

		data, err := stubs.ReadFile(p)
		if err != nil {
			return err
		}

		pr := parser.Parse("file:///stub.cfc", string(data))
		if len(pr.Funcs) == 0 && !constantsOnly[p] {
			t.Errorf("%s: no functions", p)
		}

		fw, _, _ := strings.Cut(strings.TrimPrefix(p, "stubs/"), "/")
		src := Sources[slices.IndexFunc(Sources, func(s Source) bool { return s.Framework == fw })]

		if ext := pr.Extends; strings.HasPrefix(strings.ToLower(ext), strings.ToLower(src.Prefix)+".") && all.Path(ext) == "" {
			t.Errorf("%s: extends %s, which has no stub", p, ext)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestAnIDPackageHasItsStubs: CommandBox's WireBox maps its services and
// util packages by file name, so a module's `inject="FileSystem"` is
// commandbox.system.util.FileSystem, found through the set's IDPackages.
func TestAnIDPackageHasItsStubs(t *testing.T) {
	cb := For([]string{"commandbox"})

	for _, c := range []string{"commandbox.system.util.FileSystem", "commandbox.system.services.ServerService", "commandbox.system.util.ForgeBox"} {
		if cb.Path(c) == "" {
			t.Errorf("no stub for %s", c)
		}
	}

	if got := cb.IDPackages(); !slices.Equal(got, []string{"commandbox.system.services", "commandbox.system.util"}) {
		t.Errorf("IDPackages = %v", got)
	}
}

// TestAStubKeepsWhatItsVariablesHold: TestBox's BaseSpec sets `$assert` in
// its constructor, whose body a stub drops; every spec calls $assert, and
// without the assignment the stub gave it no type.
func TestAStubKeepsWhatItsVariablesHold(t *testing.T) {
	text, ok := StubText(For([]string{"testbox"}).Path("testbox.system.BaseSpec"))
	if !ok {
		t.Fatal("no BaseSpec stub")
	}

	pr := parser.Parse("file:///BaseSpec.cfc", text)

	found := map[bool]bool{}

	for i := range pr.ComponentRefs {
		if ref := &pr.ComponentRefs[i]; ref.Variable == "$assert" && ref.Component == "testbox.system.Assertion" {
			found[ref.This] = true
		}
	}

	if !found[true] || !found[false] {
		t.Errorf("$assert in this and variables scope: %v", found)
	}
}

// constantsOnly are the stubs of classes that hold constants and no
// methods, reached because a framework variable holds one.
var constantsOnly = map[string]bool{
	"stubs/coldbox/coldbox/system/ioc/Types.cfc": true, // this.CFC = "cfc" and the like
}

// TestPathIsCaseInsensitiveAndOnlyForTheSet: a dot-path's case is the
// writer's, not the file's; a framework the configuration does not name has
// no stubs; and no preset at all costs nothing.
func TestPathIsCaseInsensitiveAndOnlyForTheSet(t *testing.T) {
	cb := For([]string{"ColdBox"})

	want := filepath.Join(Root, "coldbox", "coldbox", "system", "web", "context", "RequestContext.cfc")
	if got := cb.Path("COLDBOX.system.web.Context.requestcontext"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}

	if got := cb.Path("testbox.system.BaseSpec"); got != "" {
		t.Errorf("a framework the set does not name answered: %q", got)
	}

	if For(nil) != nil || For([]string{"nosuch"}) != nil {
		t.Error("For should be nil when no named framework has stubs")
	}

	var none *Set
	if none.Path("coldbox.system.EventHandler") != "" {
		t.Error("a nil set answered")
	}
}

// TestOverlayServesStubsAndPassesTheRestThrough: the resolver reads a stub
// through its filesystem like any component, and everything else through the
// filesystem it had.
func TestOverlayServesStubsAndPassesTheRestThrough(t *testing.T) {
	base := fakeFS{"/w/a.cfc": "component {}"}
	o := Wrap(base)

	p := For([]string{"coldbox"}).Path("coldbox.system.EventHandler")

	data, err := o.ReadFile(p)
	if err != nil || !strings.Contains(string(data), "component extends=\"coldbox.system.FrameworkSupertype\"") {
		t.Errorf("stub read = %q, %v", data, err)
	}

	if info, err := o.Stat(p); err != nil || info.IsDir() {
		t.Errorf("stub stat = %v, %v", info, err)
	}

	entries, err := o.ReadDir(filepath.Dir(p))
	if err != nil || !slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return e.Name() == "EventHandler.cfc" }) {
		t.Errorf("stub dir listing = %v, %v", entries, err)
	}

	if data, err := o.ReadFile("/w/a.cfc"); err != nil || string(data) != "component {}" {
		t.Errorf("pass-through read = %q, %v", data, err)
	}

	if !IsStub(p) || IsStub("/w/a.cfc") || !IsStubURI(string(cfpath.ToURI(p))) {
		t.Error("IsStub/IsStubURI disagree with the overlay")
	}

	if _, ok := cfpath.DefaultFS.(overlay); !ok {
		t.Error("the path package's filesystem is not wrapped, so a bare-word base beside a stub cannot resolve")
	}
}

// TestDocAtReadsTheFrameworksOwnDocs: hover and completion show what the
// framework wrote, including ColdBox's `@name text` argument docs.
func TestDocAtReadsTheFrameworksOwnDocs(t *testing.T) {
	p := For([]string{"coldbox"}).Path("coldbox.system.web.context.RequestContext")
	text, _ := StubText(p)

	line := -1

	for i, l := range strings.Split(text, "\n") {
		if strings.Contains(l, "function getValue(") {
			line = i
		}
	}

	d, ok := DocAt(p, uint32(line))
	if !ok {
		t.Fatal("no doc for getValue")
	}

	if !strings.HasPrefix(d.Summary, "Get a value from the public or private request collection") {
		t.Errorf("summary = %q", d.Summary)
	}

	if d.Param("name") != "The key name" || d.Param("defaultValue") != "default value" {
		t.Errorf("params = %+v", d.Params)
	}

	if md := d.Markdown(); !strings.Contains(md, "`name` — The key name") {
		t.Errorf("markdown = %q", md)
	}
}

type fakeFS map[string]string

func (f fakeFS) ReadFile(p string) ([]byte, error) {
	if s, ok := f[p]; ok {
		return []byte(s), nil
	}

	return nil, fs.ErrNotExist
}

func (f fakeFS) Stat(string) (fs.FileInfo, error)      { return nil, errors.ErrUnsupported }
func (f fakeFS) ReadDir(string) ([]fs.DirEntry, error) { return nil, errors.ErrUnsupported }
func (f fakeFS) Walk(string, filepath.WalkFunc) error  { return errors.ErrUnsupported }

var _ vfs.FS = fakeFS{}

// TestMXUnitIsTestBoxsCompatibilityLayer: an MXUnit test extends
// mxunit.framework.TestCase, which TestBox serves from system/compat.
func TestMXUnitIsTestBoxsCompatibilityLayer(t *testing.T) {
	want := Namespaced("testbox.system.compat.framework.TestCase")
	if want == "" {
		t.Fatal("no compat TestCase stub")
	}

	if got := Namespaced("mxunit.framework.TestCase"); got != want {
		t.Errorf("mxunit.framework.TestCase: %q, want %q", got, want)
	}

	if got := Namespaced("mxunit.framework.Missing"); got != "" {
		t.Errorf("a class compat lacks: %q", got)
	}
}
