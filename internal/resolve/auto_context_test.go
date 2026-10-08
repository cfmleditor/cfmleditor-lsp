package resolve_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/vfs"
)

func TestStartupServiceLoopResolvesVerifiedBeans(t *testing.T) {
	for _, tc := range []struct {
		name, startup string
		known         bool
	}{
		{"known services", `variables.factory=new framework.ioc('model');
 application.factory=variables.factory;
 variables.services="reporter,missing";
 for(variables.i in listToArray(variables.services)){application["#variables.i#"]=application.factory.getBean("#variables.i#");}`, true},
		{"registered alias after bootstrap", `variables.factory=new framework.ioc('model');
 variables.factory.addAlias("reporterAlias","Reporter");
 for(i in listToArray("reporterAlias")){application["#i#"]=variables.factory.getBean("#i#");}
 application.reporter=application.reporterAlias;`, true},
		{"custom factory", `variables.factory=new model.services.Reporter();for(i in listToArray("reporter")){application["#i#"]=variables.factory.getBean("#i#");}`, false},
		{"unknown factory", `variables.factory=arguments.factory;for(i in listToArray("reporter")){application["#i#"]=variables.factory.getBean("#i#");}`, false},
		{"factory overwrite", `variables.factory=new framework.ioc('model');variables.factory=arguments.value;for(i in listToArray("reporter")){application["#i#"]=variables.factory.getBean("#i#");}`, false},
		{"unknown overwrite", `variables.factory=new framework.ioc('model');for(i in listToArray("reporter")){application["#i#"]=variables.factory.getBean("#i#");}
 application.reporter=arguments.value;`, false},
		{"conflicting overwrite", `variables.factory=new framework.ioc('model');for(i in listToArray("reporter")){application["#i#"]=variables.factory.getBean("#i#");}
 application.reporter=new model.beans.User();`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, dir := writeDIApp(t, `component { include "startup.cfm"; }`, tc.startup)

			got := reasonsFor(t, r, filepath.Join(dir, "page.cfm"), `<cfscript>application.reporter.run();application.reporter.nope();application.missing.run();</cfscript>`)
			if (got["application.reporter.run"] == "") != tc.known {
				t.Fatalf("results=%v", got)
			}

			if got["application.reporter.nope"] == "" || got["application.missing.run"] == "" {
				t.Fatalf("unchecked methods/beans: %v", got)
			}
		})
	}
}

func TestRecordReceiversPreserveScopeAndMissingMethods(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"Scope.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
	src := `component {
 variables.$=new Other();
 variables.rc.$=new Scope();
 function known(rc){rc.$=new Scope();rc.$.work();rc.$.other();}
 function unrelated(data){arguments.data.$.other();}
 function shadow(){var rc={};rc.$.work();}
 function scoped(rc){variables.rc.$.work();}
 }`
	file := filepath.Join(dir, "Consumer.cfc")

	pr := parser.ParseWithOptions(cfpath.ToURI(file), src, &parser.ParseOptions{ExtractCalls: true, FuncLookup: r.FuncLookup(filepath.Dir(file))})
	for _, c := range pr.AllCalls() {
		if c.FuncName != "work" && c.FuncName != "other" {
			continue
		}

		why := r.CanResolveCall(&c, pr, dir)

		known := (c.Caller == "known" && c.FuncName == "work") || c.Caller == "scoped"
		if (why == "") != known {
			t.Errorf("%s:%s.%s: %q", c.Caller, c.Variable, c.FuncName, why)
		}
	}
	// A body edit must recompute the member, rather than borrow either field.
	pr.ApplyFullReplace(strings.ReplaceAll(src, "rc.$=new Scope();rc.$.work();", "rc.$=arguments.value;rc.$.work();"))

	for _, c := range pr.AllCalls() {
		if c.Caller == "known" && c.FuncName == "work" && r.CanResolveCall(&c, pr, dir) == "" {
			t.Error("stale member after replacement")
		}
	}
}

func TestCFConfigWithoutApplicationAndExplicitPrecedence(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"server", "editor"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, name, "User.cfc"), []byte(`component {}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, ".cfconfig.json"), []byte(`{"mappings":{"/models":{"physical":"server"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfpath.InvalidateAppMappingsCache()

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
	if got := r.ComponentPath("models.User", dir); got != filepath.Join(dir, "server", "User.cfc") {
		t.Fatalf("server mapping=%q", got)
	}

	explicit := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}, Mappings: map[string]string{"MODELS": filepath.Join(dir, "editor")}}
	if got := explicit.ComponentPath("models.User", dir); got != filepath.Join(dir, "editor", "User.cfc") {
		t.Fatalf("explicit mapping=%q", got)
	}
}
