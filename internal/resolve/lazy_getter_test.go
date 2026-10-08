package resolve

import "testing"

// TestALazyGetterReturnsWhatItLoads: Mura's configBean starts its class
// extension manager as "" and loads it on the first call to
// getClassExtensionManager(), which guards with `not isObject()` and returns
// the field. The placeholder and the component gave the field no single type,
// and 64 admin calls on what the getter returns were untyped. The guard means
// the getter never returns the placeholder; a write of anything else than a
// component created in place, or a getter that does more than guard and
// return, gives nothing.
func TestALazyGetterReturnsWhatItLoads(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"extend/Manager.cfc": `component { function init( config ){ return this; } function getSubTypes(){} }`,
		"Config.cfc": `<cfcomponent>
<cfset variables.instance.extensionManager = "" />
<cfset variables.instance.other = "" />
<cfset variables.instance.mixed = "" />
<cffunction name="load"><cfset variables.instance.extensionManager = createObject("component","extend.Manager").init(this) /></cffunction>
<cffunction name="loadOther"><cfset variables.instance.other = new extend.Manager() /></cffunction>
<cffunction name="loadMixed"><cfset variables.instance.mixed = new extend.Manager() /><cfset variables.instance.mixed = "x" /></cffunction>
<cffunction name="getManager">
	<cfif not isObject(variables.instance.extensionManager)>
		<cfset load()/>
	</cfif>
	<cfreturn variables.instance.extensionManager />
</cffunction>
<cffunction name="getMixed">
	<cfif not isObject(variables.instance.mixed)><cfset loadMixed()/></cfif>
	<cfreturn variables.instance.mixed />
</cffunction>
<cffunction name="getEarly">
	<cfif not isObject(variables.instance.other)><cfreturn "none"></cfif>
	<cfreturn variables.instance.other />
</cffunction>
</cfcomponent>`,
		"Script.cfc": `component {
	variables.cache = "";
	variables.loose = "";
	function build(){ variables.cache = new extend.Manager(); }
	function getCache(){
		if ( !isObject( variables.cache ) ) { build(); }
		return variables.cache;
	}
	function getLoose(){
		if ( !isObject( variables.loose ) ) { variables.loose = makeOne(); }
		return variables.loose;
	}
}`,
		"Application.cfc": `component { function onApplicationStart(){ include "startup.cfm"; } }`,
		"startup.cfm": `<cfset application.config = new Config()>
<cfset application.mgr = application.config.getManager()>
<cfset application.mixed = application.config.getMixed()>
<cfset application.early = application.config.getEarly()>
<cfset application.loose = new Script().getLoose()>`,
		"Page.cfc": `component {
	function f(){
		application.mgr.getSubTypes();
		application.mgr.nope();
		application.mixed.getSubTypes();
		application.early.getSubTypes();
		var cache = new Script().getCache();
		cache.getSubTypes();
		application.loose.getSubTypes();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"application.mgr.getSubTypes":   "",
		"application.mgr.nope":          "method 'nope' not found in Manager",
		"application.mixed.getSubTypes": "variable 'application.mixed' has no component ref",
		"application.early.getSubTypes": "variable 'application.early' has no component ref",
		"cache.getSubTypes":             "",
		"application.loose.getSubTypes": "variable 'application.loose' has no component ref",
	})
}

// TestAStartupCreationIsWhatItsChainedCallsReturn: a startup template's
// `application.configBean = new mura.configBean().set( props )` is a
// configBean because set() returns this; `new Script().getLoose()` is what
// getLoose() returns, not a Script. The creation was matched as a prefix, so
// whatever was chained on it was ignored and the head's type given.
func TestAStartupCreationIsWhatItsChainedCallsReturn(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"model/Cfg.cfc":   `component { function init(){ return this; } function set( p ){ return this; } function make(){ return 1; } function own(){} }`,
		"Application.cfc": `component { function onApplicationStart(){ include "startup.cfm"; } }`,
		"startup.cfm": `<cfscript>
application.cfg = new model.Cfg().set( 1 );
application.inited = createObject( "component", "model.Cfg" ).init();
application.made = new model.Cfg().make();
</cfscript>`,
		"Page.cfc": `component {
	function f(){ application.cfg.own(); application.inited.own(); application.made.own(); }
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{
		"application.cfg.own":    "",
		"application.inited.own": "",
		"application.made.own":   "variable 'application.made' has no component ref",
	})
}
