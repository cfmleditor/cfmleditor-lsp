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

func TestSetterLookupIsLimitedToManagedComponents(t *testing.T) {
	dir := t.TempDir()

	beans := filepath.Join(dir, "beans")
	if err := os.MkdirAll(beans, 0o755); err != nil {
		t.Fatal(err)
	}

	dep := filepath.Join(beans, "Service.cfc")
	if err := os.WriteFile(dep, []byte(`component { function run() {} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	consumer := filepath.Join(beans, "Consumer.cfc")

	source := `component accessors=true {
  property name="dependency";
  function setAlias(alias) { variables.dependency = arguments.alias; }
 }`
	if err := os.WriteFile(consumer, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), BeanPaths: map[string]string{"": beans}, WorkspaceFolders: []string{dir}, Resolvers: []parser.Resolver{{Match: `getBean("alias")`, Resolve: dep, Prefix: "getBean"}}}
	r.Index.SetBeans(map[string]string{"service": dep})

	lookup := r.SetterLookup(consumer)
	if lookup == nil || lookup("alias") != dep || lookup("service") != dep {
		t.Fatal("managed bean lookup did not recognize dependency")
	}

	for _, file := range []string{filepath.Join(dir, "Manual.cfc"), filepath.Join(dir, "beans-extra", "Other.cfc"), filepath.Join(beans, "page.cfm")} {
		if r.SetterLookup(file) != nil {
			t.Errorf("unmanaged file acquired injection: %s", file)
		}
	}

	getter := r.ResolveFunc("beans.Consumer", "getDependency", dir)
	if getter == nil || getter.ReturnComponent != dep {
		t.Fatalf("lazy index lost injection type: %+v", getter)
	}

	ref := r.Index.LookupComponentRefInFile("dependency", cfpath.ToURI(consumer), 100)
	if ref == nil || ref.Component != dep {
		t.Fatalf("index lost setter field: %+v", ref)
	}
	// An explicit dynamic factory rule must not fall back to a same-named CFC.
	r.Resolvers = []parser.Resolver{{Match: `getBean("service")`, Resolve: "$any", Prefix: "getBean"}}
	if got := r.SetterLookup(consumer)("service"); got != "" {
		t.Errorf("dynamic override inferred %s", got)
	}
}
