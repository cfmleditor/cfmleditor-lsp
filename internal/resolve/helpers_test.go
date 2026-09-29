package resolve

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestFrameworkHelpersReachTheFilesTheyAreMixedInto: ColdBox mixes helper
// templates into handlers, views, layouts and interceptors — a module's
// (ModuleConfig.cfc's this.applicationHelper, relative to the module), the
// application's (config/ColdBox.cfc's applicationHelper, relative to the app)
// and a view's own <view>Helper.cfm and <folder>Helper.cfm. A bare call to one
// was "no qualifier, not in file": cbMessageBox() alone, 270 times in
// ContentBox's admin with its modules installed. A file the framework does not
// mix them into, a model, still does not see them.
func TestFrameworkHelpersReachTheFilesTheyAreMixedInto(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"modules/msg/ModuleConfig.cfc":     `component { this.applicationHelper = [ "helpers/Mixins.cfm" ]; function configure() {} }`,
		"modules/msg/helpers/Mixins.cfm":   `<cfscript>function cbMessageBox() { return 1; }</cfscript>`,
		"modules/admin/ModuleConfig.cfc":   `component { function onLoad() { wirebox.getInstance( "Renderer@coldbox" ).includeUDF( "#moduleMapping#/helpers/Mixins.cfm" ); } }`,
		"modules/admin/helpers/Mixins.cfm": `<cfscript>function cbAdminComponent() { return 1; }</cfscript>`,
		"config/ColdBox.cfc":               `component { function configure() { variables.coldbox = { applicationHelper : "includes/App.cfm" }; } }`,
		"includes/App.cfm":                 `<cfscript>function appHelp() { return 1; }</cfscript>`,
		"views/main/indexHelper.cfm":       `<cfscript>function viewHelp() { return 1; }</cfscript>`,
		"views/main/mainHelper.cfm":        `<cfscript>function folderHelp() { return 1; }</cfscript>`,
		"views/main/index.cfm":             `<cfoutput>#cbMessageBox()# #cbAdminComponent()# #appHelp()# #viewHelp()# #folderHelp()# #nope()#</cfoutput>`,
		"handlers/Main.cfc":                `component { function index() { cbMessageBox(); viewHelp(); } }`,
		"models/User.cfc":                  `component { function f() { cbMessageBox(); } }`,
	})

	scope := func(path string) bool {
		p := filepath.ToSlash(path)

		return strings.Contains(p, "/views/") || strings.Contains(p, "/handlers/")
	}

	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope}, dir, "views/main/index.cfm"), map[string]string{
		"cbMessageBox":     "",
		"cbAdminComponent": "",
		"appHelp":          "",
		"viewHelp":         "",
		"folderHelp":       "",
		"nope":             "no qualifier, not in file",
	})
	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope}, dir, "handlers/Main.cfc"), map[string]string{
		"cbMessageBox": "",
		"viewHelp":     "no qualifier, not in file", // a view's helper is the view's
	})
	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope}, dir, "models/User.cfc"), map[string]string{
		"cbMessageBox": "no qualifier, not in file",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "views/main/index.cfm"), map[string]string{
		"cbMessageBox": "no qualifier, not in file",
	})
}

// TestWheelsGlobalsReachEveryControllerModelAndView: Wheels includes the
// application's global/functions.cfm into every controller, model and view,
// and that file includes the rest (the starter app's auth.cfm, logging.cfm).
// Its hasPermission() was 17 findings in the starter app's views alone. A
// template it does not include, or an app with no global/functions.cfm, gives
// nothing.
func TestWheelsGlobalsReachEveryControllerModelAndView(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"app/global/functions.cfm":     `<cfscript> include "auth.cfm"; function util() {} </cfscript>`,
		"app/global/auth.cfm":          `<cfscript> function hasPermission( p ) { return true; } </cfscript>`,
		"app/global/unused.cfm":        `<cfscript> function notIncluded() {} </cfscript>`,
		"app/views/main/index.cfm":     `<cfoutput>#hasPermission( "x" )# #util()# #notIncluded()#</cfoutput>`,
		"app/controllers/Main.cfc":     `component { function index() { hasPermission( "x" ); } }`,
		"other/controllers/Orphan.cfc": `component { function index() { hasPermission( "x" ); } }`,
	})

	scope := func(path string) bool {
		p := filepath.ToSlash(path)

		return strings.Contains(p, "/views/") || strings.Contains(p, "/controllers/")
	}

	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope}, dir, "app/views/main/index.cfm"), map[string]string{
		"hasPermission": "",
		"util":          "",
		"notIncluded":   "no qualifier, not in file",
	})
	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope}, dir, "app/controllers/Main.cfc"), map[string]string{
		"hasPermission": "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{HelperScope: scope}, dir, "other/controllers/Orphan.cfc"), map[string]string{
		"hasPermission": "no qualifier, not in file",
	})
}
