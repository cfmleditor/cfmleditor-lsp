package resolve

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func wheelsFactoryFiles() map[string]string {
	files := mapperFiles()
	files["wheels/Bindings.cfc"] = `component {function configure(){map("global").to("wheels.Global");}}`
	files["wheels/WheelsTest.cfc"] = `component {$bindApplicationHelpers(); function init(){$bindApplicationHelpers();return this;} private function $bindApplicationHelpers(){` + wheelsTestBinder + `}}`
	files["wheels/global/objects.cfm"] += `<cfscript>
 public struct function mapper(){return application[$appKey()].mapper.$draw(argumentCollection=arguments);}
 public any function $createObjectFromRoot(required string path,required string fileName,string method="init") {` + wheelsRootFactory + `}
 private function hiddenGlobal(){}
 </cfscript>`
	files["wheels/events/onapplicationstart.cfc"] = `component {function start(){application.$wheels.mapper=application.wo.$createObjectFromRoot(path="wheels",fileName="Mapper",method="$init");}}`
	files["wheels/Mapper.cfc"] = strings.Replace(files["wheels/Mapper.cfc"], "function own()", "function $init(){return init();} function own()", 1)
	files["alternate/Service.cfc"] = `component {function init(){return this;} function work(){}}`
	files["Spec.cfc"] = `component extends="wheels.WheelsTest" {
 function beforeAll(){config={path="wheels",fileName="Mapper",method="$init"};}
 public struct function $mapper(){local.args=Duplicate(config);StructAppend(local.args,arguments,true);return application.wo.$createObjectFromRoot(argumentCollection=local.args);}
 function run(){
 mapper().route("/").end();
 var a=mapper();a.route();a.missing();
 $mapper().route("/").end();
 var b=$mapper();b.route();b.missing();
 var c=$mapper(path="alternate",fileName="Service",method="init");c.work();c.route();
 var d=$mapper(path="app",fileName="Service",method=selected);d.work();
 hiddenGlobal();
 }
 }`

	return files
}

func TestWheelsBoundFactories(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, wheelsFactoryFiles())

	r := &Resolver{}
	got := reasonsWith(t, r, dir, "Spec.cfc")
	expectReasons(t, got, map[string]string{"mapper.route.end": "", "$mapper.route.end": "", "a.route": "", "a.missing": "method 'missing' not found in Mapper", "b.route": "", "b.missing": "method 'missing' not found in Mapper", "d.work": "variable 'd' has no component ref", "c.work": "", "c.route": "method 'route' not found in Service", "hiddenGlobal": "not found in extends chain"})

	def := r.ResolveFunc("wheels.Global", "mapper", dir)
	if ret := r.ReturnComponentOf(def); ret != filepath.Join(dir, "wheels", "Mapper.cfc") {
		t.Fatalf("mapper return %q", ret)
	}
	// Literal construction only qualifies when the actual initializer returns self.
	factory := r.ResolveFunc("wheels.Global", "$createObjectFromRoot", dir)
	if ret := r.wheelsFactoryReturn(factory, `$createObjectFromRoot(path="wheels",fileName="Mapper",method="$init")`, dir); ret != filepath.Join(dir, "wheels", "Mapper.cfc") {
		t.Fatalf("literal factory %q", ret)
	}

	for _, expr := range []string{`$createObjectFromRoot(path=runtime,fileName="Mapper")`, `$createObjectFromRoot(path="wheels",fileName="Mapper",method="missing")`, `$createObjectFromRoot(argumentCollection=args)`} {
		if ret := r.wheelsFactoryReturn(factory, expr, dir); ret != "" {
			t.Fatalf("unproven %s -> %s", expr, ret)
		}
	}
}

func TestWheelsFactoryBoundaries(t *testing.T) {
	cases := []struct {
		name, file, from, to string
		method               string
	}{
		{"changed startup", "wheels/events/onapplicationstart.cfc", `fileName="Mapper"`, `fileName=selected`, "mapper"},
		{"changed return", "wheels/global/objects.cfm", `return application[$appKey()].mapper.$draw(argumentCollection=arguments);`, `return selected;`, "mapper"},
		{"primitive return", "wheels/global/objects.cfm", `struct function mapper`, `numeric function mapper`, "mapper"},
		{"changed root factory", "wheels/global/objects.cfm", `local.rv = Invoke(local.instance, local.method, local.argumentCollection);`, `local.rv = incoming;`, "$mapper"},
		{"changed binding", "wheels/Bindings.cfc", `to("wheels.Global")`, `to("other.Global")`, "$mapper"},
		{"no-op binder", "wheels/WheelsTest.cfc", wheelsTestBinder, `return this;`, "$mapper"},
		{"override binder", "Spec.cfc", `function run()`, `function $bindApplicationHelpers(){} function run()`, "$mapper"},
		{"config mutation", "Spec.cfc", `function run()`, `function change(){config.fileName=selected;} function run()`, "$mapper"},
		{"local config", "Spec.cfc", `config={`, `var config={`, "$mapper"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			files := wheelsFactoryFiles()
			files[tt.file] = strings.Replace(files[tt.file], tt.from, tt.to, 1)
			writeFiles(t, dir, files)
			r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}
			fd := r.ResolveFunc("Spec", tt.method, dir)

			ret := r.ReturnComponentOf(fd)
			if tt.method == "$mapper" {
				ret = r.wheelsFactoryReturn(fd, "$mapper()", dir)
			}

			if ret != "" && ret != "$any" {
				t.Fatalf("unproven factory -> %s", ret)
			}
		})
	}
}

func TestFactoryExpressionDoesNotBorrowArguments(t *testing.T) {
	content := `// maker("wrong")
 maker("right").work();`
	if got := factoryCallExpression(content, "maker", 1); got != `maker("right")` {
		t.Fatalf("expression %q", got)
	}

	if got := factoryCallExpression(`maker("a").work(); maker("b").work();`, "maker", 0); got != "" {
		t.Fatalf("ambiguous expression %q", got)
	}
}

func TestWheelsFactoryWrapperWithoutSemicolons(t *testing.T) {
	dir := t.TempDir()
	files := wheelsFactoryFiles()
	files["Spec.cfc"] = strings.Replace(files["Spec.cfc"], `local.args=Duplicate(config);StructAppend(local.args,arguments,true);return application.wo.$createObjectFromRoot(argumentCollection=local.args);`, `local.args=Duplicate(config)
 StructAppend(local.args,arguments,true)
 return application.wo.$createObjectFromRoot(argumentCollection=local.args)`, 1)
	writeFiles(t, dir, files)

	r := &Resolver{}
	got := reasonsWith(t, r, dir, "Spec.cfc")
	expectReasons(t, got, map[string]string{"$mapper.route.end": "", "b.route": ""})
}

// TestAWrapperHandsTheFactoryItsArgumentsStruct: cfwheels' plugin specs build
// a literal config and pass it to a wrapper,
// `return g.$createObjectFromRoot( argumentCollection = arguments.config )`.
// The factory Invoke()s Plugins' $init(), which returns this, so the result
// is a Plugins. A config whose fileName is not a literal gives nothing.
func TestAWrapperHandsTheFactoryItsArgumentsStruct(t *testing.T) {
	dir := t.TempDir()
	files := wheelsFactoryFiles()
	files["wheels/Plugins.cfc"] = `component {function $init(required string pluginPath){return this;} function getPlugins(){}}`
	files["PluginSpec.cfc"] = `component extends="wheels.WheelsTest" {
 function run(){
 it("loads", function(){
 var config = {
 path = "wheels",
 fileName = "Plugins",
 method = "$init",
 pluginPath = "/a"
 };
 config.pluginPath = "/b";
 PluginObj = $pluginObj(config);
 PluginObj.getPlugins();
 PluginObj.nope();
 });
 it("computed", function(){
 var config = {path = "wheels", fileName = "Plugins", method = "$init"};
 config.fileName = selected;
 other = $pluginObj(config);
 other.getPlugins();
 });
 }
 function $pluginObj(required struct config) {
 return g.$createObjectFromRoot(argumentCollection = arguments.config)
 }
 }`
	writeFiles(t, dir, files)

	got := reasonsWith(t, &Resolver{}, dir, "PluginSpec.cfc")
	expectReasons(t, got, map[string]string{
		"PluginObj.getPlugins": "",
		"PluginObj.nope":       "method 'nope' not found in Plugins",
		"other.getPlugins":     "variable 'other' has no component ref",
	})
}
