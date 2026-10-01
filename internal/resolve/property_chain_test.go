package resolve

import "testing"

func TestConstructorPropertyChainsUsePublicFieldType(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Service.cfc": `component {function work() {}}`,
		"Builder.cfc": `component {function init() {this.service = new Service(); variables.secret = new Service(); return this;} function asService(){return new Service();}}`,
		"Dynamic.cfc": `component {function init() {this.service = createObject("component", "pkg.#kind#"); return this;}}`,
		"Changed.cfc": `component extends="Builder" {function init(){this.service = 1; return this;}}`,
		"Mixed.cfc": `component {function init(){
 this.service = new Service();
 this.service = incoming;
 return this;}}`,
		"Page.cfc": `component {function run(){
 var a = new Builder().init().service; a.work(); a.missing();
 var b = createObject("component","Builder").init().service; b.work(); b.missing();
 new Builder().init().service.work(); new Builder().init().service.missing();
 var c = new Dynamic().init().service; c.whatever();
 var d = new Changed().init().service; d.whatever();
 var e = new Mixed().init().service; e.whatever();
 var secret = new Builder().init().secret; secret.whatever();
 }}`,
		"TagPage.cfc": `<cfcomponent><cffunction name="run"><cfset var a=createObject("component","Builder").init().service><cfset a.work()><cfset a.missing()></cffunction></cfcomponent>`,
	})

	for _, file := range []string{"Page.cfc", "TagPage.cfc"} {
		got := reasonsIn(t, dir, file)
		expectReasons(t, got, map[string]string{"a.work": "", "a.missing": "method 'missing' not found in Service"})

		if file == "Page.cfc" {
			expectReasons(t, got, map[string]string{"b.work": "", "b.missing": "method 'missing' not found in Service", "c.whatever": "", "d.whatever": "", "e.whatever": "", "secret.whatever": "", "init.$property:service.work": "", "init.$property:service.missing": "method 'missing' not found in Service"})
		}
	}
}

func TestPublicPropertyReplacementDoesNotBorrowBase(t *testing.T) {
	for _, write := range []string{`this["service"]=incoming;`, `this[key]=incoming;`, `this["#key#"]=incoming;`} {
		t.Run(write, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, map[string]string{"Service.cfc": `component {function work(){}}`, "Base.cfc": `component {function init(){this.service=new Service();return this;}}`, "Child.cfc": `component extends="Base" {function change(){` + write + `}}`, "Page.cfc": `component {function run(){var a=new Child().init().service;a.unknown();}}`})
			expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{"a.unknown": ""})
		})
	}
}
