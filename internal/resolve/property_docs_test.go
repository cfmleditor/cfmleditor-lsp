package resolve

import (
	"fmt"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
)

func TestPropertyDocGenericTypesReceiverAndGetter(t *testing.T) {
	for _, tag := range []bool{false, true} {
		t.Run(fmt.Sprintf("tag=%t", tag), func(t *testing.T) {
			dir := t.TempDir()

			store := `component {
property name="converter" doc_generic="models.Converter";
function convert() { variables.converter.serialize(); variables.converter.missing(); }
}`
			if tag {
				store = `<cfcomponent>
<cfproperty name="converter" doc_generic="models.Converter">
<cffunction name="convert"><cfset variables.converter.serialize()><cfset variables.converter.missing()></cffunction>
</cfcomponent>`
			}

			writeFiles(t, dir, map[string]string{
				"models/Converter.cfc": `component { function serialize() {} }`,
				"Store.cfc":            store,
				"Caller.cfc": `component {
function run() { var store = new Store(); store.getConverter().serialize(); store.getConverter().missing(); }
}`,
			})
			expectReasons(t, reasonsIn(t, dir, "Store.cfc"), map[string]string{
				"variables.converter.serialize": "",
				"variables.converter.missing":   "method 'missing' not found in models.Converter",
			})
			expectReasons(t, reasonsIn(t, dir, "Caller.cfc"), map[string]string{
				"store.getConverter.serialize": "",
				"store.getConverter.missing":   "method 'missing' not found in Converter",
			})
		})
	}
}

func TestBundledGettersKeepDependencyTypes(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Caller.cfc": `component {
function run() {
    var injector = new coldbox.system.ioc.Injector();
    injector.getScopeStorage().exists("key");
    injector.getScopeStorage().missing();
    injector.getParent().getInstance("name");
    injector.getParent().missing();
    var stats = new coldbox.system.cache.util.CacheStats();
    stats.getAssociatedCache().getOrSet("key");
    stats.getAssociatedCache().getName();
    var testbox = new testbox.system.TestBox();
    testbox.getUtility().slugify("name");
    testbox.getUtility().missing();
}
}`,
	})

	r := &Resolver{Stubs: frameworkapi.For([]string{"coldbox", "testbox"})}
	expectReasons(t, reasonsWith(t, r, dir, "Caller.cfc"), map[string]string{
		"injector.getScopeStorage.exists":   "",
		"injector.getScopeStorage.missing":  "method 'missing' not found in coldbox.system.core.collections.ScopeStorage",
		"injector.getParent.getInstance":    "",
		"injector.getParent.missing":        "method 'missing' not found in coldbox.system.ioc.Injector",
		"stats.getAssociatedCache.getOrSet": "",
		"stats.getAssociatedCache.getName":  "",
		"testbox.getUtility.slugify":        "",
		"testbox.getUtility.missing":        "method 'missing' not found in testbox.system.util.Util",
	})
}

func TestGeneratedGetterReadsConstructorField(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/Converter.cfc": `component { function serialize() {} }`,
		"models/Other.cfc":     `component { function other() {} }`,
		"Store.cfc": `component {
property name="converter";
property name="employee";
property name="localOnly";
property name="publicOnly";
property name="text" type="string";
property name="conflicted";
property name="explicit";
function init(required models.Converter employee) {
    variables.converter = new models.Converter();
    variables.employee = arguments.employee;
    var localOnly = new models.Converter();
    this.publicOnly = new models.Converter();
    variables.text = new models.Converter();
    variables.conflicted = new models.Converter();
    variables.conflicted = new models.Other();
    variables.explicit = new models.Converter();
}
models.Other function getExplicit() {}
}`,
		"Caller.cfc": `component {
function run() {
    var store = new Store();
    store.getConverter().serialize();
    store.getConverter().missing();
    store.getEmployee().serialize();
    store.getEmployee().missing();
    store.getLocalOnly().serialize();
    store.getPublicOnly().serialize();
    store.getText().serialize();
    store.getConflicted().serialize();
    store.getExplicit().other();
    store.getExplicit().serialize();
}
}`,
	})
	expectReasons(t, reasonsIn(t, dir, "Caller.cfc"), map[string]string{
		"store.getConverter.serialize":  "",
		"store.getConverter.missing":    "method 'missing' not found in Converter",
		"store.getEmployee.serialize":   "",
		"store.getEmployee.missing":     "method 'missing' not found in Converter",
		"store.getLocalOnly.serialize":  "method 'getLocalOnly' in Store has no component return type (chain to 'serialize')",
		"store.getPublicOnly.serialize": "method 'getPublicOnly' in Store has no component return type (chain to 'serialize')",
		"store.getText.serialize":       "method 'getText' in Store has no component return type (chain to 'serialize')",
		"store.getConflicted.serialize": "",
		"store.getExplicit.other":       "",
		"store.getExplicit.serialize":   "method 'serialize' not found in Other",
	})
}
