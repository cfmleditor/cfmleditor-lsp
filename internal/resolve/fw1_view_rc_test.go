package resolve

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestAnFW1ViewsRcIsWhatItsControllerAssigns: Mura's admin view
// views/carch/edit.cfm reads rc.contentBean, which controllers/carch.cfc's
// edit( rc ) assigns, and its partials read it through the view including
// them. An action that assigns nothing defers to before(); actions that
// disagree, or a view no action renders, give nothing; and outside a
// framework whose views have a base the convention is not applied.
func TestAnFW1ViewsRcIsWhatItsControllerAssigns(t *testing.T) {
	files := map[string]string{
		"framework/one.cfc": `component { function view(){} }`,
		"model/Content.cfc": `component { function getTitle(){} }`,
		"model/Site.cfc":    `component { function getName(){} }`,
		"controllers/carch.cfc": `component {
	function before( rc ){ arguments.rc.siteBean = new model.Site(); }
	function edit( rc ){
		arguments.rc.contentBean = new model.Content();
	}
	function list( rc ){
		rc.contentBean = new model.Site();
	}
	function other( rc ){
		variables.fw.setView("carch.list");
		rc.contentBean = new model.Content();
	}
}`,
		"views/carch/edit.cfm":       `<cfoutput>#rc.contentBean.getTitle()# #rc.siteBean.getName()# #rc.contentBean.nope()#</cfoutput><cfinclude template="form/panel.cfm">`,
		"views/carch/form/panel.cfm": `<cfoutput>#rc.contentBean.getTitle()#</cfoutput>`,
		"views/carch/list.cfm":       `<cfoutput>#rc.contentBean.getTitle()#</cfoutput>`,
		"views/carch/orphan.cfm":     `<cfoutput>#rc.contentBean.getTitle()#</cfoutput>`,
	}

	dir := t.TempDir()
	writeFiles(t, dir, files)

	views := func(path string) string {
		if strings.Contains(filepath.ToSlash(path), "/views/") {
			return "framework.one"
		}

		return ""
	}

	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: views}, dir, "views/carch/edit.cfm"), map[string]string{
		"rc.contentBean.getTitle": "",
		"rc.siteBean.getName":     "",
		"rc.contentBean.nope":     "method 'nope' not found in Content",
	})
	expectReasons(t, templateReasonsWith(t, &Resolver{ImplicitExtends: views}, dir, "views/carch/form/panel.cfm"), map[string]string{
		"rc.contentBean.getTitle": "",
	})

	for _, page := range []string{"views/carch/list.cfm", "views/carch/orphan.cfm"} {
		if got := reasonsWith(t, &Resolver{ImplicitExtends: views}, dir, page)["rc.contentBean.getTitle"]; got == "" {
			t.Errorf("%s: resolved with no single answer", page)
		}
	}

	if got := reasonsWith(t, &Resolver{}, dir, "views/carch/edit.cfm")["rc.contentBean.getTitle"]; got == "" {
		t.Errorf("edit.cfm: resolved without a framework giving views a base")
	}
}

// TestAnRcMemberAssignedFromAnInheritedServiceIsTyped: Masa's csettings
// controller fills `arguments.rc.siteBean = variables.settingsManager.read(
// arguments.rc.siteid )`, where settingsManager is injected through a setter
// on the base controller. The parse cannot type that, and the lookup that
// types `x = svc.read()` at the line took only a bare name and prc.x, so the
// controller's rc.siteBean, and every call on it in the view, was untyped.
func TestAnRcMemberAssignedFromAnInheritedServiceIsTyped(t *testing.T) {
	files := map[string]string{
		"framework/one.cfc": `component { function view(){} }`,
		"model/Site.cfc":    `component { function getThemes(){} }`,
		"model/Manager.cfc": `component { model.Site function read( id ){} }`,
		"controllers/controller.cfc": `component {
	function setSettingsManager( settingsManager ){ variables.settingsManager = arguments.settingsManager; }
	function init(){ variables.settingsManager = new model.Manager(); }
}`,
		"controllers/csettings.cfc": `component extends="controller" {
	function editSite( rc ){
		arguments.rc.siteBean = variables.settingsManager.read( arguments.rc.siteid );
		arguments.rc.siteBean.getThemes();
	}
	function tagged( rc ){ }
}`,
		"controllers/ctag.cfc": `<cfcomponent extends="controller">
<cffunction name="show"><cfargument name="rc">
	<cfset rc.siteBean = variables.settingsManager.read(rc.siteid)>
	<cfset rc.siteBean.getThemes()>
	<cfset rc.siteBean = rc.siteBean.missingSave()>
	<cfset rc.siteBean.getThemes()>
</cffunction>
</cfcomponent>`,
		"views/csettings/editsite.cfm": `<cfoutput>#rc.siteBean.getThemes()# #rc.siteBean.nope()#</cfoutput>`,
	}

	dir := t.TempDir()
	writeFiles(t, dir, files)

	views := func(path string) string {
		if strings.Contains(filepath.ToSlash(path), "/views/") {
			return "framework.one"
		}

		return ""
	}

	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: views}, dir, "controllers/csettings.cfc"), map[string]string{
		"arguments.rc.siteBean.getThemes": "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{ImplicitExtends: views}, dir, "views/csettings/editsite.cfm"), map[string]string{
		"rc.siteBean.getThemes": "",
		"rc.siteBean.nope":      "method 'nope' not found in Site",
	})
	// A member assigned from a call on itself is not typed by that call.
	got := reasonsWith(t, &Resolver{ImplicitExtends: views}, dir, "controllers/ctag.cfc")
	if got["rc.siteBean.missingSave"] != "method 'missingSave' not found in model.Site" {
		t.Errorf("rc.siteBean.missingSave: %q", got["rc.siteBean.missingSave"])
	}
}
