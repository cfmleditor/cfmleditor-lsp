package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

func TestDI1ConstructorDependenciesAndLazyGetters(t *testing.T) {
	for _, source := range []string{
		`component accessors=true {
 property name="dependency";
 function init(required user, any reporter, string label, absent="") {
 variables.dependency=arguments.user;
 variables.reporter=arguments.reporter;
 return this;
 }
 function setUser(user) {variables.user=arguments.user;}
 function sibling(user) {user.run();}
 }`,
		`<cfcomponent accessors="true"><cfproperty name="dependency">
 <cffunction name="init"><cfargument name="user" required="true"><cfargument name="reporter" type="any"><cfargument name="label" type="string"><cfargument name="absent" default="">
 <cfset variables.dependency=arguments.user><cfset variables.reporter=arguments.reporter><cfreturn this></cffunction>
 <cffunction name="setUser"><cfargument name="user"><cfset variables.user=arguments.user></cffunction>
 <cffunction name="sibling"><cfargument name="user"><cfset user.run()></cffunction></cfcomponent>`,
		`<cfcomponent accessors="true"><cfproperty name="dependency"><cfscript>
 function init(required user,any reporter,string label,absent="") {variables.dependency=arguments.user;variables.reporter=arguments.reporter;return this;}
 function setUser(user) {variables.user=arguments.user;}
 function sibling(user) {user.run();}
 </cfscript></cfcomponent>`,
	} {
		r, dir := writeDIApp(t, `component extends="framework.one" {}`, "")

		file := filepath.Join(dir, "model", "services", "Consumer.cfc")
		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		indexed := r.EnsureIndexed(file)

		closed := make([]parser.FunctionDef, len(indexed))
		for i, fn := range indexed {
			closed[i] = *fn
		}

		for _, defs := range [][]parser.FunctionDef{
			closed,
			parser.ParseWithOptions(cfpath.ToURI(file), source, &parser.ParseOptions{SetterLookup: r.SetterLookup(file), ConstructorLookup: r.ConstructorLookup(file), BeanLookup: r.InjectionBeanLookup(file), PropertyBeanLookup: r.InjectionPropertyLookup(file)}).Funcs,
		} {
			found := map[string]bool{}

			for _, fn := range defs {
				switch fn.Name {
				case "init":
					found[fn.Name] = true
					for i, arg := range fn.Arguments {
						if (arg.Component != "") != (i < 2) {
							t.Fatalf("constructor argument: %+v", arg)
						}

						if i == 0 && arg.Type != "" {
							t.Fatalf("declared signature modified: %+v", arg)
						}
					}
				case "getDependency":
					found[fn.Name] = true
					if fn.ReturnComponent != r.BeanLookup("user") {
						t.Fatalf("transient constructor field lost: %+v", fn)
					}
				case "setUser", "sibling":
					if fn.Arguments[0].Component != "" {
						t.Fatalf("constructor inference escaped its method: %+v", fn)
					}
				}
			}

			if !found["init"] || !found["getDependency"] {
				t.Fatalf("missing indexed signatures: %v", found)
			}
		}
	}
}

func TestDI1ConstructorContractsAndOverrides(t *testing.T) {
	for _, test := range []struct {
		name, startup string
		blocked       bool
	}{
		{"default", "", false},
		{"literal override", `f.declareBean("consumer","model.services.Consumer",true,{user:"runtime string"});`, true},
		{"dynamic overrides", `f.declareBean("consumer","model.services.Consumer",true,settings.overrides);`, true},
		{"getBean override", `f.getBean("consumer",{user:"runtime string"});`, true},
		{"empty registration overrides", `f.declareBean("consumer","model.services.Consumer",true,{});`, false},
		{"empty getBean overrides", `f.getBean("consumer",{});`, false},
		{"named getBean override", `f.getBean(constructorArgs={user:"runtime string"},beanName="consumer");`, true},
		{"registered primitive", `f.addBean("consumer","runtime value");`, true},
		{"registered instance", `instance=new model.services.Consumer();f.addBean("consumer",instance);`, true},
		{"getBean dynamic", `f.getBean("consumer",settings.args);`, true},
		{"other argument", `f.declareBean("consumer","model.services.Consumer",true,{reporter:"runtime string"});`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			r, dir := writeDIApp(t, `component {}`, `<cfscript>f=new framework.ioc("/model");`+test.startup+`</cfscript>`)
			file := filepath.Join(dir, "model", "services", "Consumer.cfc")

			lookup := r.ConstructorLookup(file)
			if lookup == nil {
				t.Fatal("missing constructor policy")
			}

			if (lookup("user") == "") != test.blocked {
				t.Fatalf("override policy mismatch: %q", lookup("user"))
			}

			source := `component {
 /** @param {model.jobs.Job} user */
 function init(user) {variables.user=arguments.user;}
 }`

			pr := parser.ParseWithOptions(cfpath.ToURI(file), source, &parser.ParseOptions{ConstructorLookup: lookup})
			if pr.Funcs[0].Arguments[0].Type != "model.jobs.Job" {
				t.Fatalf("documented constructor contract lost: %+v", pr.Funcs[0])
			}

			primitive := parser.ParseWithOptions(cfpath.ToURI(file), `component {function init(string user) {}}`, &parser.ParseOptions{ConstructorLookup: lookup})
			if primitive.Funcs[0].Arguments[0].Component != "" {
				t.Fatal("primitive constructor contract overwritten")
			}

			private := parser.ParseWithOptions(cfpath.ToURI(file), `component {private function init(user) {}}`, &parser.ParseOptions{ConstructorLookup: lookup})
			if private.Funcs[0].Arguments[0].Component != "" {
				t.Fatal("private constructor inferred")
			}

			if r.ConstructorLookup(filepath.Join(dir, "Manual.cfc")) != nil {
				t.Fatal("unmanaged constructor inferred")
			}
		})
	}

	r, dir := writeDIApp(t, `component {}`, "")
	if r.ConstructorLookup(filepath.Join(dir, "model", "services", "Consumer.cfc")) != nil {
		t.Fatal("generic bean roots established DI1 construction")
	}
}

func TestDI1ConstructorRejectsCustomConstruction(t *testing.T) {
	r, dir := writeDIApp(t, `component {}`, `<cfscript>f=new framework.custom("/model");</cfscript>`)
	if err := os.WriteFile(filepath.Join(dir, "framework", "custom.cfc"), []byte(`component extends="framework.ioc" {function construct(path) {return new model.jobs.Job();}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if r.ConstructorLookup(filepath.Join(dir, "model", "services", "Consumer.cfc")) != nil {
		t.Fatal("custom construction established DI1 constructor identity")
	}
}

func TestDI1OverridesAlsoProtectSetterAndPropertyTypes(t *testing.T) {
	r, dir := writeDIApp(t, `component {}`, `<cfscript>f=new framework.ioc("/model");f.declareBean("consumer","model.services.Consumer",true,{reporter:"runtime string"});</cfscript>`)

	file := filepath.Join(dir, "model", "services", "Consumer.cfc")
	for _, fn := range r.EnsureIndexed(file) {
		if fn.Name == "setReporter" && fn.Arguments[0].Component != "" {
			t.Fatal("setter borrowed an overridden bean's type")
		}

		if fn.Name == "getReporter" && fn.ReturnComponent != "" {
			t.Fatal("property borrowed an overridden bean's type")
		}
	}
}

func TestDI1ConstructorDoesNotInferConstructionForConstants(t *testing.T) {
	r, dir := writeDIApp(t, `component extends="framework.one" {variables.framework={diConfig:{constants:{consumer:"runtime value"}}};}`, "")

	lookup := r.ConstructorLookup(filepath.Join(dir, "model", "services", "Consumer.cfc"))
	if lookup == nil || lookup("user") != "" {
		t.Fatal("constant registration inferred factory construction")
	}
}

// TestDI1BeanAddedAsANewInstanceIsThatComponent: Mura registers
// `serviceFactory.addBean( "fileWriter", new mura.fileWriter() )`, and every
// service taking a fileWriter constructor argument gets that instance. Only a
// variable or a dotted name was read as addBean's value, so the bean had no
// component and the argument stayed untyped. A call chained on the instance
// is not the instance.
func TestDI1BeanAddedAsANewInstanceIsThatComponent(t *testing.T) {
	r, dir := writeDIApp(t, `component {}`, `<cfscript>f=new framework.ioc("/model");
f.addBean("writer", new model.jobs.Job());
f.addBean("maker", createObject("component", "model.jobs.Job"));
f.addBean("chained", new model.jobs.Job().other());
</cfscript>`)

	lookup := r.ConstructorLookup(filepath.Join(dir, "model", "services", "Consumer.cfc"))
	if lookup == nil {
		t.Fatal("missing constructor policy")
	}

	job := filepath.Join(dir, "model", "jobs", "Job.cfc")

	for name, want := range map[string]string{"writer": job, "maker": job, "chained": ""} {
		if got := lookup(name); !cfpath.SamePath(got, want) && got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}
