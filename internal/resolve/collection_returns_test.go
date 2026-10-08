package resolve_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

func TestClosedCollectionReturnDependencies(t *testing.T) {
	r, dir := writeDIApp(t, `component extends="framework.one" {}`, "")

	sources := map[string]string{
		"Producer": `component {
 model.beans.Site function read() {}
 }`,
		"Site": `component { function name() {} }`,
		"Manager": `component {
 variables.items=structNew();
 variables.producer=new model.beans.Producer();
 function populate() {
  var built=structNew();
  built[key]=variables.items[key];
  built[other]=variables.producer.read();
  variables.items=built;
 }
 function one(key) { return variables.items[key]; }
 function all() { return variables.items; }
 }`,
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, "model", "beans", name+".cfc"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	manager := filepath.Join(dir, "model", "beans", "Manager.cfc")
	defs := r.EnsureIndexed(manager)

	var getter *parser.FunctionDef

	for _, f := range defs {
		if f.Name == "one" {
			getter = f
		}

		if f.Name == "all" && (f.ReturnComponent != "" || len(f.ReturnSources) > 0) {
			t.Fatal("whole struct typed as element")
		}
	}

	if getter == nil || len(getter.ReturnSources) == 0 {
		t.Fatal("closed indexing lost return dependencies")
	}

	want := filepath.Join(dir, "model", "beans", "Site.cfc")
	if got := r.ReturnComponentOf(getter); got != want {
		t.Fatalf("closed return=%q want=%q", got, want)
	}

	page := filepath.Join(dir, "Page.cfc")
	pr := parser.ParseWithOptions(cfpath.ToURI(page), `component {
 function run() {
  var manager=new model.beans.Manager();
  manager.one('key').name();
  manager.one('key').notASiteMethod();
 }
}`, &parser.ParseOptions{ExtractCalls: true, FuncLookup: r.FuncLookup(dir)})
	checked := 0

	for _, call := range pr.AllCalls() {
		if call.FuncName != "name" && call.FuncName != "notASiteMethod" {
			continue
		}

		checked++

		reason := r.CanResolveCall(&call, pr, dir)
		if call.FuncName == "name" && reason != "" {
			t.Errorf("name unresolved: %s", reason)
		}

		if call.FuncName == "notASiteMethod" && !strings.Contains(reason, "notASiteMethod") {
			t.Errorf("missing method lost: %q", reason)
		}
	}

	if checked != 2 {
		t.Fatalf("checked %d calls", checked)
	}
	// A dependency update must affect the existing getter immediately, without
	// mutating its definition or relying on a cached inferred return.
	producer := filepath.Join(dir, "model", "beans", "Producer.cfc")
	r.Index.IndexFile(cfpath.ToURI(producer), `component { function read() {} }`)

	if got := r.ReturnComponentOf(getter); got != "" {
		t.Fatalf("stale dependency type: %q", got)
	}
}

func TestClosedCollectionContractsRejectConflictsAndCycles(t *testing.T) {
	r, dir := writeDIApp(t, `component extends="framework.one" {}`, "")
	file := filepath.Join(dir, "model", "beans", "Manager.cfc")

	site := filepath.Join(dir, "model", "beans", "Site.cfc")
	for name, source := range map[string]string{
		"Site":     `component { function name() {} }`,
		"Other":    `component { function other() {} }`,
		"Producer": `component { model.beans.Site function read() {} model.beans.Other function other() {} query function queryResult() {} }`,
	} {
		if err := os.WriteFile(filepath.Join(dir, "model", "beans", name+".cfc"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct{ name, writes, declared, want string }{
		{"canonical agreeing sources", `variables.items[a]=new model.beans.Site(); variables.items[b]=variables.producer.read();`, "", site},
		{"different producer returns", `variables.items[a]=variables.producer.read(); variables.items[b]=variables.producer.other();`, "", ""},
		{"primitive producer return", `variables.items[a]=variables.producer.queryResult();`, "", ""},
		{"declared component priority", `variables.items[a]=new model.beans.Other();`, "model.beans.Site", site},
		{"declared primitive priority", `variables.items[a]=new model.beans.Site();`, "query", ""},
		{"recursive collection return", `variables.items[a]=one();`, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r.Index.IndexFile(cfpath.ToURI(file), `component {
    variables.items=structNew(); variables.producer=new model.beans.Producer();
    function fill() { `+tc.writes+` }
    `+tc.declared+` function one() { return variables.items[key]; }
   }`)

			getter := r.LookupFuncWithExtends(file, "one")
			if getter == nil {
				t.Fatal("getter missing")
			}

			got := r.ReturnComponentOf(getter)
			if path := r.ComponentPath(got, dir); path != "" {
				got = path
			}

			if got != tc.want {
				t.Fatalf("return=%q want=%q", got, tc.want)
			}
		})
	}
}
