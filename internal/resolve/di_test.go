package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/uri"

	"github.com/cfmleditor/clif/internal/index"
	"github.com/cfmleditor/clif/internal/parser"
	cfpath "github.com/cfmleditor/clif/internal/path"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/vfs"
)

func writeDIApp(t *testing.T, app, startup string) (*resolve.Resolver, string) {
	t.Helper()
	dir := t.TempDir()

	sources := map[string]string{
		"Application.cfc": app,
		"startup.cfm":     startup,
		"framework/ioc.cfc": `component {
 function init(folders,config={}) {}
 function getBean(beanName) {}
 function isSingleton(beanName) {}
 function beanIsTransient(singleDir,dir,beanName) {}
 function findSetters(cfc,iocMeta) { isSingleton("name"); }
 function declareBean(beanName,dottedPath,isSingleton=true) {}
 function addAlias(aliasName,beanName) {}
 function addBean(beanName,beanValue) {}
 }`,
		"model/beans/User.cfc":        `component { function run() {} }`,
		"model/services/Reporter.cfc": `component { function run() {} }`,
		"model/jobs/Job.cfc":          `component { function run() {} }`,
		"model/entities/Entity.cfc":   `component { function run() {} }`,
		"model/services/Consumer.cfc": `component accessors=true {
 property name="user";
 property name="reporter";
 function setUser(user) { variables.user=arguments.user; }
 function setReporter(reporter) { variables.reporter=arguments.reporter; }
 function use() { variables.user.run(); variables.reporter.run(); }
 }`,
	}
	for name, source := range sources {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}, BeanPaths: map[string]string{"": filepath.Join(dir, "model")}, StartupFiles: []string{filepath.Join(dir, "startup.cfm")}, Resolvers: []parser.Resolver{{Match: `getBean("$1")`, Resolve: "$1", Prefix: "getBean"}}}
	r.Index.SetBeans(cfpath.BuildBeanMap(r.BeanPaths, r.FS))
	r.Resolvers = r.BeanResolvers(r.BeanPaths)

	return r, dir
}

func TestDI1InjectionUsesLifetimeAndConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, config string
		blocked      []string
	}{
		{"defaults", `{}`, []string{"user", "userBean"}},
		{"folder transients", `{transients:["services"]}`, []string{"user", "userBean", "reporter", "reporterService"}},
		{"pattern", `{transientPattern:"Job$"}`, []string{"user", "userBean", "job"}},
		{"singleton pattern", `{singletonPattern:"Reporter$"}`, []string{"user", "userBean", "job", "entity"}},
		{"singular mapping", `{singulars:{entities:"bean"}}`, []string{"user", "userBean", "entity"}},
		{"constant shadow", `{constants:{reporter:"not a CFC"}}`, []string{"user", "userBean", "reporter"}},
		{"aliases omitted", `{omitDirectoryAliases:true}`, []string{"user", "userBean", "reporterService"}},
		{"dynamic pattern", `{transientPattern:settings.pattern()}`, []string{"user", "userBean", "reporter", "reporterService", "job", "entity"}},
		{"unsupported regex", `{transientPattern:"(?<=prefix)Job"}`, []string{"user", "userBean", "reporter", "reporterService", "job", "entity"}},
		{"conflicting patterns", `{transientPattern:"Job$",singletonPattern:"Reporter$"}`, []string{"user", "userBean", "reporter", "reporterService", "job", "entity"}},
		{"literal exclusion", `{exclude:["/services/"]}`, []string{"user", "userBean", "reporter", "reporterService", "job", "entity"}},
		{"nonrecursive", `{recurse:false}`, []string{"user", "userBean", "reporter", "reporterService", "job", "entity"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, dir := writeDIApp(t, `component extends="framework.one" { variables.framework={diConfig:`+test.config+`}; }`, "")
			file := filepath.Join(dir, "model", "services", "Consumer.cfc")

			lookup := r.SetterLookup(file)
			if lookup == nil {
				t.Fatal("missing managed lookup")
			}

			blocked := map[string]bool{}
			for _, name := range test.blocked {
				blocked[name] = true
			}

			for _, name := range []string{"user", "userBean", "reporter", "reporterService", "job", "entity"} {
				got := lookup(name)

				want := r.BeanLookup(name)
				if blocked[name] {
					want = ""
				}

				if got != want {
					t.Errorf("%s: %q, want %q", name, got, want)
				}
			}
			// Direct bean retrieval continues to resolve transients.
			if r.BeanLookup("user") == "" {
				t.Fatal("transient retrieval lost its identity")
			}
		})
	}
}

func TestDI1ExplicitRegistrationLifetimeAndAliases(t *testing.T) {
	startup := `<cfscript>
 f=new framework.ioc("/model",{transientPattern:"User$"});
 f.declareBean("forced", "model.beans.User", true);
 f.addAlias("aliasForced","forced");
 f.declareBean(beanName="never",dottedPath="model.services.Reporter",isSingleton=false);
 f.addAlias("aliasNever","never");
 f.declareBean("uncertain","model.services.Reporter",settings.singleton);
 f.addAlias("cycleA","cycleB"); f.addAlias("cycleB","cycleA");
 f.addBean("reporter","a string");
 alternate=new model.jobs.Job(); f.addBean("jobSingleton",alternate);
 </cfscript>`
	r, dir := writeDIApp(t, `component {}`, startup)

	lookup := r.SetterLookup(filepath.Join(dir, "model", "services", "Consumer.cfc"))
	for name, want := range map[string]string{"forced": filepath.Join(dir, "model", "beans", "User.cfc"), "aliasForced": filepath.Join(dir, "model", "beans", "User.cfc"), "jobSingleton": filepath.Join(dir, "model", "jobs", "Job.cfc"), "never": "", "aliasNever": "", "uncertain": "", "cycleA": "", "reporter": ""} {
		if got := lookup(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
	// The older FW/1 loader injects any contained bean, not just singletons.
	admin := filepath.Join(dir, "admin")
	if err := os.MkdirAll(admin, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(admin, "Application.cfc"), []byte(`component extends="framework" { function setupApplication() {setBeanFactory(factory);} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(admin, "framework.cfc"), []byte(fw1InjectionSource), 0o600); err != nil {
		t.Fatal(err)
	}

	fallback := r.SetterLookup(filepath.Join(admin, "controllers", "Controller.cfc"))
	if fallback == nil || fallback("user") != r.BeanLookup("user") {
		t.Fatal("framework fallback acquired DI/1 transient pruning")
	}
}

func TestDI1PropertyAndIndexedSetterTypes(t *testing.T) {
	r, dir := writeDIApp(t, `component extends="framework.one" {}`, "")
	consumer := filepath.Join(dir, "model", "services", "Consumer.cfc")

	defs := r.EnsureIndexed(consumer)
	for _, fn := range defs {
		if fn.Name == "setUser" && fn.Arguments[0].Component != "" {
			t.Fatal("transient setter typed")
		}

		if fn.Name == "setReporter" && fn.Arguments[0].Component == "" {
			t.Fatal("singleton setter not typed")
		}

		if fn.Name == "getUser" && fn.ReturnComponent != "" {
			t.Fatal("transient property getter typed")
		}

		if fn.Name == "getReporter" && fn.ReturnComponent == "" {
			t.Fatal("singleton property getter not typed")
		}
	}

	property := r.InjectionPropertyLookup(consumer)
	for _, attrs := range []map[string]string{{"default": ""}, {"type": "string"}, {"setter": "false"}} {
		if property("reporter", attrs) != "" {
			t.Errorf("ineligible property typed: %v", attrs)
		}
	}

	if property("reporter", map[string]string{"type": "any"}) == "" {
		t.Fatal("any property incorrectly omitted")
	}

	overridden, _ := writeDIApp(t, `component extends="framework.one" { variables.framework={diConfig:{omitTypedProperties:false,omitDefaultedProperties:false}}; }`, "")

	getter := overridden.InjectionPropertyLookup(filepath.Join(overridden.WorkspaceFolders[0], "model", "services", "Consumer.cfc"))
	if getter("reporter", map[string]string{"type": "string", "default": ""}) == "" {
		t.Fatal("explicit property policy ignored")
	}
}

func TestDI1PropertyMetadataControlsExplicitSetters(t *testing.T) {
	for _, test := range []struct {
		name, attrs, componentAttrs string
		injected                    bool
	}{
		{"default", `default=""`, `accessors=true`, false},
		{"typed", `type="string"`, `accessors=true`, false},
		{"any", `type="any"`, `accessors=true`, true},
		{"disabled implicit", `default="" setter=false`, `accessors=true`, true},
		{"no accessors", `default=""`, ``, true},
		{"persistent", `default=""`, `persistent=true`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, dir := writeDIApp(t, `component extends="framework.one" {}`, "")
			file := filepath.Join(dir, "model", "services", "Consumer.cfc")
			source := `component ` + test.componentAttrs + ` {
 function setReporter(reporter) { variables.reporter=arguments.reporter; }
 property name="reporter" ` + test.attrs + `;
 }`

			pr := parser.ParseWithOptions(uri.File(file), source, &parser.ParseOptions{SetterLookup: r.SetterLookup(file), BeanLookup: r.InjectionBeanLookup(file), PropertyBeanLookup: r.InjectionPropertyLookup(file)})
			for _, fn := range pr.Funcs {
				if fn.Name == "setReporter" && (fn.Arguments[0].Component != "") != test.injected {
					t.Fatalf("setter: %+v", fn)
				}
			}

			pr.ApplyFullReplace(`component accessors=true { property name="reporter"; function setReporter(reporter) {variables.reporter=arguments.reporter;} }`)

			for _, fn := range pr.Funcs {
				if fn.Name == "setReporter" && fn.Arguments[0].Component == "" {
					t.Fatal("property edit did not refresh injection")
				}
			}

			pr.ApplyFullReplace(source)

			for _, fn := range pr.Funcs {
				if fn.Name == "setReporter" && (fn.Arguments[0].Component != "") != test.injected {
					t.Fatal("property edit retained stale injection")
				}
			}
		})
	}
}
