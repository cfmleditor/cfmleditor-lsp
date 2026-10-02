package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func TestProducerDefaultAndSuppliedObject(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`,
		"Other.cfc":   `component {function other(){}}`,
		"DAO.cfc": `<cfcomponent>
 <cffunction name="read"><cfargument name="id"><cfargument name="supplied" default="">
 <cfset var bean=arguments.supplied>
 <cfif not isObject(bean)><cfset bean=new Product()></cfif>
 <cfif flags><cfset bean.work()></cfif>
 <cfreturn bean>
 </cffunction>
 </cfcomponent>`,
		"Wrapper.cfc": `component {variables.dao=new DAO(); function read(id){return variables.dao.read(id);} }`,
		"TagPage.cfm": `<cfset dao=new DAO()><cfset item=dao.read('id')><cfset item.work()>`,
		"Page.cfc": `component {function run(){var dao=new DAO();
 var item=dao.read('id');
 item.work();
 dao.read('id').missing();
 }}`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{"item.work": "", "dao.read.missing": "method 'missing' not found in Product"})
	expectReasons(t, reasonsWith(t, r, dir, "TagPage.cfm"), map[string]string{"item.work": ""})

	lookup := r.FuncLookup(dir)
	for _, tc := range []struct{ expression, want string }{{"read('id')", "Product"}, {"read('id',new Other())", "Other"}, {"read('id',unknown)", ""}, {"read(id='id',supplied=new Product())", "Product"}, {"read(argumentCollection=unknown)", ""}, {"read(,)", ""}, {"read(argumentCollection={id='id'})", "Product"}, {"read('id','')", "Product"}} {
		got := lookup("DAO", parser.CallHop(tc.expression))
		if tc.want != "" {
			if r.ComponentPath(got, dir) != filepath.Join(dir, tc.want+".cfc") {
				t.Errorf("%s return %q", tc.expression, got)
			}
		} else if got != "" {
			t.Errorf("%s guessed %q", tc.expression, got)
		}
	}

	if got := lookup("DAO", "read"); got != "" {
		t.Fatalf("unconditional supplied-object return %q", got)
	}

	if got := lookup("Wrapper", "read"); r.ComponentPath(got, dir) != filepath.Join(dir, "Product.cfc") {
		t.Fatalf("wrapper return %q", got)
	}
}

func TestProducerArgumentBagAndBranchFlow(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`,
		"DAO.cfc":     `<cfcomponent><cffunction name="read"><cfargument name="value" default=""><cfset var bean=arguments.value><cfif not isObject(bean)><cfset bean=new Product()></cfif><cfreturn bean></cffunction></cfcomponent>`,
		"Wrapper.cfc": `component {variables.dao=new DAO();function load(){arguments.value=this;return variables.dao.read(argumentCollection=arguments);}function work(){}}`,
		"Page.cfc":    `component {function run(){var wrapper=new Wrapper();var result=wrapper.load();result.work();}}`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{"result.work": ""})

	if got := r.FuncLookup(dir)("Wrapper", parser.CallHop("load()")); r.ComponentPath(got, dir) != filepath.Join(dir, "Wrapper.cfc") {
		t.Fatalf("argument bag lost %q", got)
	}
}

func TestProducerInferenceBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, body, call string }{
		{"supplied unknown", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}return bean;`, `read(unknown)`},
		{"primitive branch", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}if(flag){return false;}return bean;`, `read()`},
		{"component conflict", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}if(flag){bean=new Other();}return bean;`, `read()`},
		{"missing return", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}if(flag){return bean;}`, `read()`},
		{"condition escape", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}if(mutate(local)){flag=true;}return bean;`, `read()`},
		{"condition mutation", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}if((bean=unknown)){flag=true;}return bean;`, `read()`},
		{"increment", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}bean++;return bean;`, `read()`},
		{"later scope escape", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}mutate(1,local);return bean;`, `read()`},
		{"named scope escape", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}mutate(scope=local);return bean;`, `read()`},
		{"discarded result scope escape", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}var unused=mutate(local);return bean;`, `read()`},
		{"parent replacement", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}arguments.record.bean=bean;arguments.record=unknown;return arguments.record.bean;`, `read()`},
		{"scope escape", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}mutate(local);return bean;`, `read()`},
		{"container alias", `var args=arguments;mutate(args);var bean=arguments.value;if(!isObject(bean)){bean=new Product();}return bean;`, `read()`},
		{"dynamic key", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}local[key]=unknown;return bean;`, `read()`},
		{"unsupported switch", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}switch(flag){case 1:bean=new Other();break;}return bean;`, `read()`},
		{"while condition assignment", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}while((bean=unknown)){}return bean;`, `read()`},
		{"for header mutation", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}for(var i=0;i<10;bean++){}return bean;`, `read()`},
		{"loop convergence", `var bean=arguments.value;if(!isObject(bean)){bean=new Product();}var next=bean;while(flag){bean=next;next=new Other();}return bean;`, `read()`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
				"DAO.cfc": `component {function read(value=""){` + tc.body + `}}`, "Page.cfc": `component {function run(){var dao=new DAO();dao.read().work();}}`,
			})

			r := &Resolver{}
			reasonsWith(t, r, dir, "Page.cfc")

			if got := r.FuncLookup(dir)("DAO", parser.CallHop(tc.call)); got != "" && got != "$any" {
				t.Fatalf("unsafe inference %q", got)
			}
		})
	}
}

func TestProducerOptionalPresence(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc":  `component {function read(value){if(structKeyExists(arguments,"value")){return new Other();}else{return new Product();}}}`,
		"Page.cfc": `component {function run(){var dao=new DAO();dao.read().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	lookup := r.FuncLookup(dir)
	if got := lookup("DAO", "read"); got != "" {
		t.Fatalf("generic optional presence guessed %q", got)
	}

	for _, tc := range []struct{ call, want string }{{"read()", "Product"}, {"read(unknown)", "Other"}} {
		if got := lookup("DAO", parser.CallHop(tc.call)); r.ComponentPath(got, dir) != filepath.Join(dir, tc.want+".cfc") {
			t.Fatalf("%s return %q", tc.call, got)
		}
	}
}

func TestProducerQueryOutputContracts(t *testing.T) {
	for _, tc := range []struct{ name, attrs, want string }{
		{"finite attributes", `config.attrs(name='rows')`, "Product"},
		{"replaced component", `config.attrs(name='bean')`, ""},
		{"unknown output", `config.attrs(name=unknown)`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Product.cfc": `component {function work(){}}`,
				"Config.cfc":  `<cfcomponent><cffunction name="attrs"><cfset structAppend(arguments,{datasource="db"},false)><cfset structDelete(arguments,"password")><cfreturn arguments></cffunction></cfcomponent>`,
				"DAO.cfc":     `<cfcomponent><cfset variables.config=new Config()><cffunction name="read"><cfargument name="value" default=""><cfset var bean=arguments.value><cfif not isObject(bean)><cfset bean=new Product()></cfif><cfquery attributeCollection="#variables.` + tc.attrs + `#">select 1</cfquery><cfreturn bean></cffunction></cfcomponent>`,
				"Page.cfc":    `component {function run(){var dao=new DAO();dao.read().work();}}`,
			})

			r := &Resolver{}
			reasonsWith(t, r, dir, "Page.cfc")

			got := r.FuncLookup(dir)("DAO", parser.CallHop("read()"))
			if tc.want != "" {
				if r.ComponentPath(got, dir) != filepath.Join(dir, tc.want+".cfc") {
					t.Fatalf("return %q", got)
				}
			} else if got != "" {
				t.Fatalf("query replaced component but return %q", got)
			}
		})
	}
}

func TestProducerChainsRecordsAndSourceRefresh(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`,
		"Other.cfc":   `component {function other(){}}`,
		"DAO.cfc":     `component {function read(value=""){var bean=value;if(!isObject(bean)){bean=new Product();}return bean;}}`,
		"Wrapper.cfc": `component {variables.dao=new DAO();function read(){return variables.dao.read();}}`,
		"Page.cfc":    `component {function run(){var rc={};rc.dao=new DAO();rc.product=rc.dao.read();rc.product.work();var result=new Wrapper().read();result.work();}}`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{"rc.product.work": "", "result.work": ""})

	lookup := r.FuncLookup(dir)
	if got := lookup("DAO", parser.CallHop("read()")); r.ComponentPath(got, dir) != filepath.Join(dir, "Product.cfc") {
		t.Fatalf("initial result %q", got)
	}

	writeFiles(t, dir, map[string]string{"DAO.cfc": `component {function read(value=""){var bean=value;if(!isObject(bean)){bean=new Other();}return bean;}}`})

	if got := lookup("DAO", parser.CallHop("read()")); r.ComponentPath(got, dir) != filepath.Join(dir, "Other.cfc") {
		t.Fatalf("stale result %q", got)
	}
}

func TestProducerCycleAndGuardOverride(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`,
		"DAO.cfc":     `component {function read(value=""){var bean=value;if(!isObject(bean)){bean=new Product();}return bean;}function cycle(){return cycle();}}`,
		"Custom.cfc":  `component extends="DAO" {function isObject(value){return true;}}`,
		"Page.cfc":    `component {function run(){var dao=new Custom();dao.read().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	for _, tc := range []struct{ component, call string }{{"Custom", "read()"}, {"DAO", "cycle()"}} {
		if got := r.FuncLookup(dir)(tc.component, parser.CallHop(tc.call)); got != "" {
			t.Fatalf("%s %s inferred %q", tc.component, tc.call, got)
		}
	}
}

func TestProducerUncertainArgumentBagPresence(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc":     `component {function read(value){if(structKeyExists(arguments,"value")){return new Other();}else{return new Product();}}}`,
		"Wrapper.cfc": `component {variables.dao=new DAO();function read(){if(flag){arguments.value="";}return variables.dao.read(argumentCollection=arguments);}}`,
		"Page.cfc":    `component {function run(){var wrapper=new Wrapper();wrapper.read().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	if got := r.FuncLookup(dir)("Wrapper", parser.CallHop("read()")); got != "" {
		t.Fatalf("uncertain bag guessed %q", got)
	}
}

func TestProducerLiteralDispatch(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc":  `component {function read(id="",other="",supplied=""){var bean=supplied;if(!isObject(bean)){bean=new Product();}if(len(other)){return new Other();}else{return bean;}}}`,
		"Page.cfc": `component {function run(){var dao=new DAO();dao.read().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	for _, tc := range []struct{ call, want string }{{"read(id='id')", "Product"}, {"read(other='x')", "Other"}, {"read(other=unknown)", ""}} {
		got := r.FuncLookup(dir)("DAO", parser.CallHop(tc.call))
		if tc.want == "" {
			if got != "" {
				t.Fatalf("unknown dispatch %q", got)
			}
		} else if r.ComponentPath(got, dir) != filepath.Join(dir, tc.want+".cfc") {
			t.Fatalf("%s return %q", tc.call, got)
		}
	}
}

func TestProducerFactoryCallerApplications(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"DAO.cfc":                `component {function read(value=""){var bean=value;if(!isObject(bean)){bean=getBean('product');}return bean;}}`,
		"one/Application.cfc":    `component {this.mappings['/models']=expandPath('./models');}`,
		"one/models/Product.cfc": `component {function first(){}}`,
		"two/Application.cfc":    `component {this.mappings['/models']=expandPath('./models');}`,
		"two/models/Product.cfc": `component {function second(){}}`,
		"Page.cfc":               `component {function run(){var dao=new DAO();dao.read();}}`,
	})

	r := &Resolver{Resolvers: []parser.Resolver{{Match: `(?i)getBean\(\s*['"]product['"]\s*\)`, Resolve: "models.Product", Prefix: "getBean"}}}
	reasonsWith(t, r, dir, "Page.cfc")

	for _, name := range []string{"one", "two", "one"} {
		caller := filepath.Join(dir, name)

		got := r.FuncLookup(caller)(filepath.Join(dir, "DAO.cfc"), parser.CallHop("read()"))
		if r.ComponentPath(got, caller) != filepath.Join(caller, "models", "Product.cfc") {
			t.Fatalf("%s factory return %q", name, got)
		}
	}
}

func TestProducerSuppliedExpressionScope(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lib/DAO.cfc":   `component {variables.supplied=new Other();function helper(){return new Other();}function read(value=""){var bean=value;if(!isObject(bean)){bean=new Other();}return bean;}}`,
		"lib/Other.cfc": `component {function library(){}}`,
		"Other.cfc":     `component {function caller(){}}`,
		"Page.cfc":      `component {function run(){var dao=new lib.DAO();dao.read().library();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	lookup := r.FuncLookup(dir)
	if got := lookup("lib.DAO", parser.CallHop("read(new Other())")); r.ComponentPath(got, dir) != filepath.Join(dir, "Other.cfc") {
		t.Fatalf("argument resolved beside callee: %q", got)
	}

	for _, call := range []string{"read(this)", "read(variables.supplied)", "read(helper())"} {
		if got := lookup("lib.DAO", parser.CallHop(call)); got != "" {
			t.Fatalf("borrowed callee binding: %s %q", call, got)
		}
	}
}

func TestProducerGenericArgumentBag(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc":     `component {function read(supplied=""){var bean=supplied;if(!isObject(bean)){bean=new Product();}return bean;}}`,
		"Wrapper.cfc": `component {variables.dao=new DAO();function read(){return variables.dao.read(argumentCollection=arguments);}}`,
		"Page.cfc":    `component {function run(){var wrapper=new Wrapper();wrapper.read().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	lookup := r.FuncLookup(dir)
	if got := lookup("Wrapper", "read"); got != "" {
		t.Fatalf("generic bag treated as empty: %q", got)
	}

	for _, tc := range []struct{ call, want string }{{"read()", "Product"}, {"read(supplied=new Other())", "Other"}, {"read(supplied=unknown)", ""}} {
		got := lookup("Wrapper", parser.CallHop(tc.call))
		if tc.want == "" {
			if got != "" {
				t.Fatalf("unknown supplied object guessed %q", got)
			}
		} else if r.ComponentPath(got, dir) != filepath.Join(dir, tc.want+".cfc") {
			t.Fatalf("%s return %q", tc.call, got)
		}
	}
}

func TestProducerComputedTagDefaultPresence(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc":  `<cfcomponent><cffunction name="read"><cfargument name="value" default="#runtimeValue()#"><cfif structKeyExists(arguments,'value')><cfreturn new Other()><cfelse><cfreturn new Product()></cfif></cffunction><cffunction name="object"><cfargument name="value" default="#runtimeValue()#"><cfset var bean=arguments.value><cfif not isObject(bean)><cfset bean=new Product()></cfif><cfreturn bean></cffunction></cfcomponent>`,
		"Page.cfc": `component {function run(){var dao=new DAO();dao.read().other();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	lookup := r.FuncLookup(dir)
	if got := lookup("DAO", parser.CallHop("read()")); r.ComponentPath(got, dir) != filepath.Join(dir, "Other.cfc") {
		t.Fatalf("computed default was treated as absent: %q", got)
	}

	if got := lookup("DAO", parser.CallHop("object()")); got != "" {
		t.Fatalf("computed object guessed %q", got)
	}
}

func TestProducerTagReturnArguments(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc": `component {function read(id,supplied=""){var bean=supplied;if(!isObject(bean)){bean=new Product();}return bean;}}`,
		"Wrapper.cfc": `<cfcomponent><cfset variables.dao=new DAO()>
<cffunction name="scalar">
<cfset var bean=variables.dao.read('id',new Other())>
<cfsetting requesttimeout="10"><cfreturn bean>
</cffunction>
<cffunction name="produce">
<cfset var items=[]><cfset items[1]=variables.dao.read('id',new Other())>
<cfsetting requesttimeout="10"><cfreturn items[1]>
</cffunction></cfcomponent>`,
		"Page.cfc": `component {function run(){var wrapper=new Wrapper();wrapper.produce().missing();}}`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{"wrapper.produce.missing": "method 'missing' not found in Other"})

	for _, method := range []string{"scalar", "produce"} {
		want := "Other"
		if got := r.FuncLookup(dir)("Wrapper", method); r.ComponentPath(got, dir) != filepath.Join(dir, want+".cfc") {
			t.Fatalf("%s returned-call contract lost: %q", method, got)
		}
	}
}

func TestProducerOwnedArrayBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, params, body string }{
		{"mixed", "", `var items=[];items[1]=new Product();items[2]=new Other();return items[1];`},
		{"unknown mutation", "", `var items=[];items[1]=new Product();arrayAppend(items,unknown);return items[1];`},
		{"supplied array", "items=[]", `items[1]=new Product();return items[1];`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{
				"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
				"DAO.cfc": `component {function read(` + tc.params + `){` + tc.body + `}}`, "Page.cfc": `component {function run(){var dao=new DAO();dao.read().work();}}`,
			})

			r := &Resolver{}
			reasonsWith(t, r, dir, "Page.cfc")

			if got := r.FuncLookup(dir)("DAO", parser.CallHop("read()")); got != "" {
				t.Fatalf("unsafe array contract %q", got)
			}
		})
	}
}

func TestProducerFiniteRecordEscapes(t *testing.T) {
	for _, body := range []string{
		`<cfset var attrs={name='rows'}><cfset var copy=attrs><cfset mutate(attrs)>`,
		`<cfset var attrs={name='rows'}><cfset var unused=mutate(attrs)><cfset var copy=attrs>`,
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"Product.cfc": `component {function work(){}}`,
			"DAO.cfc":     `<cfcomponent><cffunction name="read"><cfargument name="value" default=""><cfset var bean=arguments.value><cfif not isObject(bean)><cfset bean=new Product()></cfif>` + body + `<cfquery attributeCollection="#copy#">select 1</cfquery><cfreturn bean></cffunction></cfcomponent>`,
			"Page.cfc":    `component {function run(){var dao=new DAO();dao.read().work();}}`,
		})

		r := &Resolver{}
		reasonsWith(t, r, dir, "Page.cfc")

		if got := r.FuncLookup(dir)("DAO", parser.CallHop("read()")); got != "" {
			t.Fatalf("escaped record inferred %q", got)
		}
	}
}

func TestProducerMixedTagScriptArguments(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`, "Other.cfc": `component {function other(){}}`,
		"DAO.cfc": `<cfcomponent>
 <cffunction name="read"><cfargument name="value" default="">
 <cfscript>var bean=arguments.value; if(!isObject(bean)){bean=new Product();}</cfscript>
 <cfreturn bean></cffunction>
 <cffunction name="unsupported"><cfargument name="value" default="">
 <cfscript>var bean=arguments.value;if(!isObject(bean)){bean=new Product();}switch(runtime){case 1:bean=unknown;}</cfscript>
 <cfreturn bean></cffunction></cfcomponent>`,
		"Wrapper.cfc": `<cfcomponent><cfset variables.dao=new DAO()>
 <cffunction name="read"><cfscript>var bean=variables.dao.read(new Other());</cfscript><cfreturn bean></cffunction></cfcomponent>`,
		"Page.cfc": `component {function run(){var dao=new DAO();dao.read().work();}}`,
	})

	r := &Resolver{}
	reasonsWith(t, r, dir, "Page.cfc")

	for _, tc := range []struct{ component, call, want string }{{"DAO", "read()", "Product"}, {"DAO", "read(new Other())", "Other"}, {"Wrapper", "read()", "Other"}, {"DAO", "read(unknown)", ""}, {"DAO", "unsupported()", ""}} {
		got := r.FuncLookup(dir)(tc.component, parser.CallHop(tc.call))
		if tc.want == "" {
			if got != "" {
				t.Fatalf("%s guessed %q", tc.call, got)
			}
		} else if r.ComponentPath(got, dir) != filepath.Join(dir, tc.want+".cfc") {
			t.Fatalf("%s returned %q", tc.call, got)
		}
	}
}

func TestProducerCreateObjectReturn(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`,
		"DAO.cfc": `<cfcomponent>
 <cffunction name="read"><cfset var bean=createObject("component","Product").init(this)><cfreturn bean></cffunction>
 <cffunction name="unknown"><cfset var bean=createObject("component",runtimePath)><cfreturn bean></cffunction>
 <cffunction name="java"><cfset var bean=createObject("java","Product")><cfreturn bean></cffunction></cfcomponent>`,
		"Page.cfc": `component {function run(){var dao=new DAO();dao.read().missing();}}`,
	})

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{"dao.read.missing": "method 'missing' not found in Product"})

	for _, method := range []string{"unknown", "java"} {
		if got := r.FuncLookup(dir)("DAO", parser.CallHop(method+"()")); got != "" && got != "$any" {
			t.Fatalf("%s guessed %q", method, got)
		}
	}
}

func TestProducerPreservesNonComponentContracts(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"DAO.cfc": `component {function read(){var q=queryNew('id');return q;}function dynamic(){var value=createObject('java','java.lang.System');return value;}}`, "Page.cfc": `component {function run(){var dao=new DAO();dao.read().missing();dao.dynamic().getProperty('test');}}`})

	r := &Resolver{}

	got := reasonsWith(t, r, dir, "Page.cfc")
	file := filepath.Join(dir, "DAO.cfc")

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	r.Index.IndexFileWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{BuiltinReturnLookup: docs.LookupBuiltinReturnComponent})

	if ret := r.FuncLookup(dir)("DAO", "read"); ret != "$builtin.querynew" {
		t.Fatalf("builtin contract lost: %q", ret)
	}

	if got["dao.dynamic.getProperty"] != "" {
		t.Fatalf("dynamic contract lost: %v", got)
	}
}

func TestProducerHiddenWritesWithholdTheReturn(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"A.cfc": `component {function a(){}}`,
		"B.cfc": `component {function b(){}}`,
		"F.cfc": `component {
function makeA(){ return new A(); }
function closure(xs){ var r = makeA(); arrayEach(xs, function(x){ r = new B(); }); return r; }
function arrow(xs){ var r = makeA(); xs.each((x) => { r = new B(); }); return r; }
function included(){ var r = makeA(); include "x.cfm"; return r; }
function plain(){ var r = makeA(); return r; }
}`,
	})

	r := &Resolver{}
	_ = reasonsWith(t, r, dir, "A.cfc")
	lookup := r.FuncLookup(dir)

	for _, m := range []string{"closure([])", "arrow([])", "included()"} {
		if got := lookup("F", parser.CallHop(m)); got != "" {
			t.Errorf("%s ignored a hidden write: %q", m, got)
		}
	}

	if got := lookup("F", parser.CallHop("plain()")); r.ComponentPath(got, dir) != filepath.Join(dir, "A.cfc") {
		t.Fatalf("plain return %q", got)
	}
}

func TestProducerPlanMustMatchTheIndexedSignature(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"DAO.cfc": `component {function read(value=""){var bean=value;if(!isObject(bean)){bean=new DAO();}return bean;}}`,
	})

	r := &Resolver{}
	_ = reasonsWith(t, r, dir, "DAO.cfc")
	fd := &parser.FunctionDef{Name: "read", URI: cfpath.ToURI(filepath.Join(dir, "DAO.cfc")), Arguments: []parser.Argument{{Name: "value"}}}

	if r.producerFor(fd) == nil {
		t.Fatal("matching signature has no plan")
	}

	// The buffer renamed the parameter; the plan on disk is another version.
	fd.Arguments = []parser.Argument{{Name: "id"}, {Name: "value"}}
	if r.producerFor(fd) != nil {
		t.Fatal("plan used against a different signature")
	}
}

// hopNames is a CallSite.Chain as the methods it calls.
func hopNames(chain []string) []string {
	names := make([]string, len(chain))
	for i, hop := range chain {
		names[i] = parser.CallHopName(hop)
	}

	return names
}

// TestAProducerHopReadsItsOwnArguments: the second chain on the line is
// typed by its own call's arguments. The hop's arguments used to be found by
// searching the line for the hop's name, which answers nothing when the name
// is called twice there — two chains, or a bare read( id ) beside one — and
// the hop fell back to read's unspecialised return, which is none.
func TestAProducerHopReadsItsOwnArguments(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Product.cfc": `component {function work(){}}`,
		"Other.cfc":   `component {function other(){}}`,
		"DAO.cfc":     `component {function read(id, supplied=""){var bean=arguments.supplied;if(!isObject(bean)){bean=new Product();}return bean;}}`,
		"Page.cfc": `component {function read(id){} function run(){var dao=new DAO();
 dao.read(1).work(); dao.read(1, new Other()).other();
 read(1); dao.read(2, new Other()).nope();
 }}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"dao.read.work":  "",
		"dao.read.other": "",
		"dao.read.nope":  "method 'nope' not found in Other",
	})
}

// TestCallTextNamesHopsWithoutTheirArguments: the explain report heads a call
// with its hops' names, as it did before a hop carried its arguments.
func TestCallTextNamesHopsWithoutTheirArguments(t *testing.T) {
	call := &parser.CallSite{FuncName: "work", Variable: "dao", Chain: []string{parser.CallHop("read( 1, x.y() )"), "make"}}
	if got := CallText(call); got != "dao.read().make().work" {
		t.Fatalf("CallText = %q", got)
	}
}
