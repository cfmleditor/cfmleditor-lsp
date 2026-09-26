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

// includeFixture is a component that mixes in two templates, extends a base,
// and a page that includes a helper through a mapping.
func includeFixture(t *testing.T) (dir string, r *Resolver) {
	t.Helper()

	dir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o750); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"Base.cfc": `<cfcomponent><cffunction name="baseFn"></cffunction></cfcomponent>`,
		"api.cfc": `<cfcomponent extends="Base">
	<cffunction name="getService"></cffunction>
	<cfinclude template="api-a.cfm">
	<cfinclude template="api-b.cfm">
</cfcomponent>`,
		"api-a.cfm":      `<cffunction name="fromA"></cffunction>`,
		"api-b.cfm":      `<cffunction name="fromB"></cffunction>`,
		"page.cfm":       `<cfinclude template="/app/lib/helper.cfm">`,
		"lib/helper.cfm": `<cffunction name="helperFn"></cffunction>`,
	}

	r = &Resolver{FS: vfs.OS{}, Index: index.New(), Mappings: map[string]string{"app": dir}}

	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		r.Index.IndexFile(cfpath.ToURI(p), body)
	}

	return dir, r
}

func bareCall(t *testing.T, r *Resolver, file, funcName string) (CallTarget, string) {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	pr := parser.Parse(cfpath.ToURI(file), string(data))

	return r.ResolveCallTarget(&parser.CallSite{FuncName: funcName}, pr, filepath.Dir(file))
}

// TestBareCallResolvesThroughCfinclude covers a template mixed into a
// component: it runs in that component's variables scope, so it can call what
// the component declares, what the component extends, and what a sibling
// template declares; and a page can call what it includes. Before, all of
// these were "no qualifier, not in file".
func TestBareCallResolvesThroughCfinclude(t *testing.T) {
	dir, r := includeFixture(t)

	for _, tc := range []struct {
		file, fn, wantIn string
	}{
		{"api-a.cfm", "getService", "api.cfc"},     // the includer
		{"api-a.cfm", "fromB", "api-b.cfm"},        // a sibling mixin
		{"api-a.cfm", "baseFn", "Base.cfc"},        // the includer's extends chain
		{"api.cfc", "fromA", "api-a.cfm"},          // the includer calling its mixin
		{"page.cfm", "helperFn", "lib/helper.cfm"}, // through a mapping
	} {
		target, reason := bareCall(t, r, filepath.Join(dir, tc.file), tc.fn)
		if reason != "" {
			t.Errorf("%s: %s() unresolved: %s", tc.file, tc.fn, reason)

			continue
		}

		want := cfpath.ToURI(filepath.Join(dir, filepath.FromSlash(tc.wantIn)))
		if target.Kind != TargetInclude && target.Kind != TargetExtends || !cfpath.SamePath(string(target.URI), string(want)) {
			t.Errorf("%s: %s() landed on %s %s, want %s", tc.file, tc.fn, target.Kind, target.URI, want)
		}
	}

	if _, reason := bareCall(t, r, filepath.Join(dir, "api-a.cfm"), "nowhere"); reason == "" {
		t.Error("nowhere() resolved; a name no file in scope declares must stay unresolved")
	}

	// page.cfm is not in api.cfc's scope, so it cannot see api's functions.
	if _, reason := bareCall(t, r, filepath.Join(dir, "page.cfm"), "getService"); reason == "" {
		t.Error("page.cfm resolved getService() through a component it is not included by")
	}
}

// TestIncludeGraphFollowsReindex is the invalidation: the resolver keeps a
// reverse map of every include, and an edit that drops one has to reach it.
func TestIncludeGraphFollowsReindex(t *testing.T) {
	dir, r := includeFixture(t)
	mixin := filepath.Join(dir, "api-b.cfm")

	if _, reason := bareCall(t, r, mixin, "getService"); reason != "" {
		t.Fatalf("before the edit: %s", reason)
	}

	api := filepath.Join(dir, "api.cfc")
	edited := `<cfcomponent extends="Base">
	<cffunction name="getService"></cffunction>
	<cfinclude template="api-a.cfm">
</cfcomponent>`

	if err := os.WriteFile(api, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}

	r.Index.IndexFile(cfpath.ToURI(api), edited)

	if _, reason := bareCall(t, r, mixin, "getService"); reason == "" {
		t.Error("api-b.cfm still resolves through api.cfc after api.cfc stopped including it")
	}
}
