package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

const (
	mapperInit       = `local.globalComponent = createObject("wheels.Global"); $integrateFunctions(local.globalComponent); $integrateComponents("wheels.mapper"); return this;`
	mapperIntegrate  = `local.plan = $componentIntegrationPlan(arguments.path); local.iEnd = ArrayLen(local.plan); for (local.i = 1; local.i <= local.iEnd; local.i++) { $integrateFunctions(local.plan[local.i].instance, local.plan[local.i].publicMethods); }`
	mapperCopy       = `if (ArrayLen(arguments.publicMethods)) { local.iEnd = ArrayLen(arguments.publicMethods); for (local.i = 1; local.i <= local.iEnd; local.i++) { local.m = arguments.publicMethods[local.i]; variables[local.m.name] = local.m.ref; this[local.m.name] = local.m.ref; } return; }`
	mapperGlobalCopy = `local.meta = getMetaData(arguments.componentInstance);
 local.methods = local.meta.functions; local.componentName = local.meta.fullName;
 local.excludeList = "get,controller";
 for(local.method in local.methods) {
  local.functionName = local.method.name;
  if (local.method.access == "public" && (!listFindNoCase(local.excludeList, local.functionName) || findNoCase("wheels.mapper", local.componentName))) {
   variables[local.functionName] = componentInstance[local.functionName]; this[local.functionName] = componentInstance[local.functionName];
  }
 }`
)

const mapperBuilder = `
 local.folderPath = ExpandPath("/#Replace(arguments.path, ".", "/", "all")#");
 local.fileList = DirectoryList(local.folderPath, false, "name", "*.cfc");
 local.rv = [];
 for (local.fileName in local.fileList) {
  local.componentName = Replace(local.fileName, ".cfc", "", "all");
  local.instance = CreateObject("component", "#arguments.path#.#local.componentName#");
  local.meta = GetMetaData(local.instance);
  local.fns = local.meta.functions;
  local.publicMethods = [];
  for (local.f = 1; local.f <= ArrayLen(local.fns); local.f++) {
   if (local.fns[local.f].access == "public") {
    local.ref = local.instance[local.fns[local.f].name];
    ArrayAppend(local.publicMethods, {name = local.fns[local.f].name, ref = local.ref});
   }
  }
  ArrayAppend(local.rv, {instance = local.instance, publicMethods = local.publicMethods});
 }
 return local.rv;`

func mapperFiles() map[string]string {
	return map[string]string{
		"wheels/Mapper.cfc": `component {
   function init() {` + mapperInit + `}
   function own() {}
   private function $integrateComponents(required string path) {` + mapperIntegrate + `}
   private function $integrateFunctions(required any componentInstance, array publicMethods = []) {` + mapperCopy + mapperGlobalCopy + `}
  }`,
		"wheels/Global.cfc": `component { include "global/objects.cfm"; }`,
		"wheels/global/objects.cfm": `<cfscript>
   public array function $componentIntegrationPlan(required string path) {return $buildComponentIntegrationPlan(arguments.path);}
   public array function $buildComponentIntegrationPlan(required string path) {` + mapperBuilder + `}
  </cfscript>`,
		"wheels/mapper/mapping.cfc": `component {
   public struct function $draw(required string name) { return this; }
   public struct function end() { return this; }
   private function secret() {}
   package function internalOnly() {}
   remote function remoteOnly() {}
function attributeSecret() access="private" {}
function attributePublic() access="public" {}
   function defaultPublic() {}
  }`,
		"wheels/mapper/routes.cfc":        `component { PUBLIC function route(required string path) {return this;} }`,
		"wheels/mapper/nested/hidden.cfc": `component {public function hidden() {}}`,
		"Page.cfc": `component {function run() {
   var mapper = new wheels.Mapper();
   mapper.$draw("routes").route("/").end().own();
   mapper.route();
   mapper.defaultPublic(); mapper.attributePublic(); mapper.attributeSecret();
   mapper.secret(); mapper.internalOnly(); mapper.remoteOnly(); mapper.hidden(); mapper.missing();
  }}`,
	}
}

func TestWheelsMapperIntegratedMethods(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, mapperFiles())

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{
		"mapper.$draw.route.end.own": "",
		"mapper.route":               "",
		"mapper.attributePublic":     "",
		"mapper.attributeSecret":     "method 'attributeSecret' not found in wheels.Mapper",
		"mapper.defaultPublic":       "",
		"mapper.secret":              "method 'secret' not found in wheels.Mapper",
		"mapper.internalOnly":        "method 'internalOnly' not found in wheels.Mapper",
		"mapper.remoteOnly":          "method 'remoteOnly' not found in wheels.Mapper",
		"mapper.hidden":              "method 'hidden' not found in wheels.Mapper",
		"mapper.missing":             "method 'missing' not found in wheels.Mapper",
	})

	def := r.ResolveFunc("wheels.Mapper", "route", dir)
	if def == nil || def.URI != cfpath.ToURI(filepath.Join(dir, "wheels", "mapper", "routes.cfc")) || len(def.Arguments) != 1 || !def.Arguments[0].Required {
		t.Fatalf("integrated definition/signature: %+v", def)
	}
}

func TestWheelsMapperLoaderBoundaries(t *testing.T) {
	tests := []struct{ name, file, from, to string }{
		{"unused loader", "wheels/Mapper.cfc", mapperInit, `return this;`},
		{"conditional loader", "wheels/Mapper.cfc", mapperInit, `if (enabled) {` + mapperInit + `}`},
		{"closure loader", "wheels/Mapper.cfc", mapperInit, `var load = function() {` + mapperInit + `}; return this;`},
		{"quoted loader", "wheels/Mapper.cfc", mapperInit, `var text = '$integrateComponents("wheels.mapper")'; return this;`},
		{"comment loader", "wheels/Mapper.cfc", mapperInit, `/* $integrateComponents("wheels.mapper"); */ return this;`},
		{"computed package", "wheels/Mapper.cfc", `"wheels.mapper"`, `"wheels." & kind`},
		{"foreign receiver", "wheels/Mapper.cfc", `$integrateComponents("wheels.mapper")`, `other.$integrateComponents("wheels.mapper")`},
		{"no-op plan consumer", "wheels/Mapper.cfc", mapperIntegrate, ``},
		{"no Global copier", "wheels/Mapper.cfc", mapperGlobalCopy, ``},
		{"no-op copier", "wheels/Mapper.cfc", mapperCopy, ``},
		{"no-op plan", "wheels/global/objects.cfm", `return $buildComponentIntegrationPlan(arguments.path);`, `return [];`},
		{"no-op builder", "wheels/global/objects.cfm", mapperBuilder, `return [];`},
		{"recursive scan", "wheels/global/objects.cfm", `DirectoryList(local.folderPath, false`, `DirectoryList(local.folderPath, true`},
		{"Global inheritance cycle", "wheels/Global.cfc", `component { include "global/objects.cfm"; }`, `component extends="wheels.Mapper" {}`},
		{"private helper", "wheels/global/objects.cfm", `public array function $componentIntegrationPlan`, `private array function $componentIntegrationPlan`},
		{"Global helper override", "wheels/Global.cfc", `include "global/objects.cfm";`, `include "global/objects.cfm"; public array function $componentIntegrationPlan(required string path) {return [];}`},
		{"private plan", "wheels/global/objects.cfm", `access == "public"`, `access == "private"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := mapperFiles()
			files[test.file] = strings.Replace(files[test.file], test.from, test.to, 1)
			dir := t.TempDir()
			writeFiles(t, dir, files)

			r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
			if def := r.ResolveFunc("wheels.Mapper", "route", dir); def != nil {
				t.Fatalf("unproven loader resolved %+v", def)
			}
		})
	}
}

func TestWheelsMapperAmbiguousMixinAndSourceChanges(t *testing.T) {
	dir := t.TempDir()
	files := mapperFiles()
	writeFiles(t, dir, files)

	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
	if r.ResolveFunc("wheels.Mapper", "route", dir) == nil {
		t.Fatal("initial route missing")
	}

	global := filepath.Join(dir, "wheels", "Global.cfc")
	if err := os.WriteFile(global, []byte(`component {include "global/objects.cfm"; public array function $componentIntegrationPlan(required string path) {return [];}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if r.ResolveFunc("wheels.Mapper", "$draw", dir) != nil {
		t.Fatal("new source override was ignored by the existing index")
	}

	if err := os.WriteFile(global, []byte(files["wheels/Global.cfc"]), 0o600); err != nil {
		t.Fatal(err)
	}

	if r.ResolveFunc("wheels.Mapper", "$draw", dir) == nil {
		t.Fatal("removing source override did not restore loader lookup")
	}

	route := filepath.Join(dir, "wheels", "mapper", "routes.cfc")
	if err := os.WriteFile(route, []byte(`component {private function route() {}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if r.ResolveFunc("wheels.Mapper", "route", dir) != nil {
		t.Fatal("cached public access survived edit")
	}

	if err := os.WriteFile(route, []byte(files["wheels/mapper/routes.cfc"]), 0o600); err != nil {
		t.Fatal(err)
	}

	writeFiles(t, dir, map[string]string{"wheels/mapper/duplicate.cfc": `component {public function route() {}}`})

	if r.ResolveFunc("wheels.Mapper", "route", dir) != nil {
		t.Fatal("ambiguous directory order selected a definition")
	}

	loader := filepath.Join(dir, "wheels", "Mapper.cfc")
	if err := os.WriteFile(loader, []byte(strings.Replace(files["wheels/Mapper.cfc"], mapperInit, `return this;`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}

	if r.ResolveFunc("wheels.Mapper", "$draw", dir) != nil {
		t.Fatal("cached integration survived loader edit")
	}
}

func TestWheelsMapperReturnForwardingAndOverrides(t *testing.T) {
	dir := t.TempDir()
	files := mapperFiles()
	files["wheels/Mapper.cfc"] = strings.Replace(files["wheels/Mapper.cfc"], `function own() {}`, `function own() {} function route() { return new Other(); }`, 1)
	files["Other.cfc"] = `component {function foreign() {}}`
	files["wheels/mapper/routes.cfc"] = `component {
  public struct function route(required string path) { return scope(); }
  public struct function scope() { return this; }
  public struct function recursive() {
   if (retry) { return scope().recursive().end(); }
   return this;
  }
  public struct function mixed() {if (flag) { return {}; } return this;}
  public struct function foreign() {return other.scope();}
  public struct function cycle() {return cycle();}
 }`
	files["Page.cfc"] = `component {function run() {
  var mapper = new wheels.Mapper();
  mapper.route("/").end().own();
  mapper.recursive().end().own();
  mapper.route("/").missing();
  mapper.mixed().own(); mapper.foreign().own(); mapper.cycle().own();
 }}`
	writeFiles(t, dir, files)

	r := &Resolver{}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{
		"mapper.route.end.own":     "",
		"mapper.recursive.end.own": "",
		"mapper.route.missing":     "method 'missing' not found in Mapper",
		"mapper.mixed.own":         "method 'mixed' in wheels.Mapper has no component return type (chain to 'own')",
		"mapper.foreign.own":       "method 'foreign' in wheels.Mapper has no component return type (chain to 'own')",
		"mapper.cycle.own":         "method 'cycle' in wheels.Mapper has no component return type (chain to 'own')",
	})

	def := r.ResolveFunc("wheels.Mapper", "route", dir)
	if def == nil || def.URI != cfpath.ToURI(filepath.Join(dir, "wheels", "mapper", "routes.cfc")) {
		t.Fatalf("copy did not overwrite own route: %+v", def)
	}

	original := r.Index.FunctionsForFile(def.URI)
	for _, f := range original {
		if f.Name == "route" && f.ReturnComponent == filepath.Join(dir, "wheels", "Mapper.cfc") {
			t.Fatal("bound return mutated the shared source definition")
		}
	}

	writeFiles(t, dir, map[string]string{"wheels/mapper/duplicate.cfc": `component {public function route() {return this;}}`})

	if r.ResolveFunc("wheels.Mapper", "route", dir) != nil {
		t.Fatal("ambiguous copy fell back to overwritten own method")
	}
}

func TestWheelsMapperMappedFrameworkRoot(t *testing.T) {
	dir := t.TempDir()

	files := make(map[string]string)
	for name, content := range mapperFiles() {
		files[strings.Replace(name, "wheels/", "framework-core/", 1)] = content
	}

	writeFiles(t, dir, files)
	r := &Resolver{Mappings: map[string]string{"wheels": filepath.Join(dir, "framework-core")}}
	expectReasons(t, reasonsWith(t, r, dir, "Page.cfc"), map[string]string{
		"mapper.$draw.route.end.own": "",
		"mapper.secret":              "method 'secret' not found in wheels.Mapper",
	})

	def := r.ResolveFunc("wheels.Mapper", "route", dir)
	if def == nil || def.URI != cfpath.ToURI(filepath.Join(dir, "framework-core", "mapper", "routes.cfc")) {
		t.Fatalf("mapped mixin source: %+v", def)
	}
}
