package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
)

func TestFactoryWrapperReturnsTheFinalComponent(t *testing.T) {
	r, dir := writeDIApp(t, `component extends="framework.one" {}`, "")

	sources := map[string]string{
		"Builder": `component {
 function configure() { return this; }
 function onlyBuilder() {}
 model.beans.Product function build() {}
 }`,
		"Product": `component { function work() {} }`,
		"Wrapper": `component {
 function make() { return getBean('Builder').configure().build(); }
 }`,
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, "model", "beans", name+".cfc"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Refresh the supplied bean roots after adding the fixture components.
	r.Index.SetBeans(cfpath.BuildBeanMap(r.BeanPaths, r.FS))
	r.Resolvers = r.BeanResolvers(r.BeanPaths)
	wrapper := filepath.Join(dir, "model", "beans", "Wrapper.cfc")
	r.Index.IndexFileWithOptions(cfpath.ToURI(wrapper), sources["Wrapper"], &parser.ParseOptions{Resolvers: r.Resolvers, FuncLookup: r.FuncLookup(filepath.Dir(wrapper))})

	consumer := filepath.Join(dir, "Consumer.cfc")
	pr := parser.ParseWithOptions(cfpath.ToURI(consumer), `component {
 function run() {
  var wrapper=new model.beans.Wrapper();
  wrapper.make().work();
  wrapper.make().onlyBuilder();
 }
}`, &parser.ParseOptions{ExtractCalls: true, FuncLookup: r.FuncLookup(dir)})
	seen := 0

	for _, call := range pr.AllCalls() {
		if call.FuncName != "work" && call.FuncName != "onlyBuilder" {
			continue
		}

		seen++
		reason := r.CanResolveCall(&call, pr, dir)

		if call.FuncName == "work" && reason != "" {
			t.Errorf("product method unresolved: %s", reason)
		}

		if call.FuncName == "onlyBuilder" && reason == "" {
			t.Error("builder method incorrectly resolved on returned product")
		}
	}

	if seen != 2 {
		t.Fatalf("checked %d calls, want 2", seen)
	}
}
