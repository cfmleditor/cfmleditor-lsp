package resolve

import (
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/parser"
)

func wheelsControllerFiles() map[string]string {
	return map[string]string{
		"wheels/Global.cfc": `component { include "global/objects.cfm"; }`,
		"wheels/global/objects.cfm": `<cfscript>
 public any function controller(required string name, struct params = {}) {
	local.rv = $cachedControllerLookup(name = arguments.name);
	if (IsBoolean(local.rv) && !local.rv) {
		local.args = {};
		local.args.name = arguments.name;
		local.rv = $doubleCheckedLock(condition = "$cachedControllerClassExists", conditionArgs = local.args, execute = "$createControllerClass", executeArgs = local.args, name = "controllerLock");
	}
	if (!StructIsEmpty(arguments.params)) {
		local.rv = local.rv.$createControllerObject(arguments.params);
	}
	return local.rv;
 }
 public any function $createControllerClass(required string name, string controllerPaths = $get("controllerPath"), string type = "controller") {` + wheelsCreateControllerClass + `}
 </cfscript>`,
		"wheels/events/init/views.cfm": `<cfscript>application.$wheels.controllerPath = "/app/controllers";</cfscript>`,
		"wheels/tests/runner.cfm": `<cfscript>
 variables.setEnv = function() {
	local.AssetPath = "/wheels/tests/_assets/";
	application.wo.set(controllerPath = local.AssetPath & "controllers");
 };
 </cfscript>`,
		"wheels/tests/_assets/controllers/Controller.cfc": `component { function base(){} }`,
		"wheels/tests/_assets/controllers/Posts.cfc":      `component extends="Controller" { function testOnly(){} }`,
		"app/controllers/Controller.cfc":                  `component { function base(){} }`,
		"app/controllers/Posts.cfc":                       `component extends="Controller" { function appOnly(){} }`,
		"wheels/tests/specs/ControllerSpec.cfc": `component { function run(){
 var a = application.wo.controller("dummy", {x = 1});
 a.base(); a.missing();
 var b = application.wo.controller(name = "Posts");
 b.testOnly();
 var c = application.wo.controller(runtimeName);
 c.base();
 application.wo.controller("Posts").testOnly();
 } }`,
		"site/Page.cfc": `component { function run(){
 var p = application.wo.controller("Posts");
 p.appOnly(); p.testOnly();
 } }`,
		"wheels/tests/restored/setup.cfm": `<cfscript>
 application.wo.set(controllerPath = "/wheels/tests/_assets/controllers");
 application.wheels.controllerPath = saved;
 </cfscript>`,
		"wheels/tests/restored/RestoreSpec.cfc": `component { function run(){
 var r = application.wo.controller("Posts");
 r.testOnly();
 } }`,
	}
}

func wheelsControllerResolver() *Resolver {
	return &Resolver{Resolvers: []parser.Resolver{{
		Match: `^(?:variables\.|arguments\.)?(?:application\.wo)$`, Resolve: "wheels.Global",
		Prefix: "application.wo|variables.application.wo|arguments.application.wo", Anchored: true, DynamicIfMissing: true,
	}}}
}

// TestAWheelsControllerIsTheClassItsPathHolds: controller( "name" ) returns
// the controller class $createControllerClass instantiates, under the
// controllerPath that governs the calling file: a test runner's literal one
// for its specs, the framework default elsewhere. A name with no file of its
// own is the path's Controller.cfc.
func TestAWheelsControllerIsTheClassItsPathHolds(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, wheelsControllerFiles())

	expectReasons(t, reasonsWith(t, wheelsControllerResolver(), dir, "wheels/tests/specs/ControllerSpec.cfc"), map[string]string{
		"a.base":                             "",
		"a.missing":                          "method 'missing' not found in Controller",
		"b.testOnly":                         "",
		"c.base":                             "variable 'c' has no component ref",
		"application.wo.controller.testOnly": "",
	})

	expectReasons(t, reasonsWith(t, wheelsControllerResolver(), dir, "site/Page.cfc"), map[string]string{
		"p.appOnly":  "",
		"p.testOnly": "method 'testOnly' not found in Posts",
	})

	// A computed write of controllerPath beside the spec withholds the type,
	// even beside a literal one.
	expectReasons(t, reasonsWith(t, wheelsControllerResolver(), dir, "wheels/tests/restored/RestoreSpec.cfc"), map[string]string{
		"r.testOnly": "variable 'r' has no component ref",
	})
}

// TestAWheelsControllerNeedsThePinnedClassLookup: the rule reads the call only
// once Global's $createControllerClass is the source it describes.
func TestAWheelsControllerNeedsThePinnedClassLookup(t *testing.T) {
	dir := t.TempDir()
	files := wheelsControllerFiles()
	files["wheels/global/objects.cfm"] = strings.Replace(files["wheels/global/objects.cfm"], `local.fileName != "Controller"`, `local.fileName != "Base"`, 1)
	writeFiles(t, dir, files)

	expectReasons(t, reasonsWith(t, wheelsControllerResolver(), dir, "wheels/tests/specs/ControllerSpec.cfc"), map[string]string{
		"b.testOnly": "variable 'b' has no component ref",
	})
}

// TestAVariableAssignedInAnotherFunctionIsTypedByThatAssignment: TestBox's
// beforeAll() assigns what run()'s specs read, through the variables scope.
// A var in the assigning function never left it, and a local of the same name
// in the reading function (a for-in loop variable here) hides the variable.
func TestAVariableAssignedInAnotherFunctionIsTypedByThatAssignment(t *testing.T) {
	dir := t.TempDir()
	files := wheelsControllerFiles()
	files["wheels/tests/specs/SetupSpec.cfc"] = `component {
 function beforeAll() {
	shared = application.wo.controller("Posts");
	var hidden = application.wo.controller("Posts");
 }
 function run() {
	shared.testOnly();
	hidden.testOnly();
 }
 function shadowed(list) {
	for (var shared in list) {
		shared.base();
	}
 }
 }`
	writeFiles(t, dir, files)

	expectReasons(t, reasonsWith(t, wheelsControllerResolver(), dir, "wheels/tests/specs/SetupSpec.cfc"), map[string]string{
		"shared.testOnly": "",
		"hidden.testOnly": "variable 'hidden' has no component ref",
		"shared.base":     "variable 'shared' has no component ref",
	})
}
