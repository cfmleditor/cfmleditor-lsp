package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/vfs"
)

func TestCallerMappingsFollowLibraryContracts(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()

		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	library := filepath.Join(root, "library")

	write("library/Application.cfc", `component { function onRequestStart() { abort; } }`)
	write("library/Child.cfc", `component extends="api.Base" {}`)
	write("library/Factory.cfc", `component { /** @return api.Result */ function make() {} }`)
	write("library/Forward.cfc", `component { variables.items=structNew(); variables.factory=new lib.Factory(); function populate(key) { variables.items[key]=variables.factory.make(); } function make(key) { return variables.items[key]; } }`)

	for _, app := range []string{"a", "b"} {
		write(app+"/Application.cfc", `component { this.mappings["/api"] = expandPath("./api"); this.mappings["/lib"] = "`+filepath.ToSlash(library)+`"; }`)
		write(app+"/api/Base.cfc", `component { function only`+app+`() {} }`)
		write(app+"/api/Result.cfc", `component { function result`+app+`() {} }`)
	}

	write("a/nested/Application.cfc", `component {}`)

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{root}}
	// Repeat in the opposite order: neither lookup nor indexed source metadata
	// may depend on the first caller of a shared physical library component.
	for _, app := range []string{"a", "b", "b", "a"} {
		dir := filepath.Join(root, app)

		other := "a"
		if app == "a" {
			other = "b"
		}

		if got := r.ResolveFunc("lib.Child", "only"+app, dir); got == nil {
			t.Fatalf("%s: caller's mapped inherited method did not resolve", app)
		}

		if got := r.ResolveFunc("lib.Child", "only"+other, dir); got != nil {
			t.Fatalf("%s: borrowed another application's inherited method", app)
		}

		for _, component := range []string{"lib.Factory", "lib.Forward"} {
			got := r.FuncLookup(dir)(component, "make")

			want := filepath.Join(dir, "api", "Result.cfc")
			if got != want {
				t.Errorf("%s %s: returned %q, want %q", app, component, got, want)
			}
		}

		page := filepath.Join(dir, "Page.cfc")
		pr := parser.ParseWithOptions(cfpath.ToURI(page), `component { function run() {
 var child=new lib.Child(); child.only`+app+`(); child.only`+other+`();
 var factory=new lib.Factory(); factory.make().result`+app+`(); factory.make().result`+other+`();
 } }`, &parser.ParseOptions{ExtractCalls: true})
		checked := 0

		for _, call := range pr.AllCalls() {
			if call.FuncName != "only"+app && call.FuncName != "only"+other && call.FuncName != "result"+app && call.FuncName != "result"+other {
				continue
			}

			checked++
			wantResolved := call.FuncName == "only"+app || call.FuncName == "result"+app

			reason := r.CanResolveCall(&call, pr, dir)
			if (reason == "") != wantResolved {
				t.Errorf("%s %s: verdict %q, want resolved=%v", app, call.FuncName, reason, wantResolved)
			}

			explained, _ := r.ExplainCall(&call, pr, dir)

			target, targeted := r.ResolveCallTarget(&call, pr, dir)
			if explained != reason || targeted != reason {
				t.Error("call resolution, explanation and target disagree")
			}

			if wantResolved && target.URI == "" {
				t.Error("resolved call lost its definition target")
			}
		}

		if checked != 4 {
			t.Fatalf("checked %d caller calls, want 4", checked)
		}
	}

	if got := r.ResolveFunc(filepath.Join(library, "Child.cfc"), "onlya", filepath.Join(root, "a", "nested")); got != nil {
		t.Fatal("nested application inherited outer application's mappings")
	}

	if got := r.FuncLookup(library)("Factory", "make"); got == filepath.Join(root, "a", "api", "Result.cfc") || got == filepath.Join(root, "b", "api", "Result.cfc") {
		t.Fatal("direct library lookup borrowed a caller's mapping context")
	}

	for _, def := range r.EnsureIndexed(filepath.Join(library, "Factory.cfc")) {
		if def.Name == "make" && (def.DocReturn != "api.Result" || def.ReturnComponent != "") {
			t.Fatal("shared indexed contract was rewritten for a caller")
		}
	}
	// Explicit editor mappings win over discovered mappings, and rebuilding the
	// resolver after a mapping edit refreshes views while reusing the source index.
	overridden := &resolve.Resolver{FS: vfs.OS{}, Index: r.Index, WorkspaceFolders: []string{root}, Mappings: map[string]string{"API": filepath.Join(root, "b", "api")}}
	if overridden.ResolveFunc("lib.Child", "onlyb", filepath.Join(root, "a")) == nil {
		t.Fatal("caller view lost explicit mapping precedence")
	}

	write("a/Application.cfc", `component { this.mappings["/api"] = "`+filepath.ToSlash(filepath.Join(root, "b", "api"))+`"; this.mappings["/lib"] = "`+filepath.ToSlash(library)+`"; }`)
	cfpath.InvalidateAppMappingsCache()

	refreshed := &resolve.Resolver{FS: vfs.OS{}, Index: r.Index, WorkspaceFolders: []string{root}}
	if refreshed.ResolveFunc("lib.Child", "onlyb", filepath.Join(root, "a")) == nil || refreshed.ResolveFunc("lib.Child", "onlya", filepath.Join(root, "a")) != nil {
		t.Fatal("recreated caller view retained stale mappings")
	}
}
