package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const fw1InjectionSource = `component {
 function setBeanFactory(factory) {}
 function getBeanFactory() {}
 function getController() {}
 function getService() {}
 function autowire(cfc, beanFactory) {}
 function getCachedComponent() { autowire(cfc, getBeanFactory()); }
}`

func TestFW1SetterScopeRequiresApplicationWiring(t *testing.T) {
	for _, test := range []struct {
		name, config, setup, framework, path string
		managed                              bool
	}{
		{name: "controller", path: "controllers/Consumer.cfc", managed: true},
		{name: "service", path: "services/Consumer.cfc", managed: true},
		{name: "foreign settings ignored", config: `other.framework.usingSubsystems=true;`, path: "controllers/Consumer.cfc", managed: true},
		{name: "subsystem controller", config: "variables.framework.usingSubsystems=true;", path: "core/controllers/Consumer.cfc", managed: true},
		{name: "subsystem service", config: "variables.framework.usingSubsystems=true;", path: "core/services/Consumer.cfc", managed: true},
		{name: "wrong subsystem depth", config: "variables.framework.usingSubsystems=true;", path: "controllers/Consumer.cfc"},
		{name: "unconfigured subsystem", path: "core/controllers/Consumer.cfc"},
		{name: "nested directory", path: "controllers/nested/Consumer.cfc"},
		{name: "model", path: "models/Consumer.cfc"},
		{name: "view", path: "views/Consumer.cfc"},
		{name: "template", path: "controllers/Consumer.cfm"},
		{name: "missing wiring", setup: `other.setBeanFactory(factory);`, path: "controllers/Consumer.cfc"},
		{name: "comments and strings", setup: `/* setBeanFactory(factory); */ var example="setBeanFactory(factory)";`, path: "controllers/Consumer.cfc"},
		{name: "method stubs", framework: `component { function setBeanFactory(factory) {} function getBeanFactory() {} function getController() {} function autowire(cfc, factory) {} }`, path: "controllers/Consumer.cfc"},
		{name: "foreign base", config: `variables.framework.base="/elsewhere/";`, path: "controllers/Consumer.cfc"},
		{name: "mapped base", config: `variables.framework.base="/app/";`, path: "controllers/Consumer.cfc", managed: true},
		{name: "runtime base only", config: `variables.framework.base="/#request.folder#/";`, path: "controllers/Consumer.cfc"},
		{name: "default with runtime override", config: `variables.framework.base="/app/"; variables.framework.base="/#request.folder#/";`, path: "controllers/Consumer.cfc", managed: true},
		{name: "conflicting bases", config: `variables.framework.base="/app/"; variables.framework.base="/elsewhere/";`, path: "controllers/Consumer.cfc"},
		{name: "computed flag", config: `variables.framework.usingSubsystems=true && false;`, path: "core/controllers/Consumer.cfc"},
		{name: "conflicting flag", config: `variables.framework.usingSubsystems=true; variables.framework.usingSubsystems=false;`, path: "core/controllers/Consumer.cfc"},
		{name: "struct config", config: `variables.framework={usingSubsystems=true};`, path: "core/controllers/Consumer.cfc"},
		{name: "implicit subsystem", config: `variables.framework.defaultSubsystem="core";`, path: "controllers/Consumer.cfc"},
		{name: "custom controllers", config: `variables.framework.controllersFolder="actions";`, path: "controllers/Consumer.cfc"},
		{name: "custom subsystem folder", config: `variables.framework.subsystemsFolder="modules"; variables.framework.usingSubsystems=true;`, path: "core/controllers/Consumer.cfc"},
		{name: "subsystem factory", setup: `setBeanFactory(factory); setSubsystemBeanFactory("core", other);`, path: "controllers/Consumer.cfc"},
		{name: "overridden factory method", config: `function setBeanFactory(factory) {}`, path: "controllers/Consumer.cfc"},
		{name: "modern controller", framework: `component { function setBeanFactory(factory) {} function getBeanFactory() {} function getController() {} function autowire(cfc, factory) {} function getCachedController() { autowire(cfc, getBeanFactory()); } }`, path: "controllers/Consumer.cfc", managed: true},
		{name: "modern service", framework: `component { function setBeanFactory(factory) {} function getBeanFactory() {} function getController() {} function autowire(cfc, factory) {} function getCachedController() { autowire(cfc, getBeanFactory()); } }`, path: "services/Consumer.cfc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()

			framework := test.framework
			if framework == "" {
				framework = fw1InjectionSource
			}

			setup := test.setup
			if setup == "" {
				setup = "setBeanFactory(factory);"
			}

			files := map[string]string{
				"Application.cfc":   `component extends="framework" { ` + test.config + ` function setupApplication() { ` + setup + ` } }`,
				"framework.cfc":     framework,
				"beans/Service.cfc": `component { function run() {} }`,
				test.path: `component accessors=true {
 property name="dependency";
 function setService(service) { variables.dependency=arguments.service; }
}`,
			}
			for name, source := range files {
				file := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}, Mappings: map[string]string{"app": dir}, BeanPaths: map[string]string{"": filepath.Join(dir, "beans")}}
			dep := filepath.Join(dir, "beans", "Service.cfc")
			r.Index.SetBeans(map[string]string{"service": dep})

			file := filepath.Join(dir, test.path)

			lookup := r.SetterLookup(file)
			if (lookup != nil) != test.managed {
				t.Fatalf("managed=%v, want %v", lookup != nil, test.managed)
			}

			if test.managed {
				if lookup("service") != dep || lookup("unknown") != "" {
					t.Fatal("factory lookup lost its dependency or invented an unknown bean")
				}

				getter := r.ResolveFunc(file, "getDependency", dir)
				if getter == nil || getter.ReturnComponent != dep {
					t.Fatalf("lazy getter lost injection: %+v", getter)
				}
			}
			// A nearer Application ends the enclosing FW/1 scope.
			nested := filepath.Join(dir, "nested")
			if err := os.MkdirAll(nested, 0o755); err != nil {
				t.Fatal(err)
			}

			if err := os.WriteFile(filepath.Join(nested, "Application.cfc"), []byte(`component {}`), 0o600); err != nil {
				t.Fatal(err)
			}

			if r.SetterLookup(filepath.Join(nested, "controllers", "Other.cfc")) != nil {
				t.Fatal("nested application inherited FW/1 injection")
			}
		})
	}
}
