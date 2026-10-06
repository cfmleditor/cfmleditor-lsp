package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func controllerFiles() map[string]string {
	files := mapperFiles()
	files["wheels/Controller.cfc"] = `component extends="wheels.Global" {
 function init() {$integrateComponents("wheels.controller"); $integrateComponents("wheels.view"); return this;}
 private function $integrateComponents(required string path) {local.plan=$componentIntegrationPlan(arguments.path); local.overrideSet=$mixinOverrideSet("controller"); local.iEnd=ArrayLen(local.plan); for(local.i=1;local.i<=local.iEnd;local.i++) {$integrateFunctions(local.plan[local.i].publicMethods,local.overrideSet);}}
 private function $integrateFunctions(required array publicMethods, required struct overrideSet) {` + wheelsControllerCopy + `}
 }`
	files["wheels/global/objects.cfm"] += `<cfscript>public struct function $mixinOverrideSet(required string primaryType) {return {};}</cfscript>`
	files["wheels/controller/actions.cfc"] = `component {public function redirectTo(required string route) {} private function secret() {}}`
	files["wheels/view/links.cfc"] = `component {public string function linkTo(required string text) {} public function untouched() {}}`
	files["app/controllers/Posts.cfc"] = `component extends="wheels.Controller" {public numeric function linkTo() {return 42;} function show() {redirectTo("home"); linkTo(); superLinkTo("original"); untouched(); superUntouched(); secret(); missing();}}`

	return files
}

func TestWheelsControllerPublicMixinsAndSuperAlias(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, controllerFiles())

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "app/controllers/Posts.cfc"), map[string]string{
		"redirectTo": "", "linkTo": "", "superLinkTo": "", "untouched": "",
		"superUntouched": "not found in extends chain", "secret": "not found in extends chain", "missing": "not found in extends chain",
	})
	own := r.ResolveFunc("app.controllers.Posts", "linkTo", dir)

	original := r.ResolveFunc("app.controllers.Posts", "superLinkTo", dir)
	if own == nil || own.URI != cfpath.ToURI(filepath.Join(dir, "app", "controllers", "Posts.cfc")) {
		t.Fatalf("own override lost: %+v", own)
	}

	if original == nil || original.URI != cfpath.ToURI(filepath.Join(dir, "wheels", "view", "links.cfc")) || len(original.Arguments) != 1 || !original.Arguments[0].Required {
		t.Fatalf("super target/signature: %+v", original)
	}
}

func TestWheelsControllerLoaderBoundaries(t *testing.T) {
	tests := []struct{ name, file, from, to string }{
		{"conditional init", "wheels/Controller.cfc", `$integrateComponents("wheels.controller");`, `if(enabled){$integrateComponents("wheels.controller");}`},
		{"unused loader", "wheels/Controller.cfc", `$integrateComponents("wheels.controller"); $integrateComponents("wheels.view");`, `/* $integrateComponents("wheels.controller"); $integrateComponents("wheels.view"); */`},
		{"no-op copy", "wheels/Controller.cfc", wheelsControllerCopy, ``},
		{"wrong base", "wheels/Controller.cfc", `extends="wheels.Global"`, ``},
		{"own init", "app/controllers/Posts.cfc", `function show()`, `function init(){return this;} function show()`},
		{"own loader", "app/controllers/Posts.cfc", `function show()`, `function $integrateComponents(path){} function show()`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			files := controllerFiles()
			files[tt.file] = strings.Replace(files[tt.file], tt.from, tt.to, 1)
			writeFiles(t, dir, files)

			r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
			if d := r.ResolveFunc("app.controllers.Posts", "redirectTo", dir); d != nil {
				t.Fatalf("unproven integration: %+v", d)
			}
		})
	}
}

func TestWheelsMapperGlobalPublicAPI(t *testing.T) {
	dir := t.TempDir()
	files := mapperFiles()
	files["wheels/Global.cfc"] = `component {include "global/objects.cfm"; public function globalOwn(required string name){} private function hidden(){} public function get(){} public function controller(){}}`
	files["wheels/global/objects.cfm"] += `<cfscript>public function includedGlobal() {} private function includedPrivate() {}</cfscript>`
	files["wheels/Mapper.cfc"] = strings.Replace(files["wheels/Mapper.cfc"], mapperGlobalCopy, mapperGlobalCopy+wheelsGlobalIncludeCopy, 1)
	writeFiles(t, dir, files)

	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
	for _, name := range []string{"globalOwn", "includedGlobal"} {
		if r.ResolveFunc("wheels.Mapper", name, dir) == nil {
			t.Errorf("public Global method %s missing", name)
		}
	}

	for _, name := range []string{"hidden", "includedPrivate", "get", "controller"} {
		if r.ResolveFunc("wheels.Mapper", name, dir) != nil {
			t.Errorf("excluded Global method %s imported", name)
		}
	}
}

func TestWheelsControllerKeepsInheritedOverride(t *testing.T) {
	dir := t.TempDir()
	files := controllerFiles()
	files["app/controllers/Base.cfc"] = `component extends="wheels.Controller" {public numeric function linkTo(){return 42;}}`
	files["app/controllers/Posts.cfc"] = strings.Replace(files["app/controllers/Posts.cfc"], `extends="wheels.Controller" {public numeric function linkTo() {return 42;}`, `extends="app.controllers.Base" {`, 1)
	writeFiles(t, dir, files)

	r := &Resolver{}
	reasonsWith(t, r, dir, "app/controllers/Posts.cfc")

	fd := r.ResolveFunc("app.controllers.Posts", "linkTo", dir)
	if fd == nil || fd.URI != cfpath.ToURI(filepath.Join(dir, "app", "controllers", "Base.cfc")) {
		t.Fatalf("inherited override lost %+v", fd)
	}
}

// TestAWheelsMixinsBareCallsAreTheControllers: wheels/view/*.cfc and
// wheels/controller/*.cfc are never instantiated; Controller copies their
// public methods into every controller, so a bare call in one is a call on
// the controller — its own methods, Global's and what Global includes, and
// the other packages' public methods.
func TestAWheelsMixinsBareCallsAreTheControllers(t *testing.T) {
	files := controllerFiles()
	files["wheels/view/links.cfc"] = `component {public string function linkTo(required string text) {} public function untouched() {
	redirectTo("home"); $mixinOverrideSet("x"); init(); secret(); missing(); }}`

	dir := t.TempDir()
	writeFiles(t, dir, files)
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "wheels/view/links.cfc"), map[string]string{
		"redirectTo": "", "$mixinOverrideSet": "", "init": "",
		"secret": "no qualifier, not in file", "missing": "no qualifier, not in file",
	})

	// A Controller that does not integrate the package proves nothing.
	unproven := controllerFiles()
	unproven["wheels/view/links.cfc"] = files["wheels/view/links.cfc"]
	unproven["wheels/Controller.cfc"] = strings.Replace(unproven["wheels/Controller.cfc"], `$integrateComponents("wheels.view");`, ``, 1)

	dir = t.TempDir()
	writeFiles(t, dir, unproven)

	got := reasonsWith(t, &Resolver{}, dir, "wheels/view/links.cfc")
	for _, name := range []string{"redirectTo", "$mixinOverrideSet", "init"} {
		if got[name] == "" {
			t.Errorf("%s resolved through a Controller that does not integrate wheels.view", name)
		}
	}
}
