package unresolved

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func TestWriteKnownIssuesIsProjectRelative(t *testing.T) {
	base := filepath.FromSlash("/work/tassweb")
	results := []Call{
		{File: filepath.FromSlash("/work/tassweb/webroot/b.cfm"), Line: 9, Function: "tableCell", Reason: "no qualifier, not in file"},
		{File: filepath.FromSlash("/work/tassweb/packages/a.cfc"), Line: 1, Variable: "svc", Function: "run", Reason: "method 'run' not found in x"},
		{File: filepath.FromSlash("/work/kiosk/c.cfm"), Line: 0, Function: "go", Reason: "no qualifier, not in file"},
	}

	var out bytes.Buffer
	if skipped := WriteKnownIssues(&out, results, base, false, RegenerateHint, "test"); skipped != 1 {
		t.Errorf("skipped %d, want 1 (the sibling project)", skipped)
	}

	var entries []string

	for l := range strings.SplitSeq(strings.TrimSpace(out.String()), "\n") {
		if !strings.HasPrefix(l, "#") {
			entries = append(entries, l)
		}
	}

	want := []string{
		"packages/a.cfc:2: svc.run (method 'run' not found in x)",
		"webroot/b.cfm:10: tableCell (no qualifier, not in file)",
	}

	if strings.Join(entries, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(entries, "\n"), strings.Join(want, "\n"))
	}

	if strings.Contains(out.String(), "/work/") {
		t.Error("an absolute path was written")
	}

	out.Reset()
	WriteKnownIssues(&out, results, base, true, RegenerateHint, "test")

	if !strings.Contains(out.String(), "\n../kiosk/c.cfm:1: go (no qualifier, not in file)\n") {
		t.Errorf("--include-workspace did not write the sibling as a ../ path:\n%s", out.String())
	}
}

// TestSplitByTargetGivesEachReportItsOwnDirectory covers several generated
// files of one kind: each gets the calls under its own directory, the deepest
// when directories nest, and a call under none is handed back.
func TestSplitByTargetGivesEachReportItsOwnDirectory(t *testing.T) {
	top := filepath.FromSlash("/work/.cfmleditor-unresolved.txt")
	web := filepath.FromSlash("/work/tassweb/.cfmleditor-unresolved.txt")
	calls := []Call{
		{File: filepath.FromSlash("/work/tassweb/a.cfc")},
		{File: filepath.FromSlash("/work/kiosk/b.cfm")},
		{File: filepath.FromSlash("/elsewhere/c.cfm")},
	}

	by, rest := SplitByTarget(calls, []string{top, web})

	if len(by[web]) != 1 || by[web][0].File != calls[0].File {
		t.Errorf("tassweb report: %+v", by[web])
	}

	if len(by[top]) != 1 || by[top][0].File != calls[1].File {
		t.Errorf("top report: %+v", by[top])
	}

	if len(rest) != 1 || rest[0].File != calls[2].File {
		t.Errorf("rest: %+v", rest)
	}
}

func TestIsBuiltin(t *testing.T) {
	// A tag called as a function is built in too: cfheader( name = "x" ).
	// "cf" alone and a cf-prefixed name no tag has are not.
	for name, want := range map[string]bool{
		"trim": true, "TRIM": true, "append": true, "someUserDefinedFunctionXyz": false,
		"cfheader": true, "CFHTTP": true, "cf": false, "cfNotATag": false,
	} {
		if got := IsBuiltin(name); got != want {
			t.Errorf("IsBuiltin(%q) = %v, want %v", name, got, want)
		}
	}

	for name, want := range map[string]bool{"append": true, "APPEND": true, "notARealMemberFunctionXyz": false} {
		if got := IsMemberFunction(name); got != want {
			t.Errorf("IsMemberFunction(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestConfiguredResolverReturnCrossesFiles covers the batch-indexing half of
// resolver-backed returns. Factory.make() is parsed before Page calls it, so
// its configured return must be stored in the index rather than existing only
// in the same-file parse used during scanning.
func TestConfiguredResolverReturnCrossesFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Model.cfc":   `component { function run() {} }`,
		"Factory.cfc": `component { function make() { return locate("model"); } }`,
		"Page.cfc": `component {
	function check() {
		var factory = new Factory();
		factory.make().run();
	}
}`,
	}

	paths := make([]string, 0, len(files))
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}

		paths = append(paths, path)
	}

	opt := &Options{
		Resolvers: []parser.Resolver{{
			Match: `locate\("model"\)`, Resolve: "Model", Prefix: "locate",
		}},
		WorkspaceFolders: []string{dir},
	}

	page := filepath.Join(dir, "Page.cfc")
	if rep := Scan(vfs.OS{}, paths, []string{page}, opt); len(rep.Calls) != 0 {
		for i := range rep.Calls {
			t.Errorf("%s (%s)", rep.Calls[i].CallText(), rep.Calls[i].Reason)
		}
	}
}

// TestAMissingBaseIsOneEntryPerFile: every inherited call in a file whose
// extends chain breaks is unchecked for the same reason, so the report holds
// one entry for the file, on its extends line and counting the calls — `toBe`
// among them, since it is chained on `expect`, and `print.line`, since `print`
// is not declared here and so is the base's — rather than one per call. `svc`
// is an argument, so the base cannot explain it, and it is reported as before.
func TestAMissingBaseIsOneEntryPerFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Spec.cfc")
	src := "/** a spec */\ncomponent extends=\"vendor.missing.BaseSpec\" {\n" +
		"\tfunction run( svc ) {\n\t\tdescribe( \"x\", function() {\n\t\t\texpect( 1 ).toBe( 1 );\n\t\t\tit( \"y\", function() {} );\n" +
		"\t\t\tsvc.missing();\n\t\t\tprint.line( \"z\" );\n\t\t} );\n\t}\n}\n"

	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	rep := Scan(vfs.OS{}, []string{file}, nil, &Options{})

	var got []string

	for i := range rep.Calls {
		c := &rep.Calls[i]
		got = append(got, fmt.Sprintf("%d: %s (%s)", c.Line+1, c.CallText(), c.Reason))
	}

	want := []string{
		"2: vendor.missing.BaseSpec (base component does not resolve; 5 inherited calls not checked)",
		"7: svc.missing (variable 'svc' has no component ref)",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if bases := MissingBases(rep.Calls); len(bases) != 1 || bases[0] != (MissingBase{Component: "vendor.missing.BaseSpec", Files: 1, Calls: 5}) {
		t.Errorf("MissingBases: got %+v", bases)
	}
}

// TestCallsOnAComponentWithAMissingBaseAreOneEntry: calls in a file made on
// a component whose chain breaks are one entry for that base, on the first
// of them, beside the file's own missing base, which stays on its extends
// line.
func TestCallsOnAComponentWithAMissingBaseAreOneEntry(t *testing.T) {
	dir := t.TempDir()

	for name, src := range map[string]string{
		"Service.cfc": "component extends=\"vendor.orm.VirtualEntityService\" {}\n",
		"Handler.cfc": "component extends=\"vendor.missing.EventHandler\" {\n" +
			"\tproperty name=\"svc\" inject=\"Service@app\";\n" +
			"\tfunction index( event ) {\n\t\tsvc.findWhere();\n\t\tsvc.save();\n\t\tsetNextEvent();\n\t}\n}\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{filepath.Join(dir, "Service.cfc"), filepath.Join(dir, "Handler.cfc")}
	rep := Scan(vfs.OS{}, files, files[1:], &Options{})

	var got []string

	for i := range rep.Calls {
		c := &rep.Calls[i]
		got = append(got, fmt.Sprintf("%d: %s (%s)", c.Line+1, c.CallText(), c.Reason))
	}

	want := []string{
		"1: vendor.missing.EventHandler (base component does not resolve; 1 inherited call not checked)",
		"4: vendor.orm.VirtualEntityService (calls a component whose chain breaks at vendor.orm.VirtualEntityService, which does not resolve; 2 calls not checked)",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestTheReportReadsBeanPathsAndPropertyResolvers: the editor types an
// injected property from the workspace's beanPaths and propertyResolvers, and
// the report ignored both, so it disagreed with the editor about the same
// line. Both cases here are ones the defaults cannot answer: a caller beside a
// Mailer.cfc of its own, when the bean named Mailer@app is another, and a
// `thing:` injection only a propertyResolver knows.
func TestTheReportReadsBeanPathsAndPropertyResolvers(t *testing.T) {
	dir := t.TempDir()

	for name, src := range map[string]string{
		"beans/Mailer.cfc":    "component { function send() {} }",
		"lib/Clock.cfc":       "component { function tick() {} }",
		"handlers/Mailer.cfc": "component {}",
		"handlers/Main.cfc": "component {\n\tproperty name=\"mailer\" inject=\"Mailer@app\";\n" +
			"\tproperty name=\"clock\" inject=\"thing:Clock\";\n" +
			"\tfunction f() {\n\t\tmailer.send();\n\t\tclock.tick();\n\t}\n}\n",
	} {
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(full, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var files []string

	for _, name := range []string{"beans/Mailer.cfc", "lib/Clock.cfc", "handlers/Mailer.cfc", "handlers/Main.cfc"} {
		files = append(files, filepath.Join(dir, filepath.FromSlash(name)))
	}

	opt := &Options{
		BeanPaths:         map[string]string{"app": filepath.Join(dir, "beans")},
		PropertyResolvers: []parser.PropertyResolver{{Match: "thing:$1", Resolve: "lib.$1", Attribute: "inject"}},
		WorkspaceFolders:  []string{dir},
	}

	if rep := Scan(vfs.OS{}, files, files[3:], opt); len(rep.Calls) != 0 {
		for i := range rep.Calls {
			t.Errorf("%s (%s)", rep.Calls[i].CallText(), rep.Calls[i].Reason)
		}
	}
}

// Registration rules must be available while indexing returns, as well as
// while parsing callers; otherwise another file's getter loses the alias type.
func TestReportUsesStartupBeanAliasesBeforeIndexing(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"Factory.cfc":           `component { function getBean(string beanName) {} function addAlias(string aliasName, string beanName) {} }`,
		"Content.cfc":           `component { function unrelated() {} }`,
		"beans/ContentBean.cfc": `component { function load() {} }`,
		"startup.cfm":           `<cfscript>f = new Factory(); f.addAlias("content", "ContentBean");</cfscript>`,
		"Provider.cfc":          `component { function getContent() { return getBean("content"); } }`,
		"page.cfm": `<cfscript>
   function getBean(string beanName) {}
   content = getBean("content");
   content.load();
   content.missing();
   p = new Provider();
   value = p.getContent();
   value.load();
   ordinary = new Content();
   ordinary.load();
  </cfscript>`,
	}
	files := []string{}

	for name, source := range sources {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		files = append(files, path)
	}

	opt := &Options{WorkspaceFolders: []string{dir}, BeanPaths: map[string]string{"": filepath.Join(dir, "beans")}, StartupFiles: []string{filepath.Join(dir, "startup.cfm")}, Resolvers: []parser.Resolver{{Match: `getBean("$1")`, Resolve: "$1", Prefix: "getBean"}}}

	rep := Scan(vfs.OS{}, files, []string{filepath.Join(dir, "page.cfm")}, opt)
	if len(rep.Calls) != 2 {
		t.Fatalf("want only missing method and ordinary component error, got %+v", rep.Calls)
	}

	for _, call := range rep.Calls {
		if call.Variable != "content" && call.Variable != "ordinary" {
			t.Errorf("unexpected failure: %+v", call)
		}
	}
}

func TestReportUsesManagedSetterDependencies(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"beans/Service.cfc": `component { function run() {} }`,
		"beans/Consumer.cfc": `component accessors=true {
   property name="dependency";
   function setService(service) { variables.dependency = arguments.service; variables.service = arguments.service; }
   function use() { variables.dependency.run(); variables.dependency.missing(); }
   function unrelated(service) { service.run(); }
  }`,
		"beans/Properties.cfc": `component accessors=true { property name="service"; }`,
		"Manual.cfc": `component {
   function setService(service) { variables.service = arguments.service; }
   function use() { variables.service.run(); }
  }`,
		"page.cfm": `<cfscript>
   c = new beans.Consumer();
   c.use();
   value = c.getDependency();
   value.run();
   properties = new beans.Properties();
   service = properties.getService();
   service.run();
  </cfscript>`,
	}
	files := []string{}

	for name, source := range sources {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		files = append(files, file)
	}

	rep := Scan(vfs.OS{}, files, nil, &Options{BeanPaths: map[string]string{"": filepath.Join(dir, "beans")}, WorkspaceFolders: []string{dir}})
	if len(rep.Calls) != 3 {
		t.Fatalf("want missing method, sibling parameter and unmanaged field errors, got %+v", rep.Calls)
	}

	for _, call := range rep.Calls {
		if call.File == filepath.Join(dir, "page.cfm") {
			t.Errorf("indexed getter lost its type: %+v", call)
		}

		if call.Function == "missing" && !strings.Contains(call.Reason, "not found in Service") {
			t.Errorf("missing method was not verified: %+v", call)
		}
	}
}

func TestReportUsesFW1ControllerSetters(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"admin/Application.cfc": `component extends="framework" {
   variables.framework.usingSubsystems=true;
   function setupApplication() { setBeanFactory(application.factory); }
  }`,
		"admin/framework.cfc": `component {
   function setBeanFactory(factory) {} function getBeanFactory() {} function getController() {} function getService() {}
   function autowire(cfc, factory) {} function getCachedComponent() { autowire(cfc, getBeanFactory()); }
  }`,
		"beans/Service.cfc": `component { function run() {} }`,
		"admin/core/controllers/Consumer.cfc": `component accessors=true {
   property name="dependency";
   function setService(service) { variables.dependency=arguments.service; }
   function use() { variables.dependency.run(); variables.dependency.missing(); }
  }`,
		"admin/core/models/Manual.cfc": `component {
   function setService(service) { variables.dependency=arguments.service; }
   function use() { variables.dependency.run(); }
  }`,
		"page.cfm": `<cfscript>
   controller=new admin.core.controllers.Consumer();
   dependency=controller.getDependency();
   dependency.run();
  </cfscript>`,
	}
	files := []string{}

	for name, source := range sources {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		files = append(files, file)
	}

	rep := Scan(vfs.OS{}, files, nil, &Options{BeanPaths: map[string]string{"": filepath.Join(dir, "beans")}, WorkspaceFolders: []string{dir}})
	if len(rep.Calls) != 2 {
		t.Fatalf("want missing method and unmanaged model, got %+v", rep.Calls)
	}

	for _, call := range rep.Calls {
		if call.Function == "missing" && !strings.Contains(call.Reason, "not found in Service") {
			t.Errorf("missing method was not checked: %+v", call)
		}
	}
}

func TestReportUsesDI1LifetimeForInjectionOnly(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"Application.cfc":             `component extends="framework.one" {}`,
		"model/beans/User.cfc":        `component { function run() {} }`,
		"model/services/Reporter.cfc": `component { function run() {} }`,
		"model/services/Consumer.cfc": `component accessors=true {
 property name="user";
 property name="reporter";
 function setUser(user) { variables.user=arguments.user; }
 function setReporter(reporter) { variables.reporter=arguments.reporter; }
 function use() { variables.user.run(); variables.reporter.run(); variables.reporter.missing(); }
 }`,
		"page.cfm": `<cfscript>
 function getBean(name) {}
 c=new model.services.Consumer();
 c.getUser().run(); c.getReporter().run();
 user=getBean("user"); user.run();
 </cfscript>
`,
	}
	files := []string{}

	for name, source := range sources {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}

		files = append(files, file)
	}

	rep := Scan(vfs.OS{}, files, nil, &Options{BeanPaths: map[string]string{"": filepath.Join(dir, "model")}, WorkspaceFolders: []string{dir}, Resolvers: []parser.Resolver{{Match: `getBean("$1")`, Resolve: "$1", Prefix: "getBean"}}})
	if len(rep.Calls) != 3 {
		t.Fatalf("want two unknown transient fields and one missing singleton method: %+v", rep.Calls)
	}

	for _, call := range rep.Calls {
		if call.Function == "missing" && !strings.Contains(call.Reason, "not found in Reporter") {
			t.Errorf("singleton method validation lost: %+v", call)
		}
	}
}
