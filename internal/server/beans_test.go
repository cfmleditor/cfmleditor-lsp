package server

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func beansTestdataDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "beans")
}

func TestBuildBeanMap_Namespaces(t *testing.T) {
	dir := beansTestdataDir()
	beanPaths := map[string]string{
		"dao":      filepath.Join(dir, "dao"),
		"services": filepath.Join(dir, "services"),
	}
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// Namespace-qualified entries should be absolute paths
	tests := []struct {
		key          string
		expectedFile string
	}{
		{"userdao@dao", "dao/UserDAO.cfc"},
		{"orderdao@dao", "dao/OrderDAO.cfc"},
		{"beanuserservice@services", "services/BeanUserService.cfc"},
	}
	for _, tt := range tests {
		got := beans[tt.key]
		if !strings.HasSuffix(got, tt.expectedFile) {
			t.Errorf("beans[%q] = %q, want suffix %q", tt.key, got, tt.expectedFile)
		}
	}

	// Bare names should exist since they're unique across namespaces
	if !strings.HasSuffix(beans["userdao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao] = %q, want suffix dao/UserDAO.cfc", beans["userdao"])
	}

	if !strings.HasSuffix(beans["orderdao"], "dao/OrderDAO.cfc") {
		t.Errorf("beans[orderdao] = %q, want suffix dao/OrderDAO.cfc", beans["orderdao"])
	}
}

func TestBuildBeanMap_DuplicateBareNames(t *testing.T) {
	dir := beansTestdataDir()
	// Both root and "dao" namespace — root walks recursively finding dao/UserDAO.cfc too
	beanPaths := map[string]string{
		"":    dir,
		"dao": filepath.Join(dir, "dao"),
	}
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// Namespace-qualified should always work
	if !strings.HasSuffix(beans["userdao@dao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao@dao] = %q, want suffix dao/UserDAO.cfc", beans["userdao@dao"])
	}
}

func TestBuildBeanMap_EmptyPaths(t *testing.T) {
	beans := buildBeanMap(nil, vfs.OS{})
	if len(beans) != 0 {
		t.Errorf("expected empty map, got %d entries", len(beans))
	}

	beans = buildBeanMap(map[string]string{}, vfs.OS{})
	if len(beans) != 0 {
		t.Errorf("expected empty map, got %d entries", len(beans))
	}
}

func TestBuildBeanMap_SingleNamespace(t *testing.T) {
	dir := beansTestdataDir()
	beanPaths := map[string]string{
		"": filepath.Join(dir, "dao"),
	}
	beans := buildBeanMap(beanPaths, vfs.OS{})

	// With empty namespace, no @-qualified entries
	if _, ok := beans["userdao@"]; ok {
		t.Error("empty namespace should not produce @-qualified entries")
	}
	// Bare names should be absolute paths
	if !strings.HasSuffix(beans["userdao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao] = %q, want suffix dao/UserDAO.cfc", beans["userdao"])
	}
}

// A bare bean name is only meaningful when it identifies one file. Every bean
// used to get one regardless, and where two namespaces held the same name the
// winner was whichever came last out of `range beanPaths` — a Go map, so the
// order is randomised per process and the same name resolved to different
// components on different launches.
func TestBuildBeanMap_AmbiguousBareNameIsDropped(t *testing.T) {
	dir := t.TempDir()

	for _, ns := range []string{"alpha", "beta"} {
		sub := filepath.Join(dir, ns)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(sub, "Widget.cfc"), []byte("component {}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	beans := buildBeanMap(map[string]string{
		"alpha": filepath.Join(dir, "alpha"),
		"beta":  filepath.Join(dir, "beta"),
	}, vfs.OS{})

	if _, ok := beans["widget"]; ok {
		t.Errorf("bare name should be dropped when two files claim it, got %q", beans["widget"])
	}

	for _, want := range []string{"widget@alpha", "widget@beta"} {
		if _, ok := beans[want]; !ok {
			t.Errorf("namespace-qualified entry %q missing", want)
		}
	}
}

// The same file reached through both a namespace and its enclosing root is not
// ambiguous — it is one file seen twice — so its bare name must survive.
func TestBuildBeanMap_SameFileViaTwoNamespacesKeepsBareName(t *testing.T) {
	dir := t.TempDir()

	sub := filepath.Join(dir, "dao")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sub, "UserDAO.cfc"), []byte("component {}"), 0o644); err != nil {
		t.Fatal(err)
	}

	beans := buildBeanMap(map[string]string{"": dir, "dao": sub}, vfs.OS{})

	if !strings.HasSuffix(beans["userdao"], "dao/UserDAO.cfc") {
		t.Errorf("beans[userdao] = %q, want the single file it names", beans["userdao"])
	}
}

func TestStartupBeanRulesRefreshWithResolver(t *testing.T) {
	dir := t.TempDir()
	write := func(name, source string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Factory.cfc", `component { function getBean(string beanName) {} function addAlias(string aliasName, string beanName) {} }`)
	write("First.cfc", `component { function load() {} }`)
	write("Second.cfc", `component { function save() {} }`)
	write("startup.cfm", `<cfset f = new Factory()><cfset f.addAlias("alias", "First")>`)

	s := NewServer(nil, cflog.NewLogger(false))
	s.WorkspaceFolders = []string{dir}
	s.BeanPaths = map[string]string{"": dir}
	s.StartupFiles = []string{filepath.Join(dir, "startup.cfm")}

	s.ComponentResolvers = []config.Resolver{{Match: `getBean("$1")`, Resolve: "$1", Prefix: "getBean"}}
	if got := parser.ResolveFromCall(`getBean("alias")`, s.cfResolvers()); got != filepath.Join(dir, "First.cfc") {
		t.Fatalf("initial alias: %s", got)
	}

	write("startup.cfm", `<cfset f = new Factory()><cfset f.addAlias("alias", "Second")>`)
	s.invalidateResolveCache()

	if got := parser.ResolveFromCall(`getBean("alias")`, s.getResolver().Resolvers); got != filepath.Join(dir, "Second.cfc") {
		t.Fatalf("stale alias: %s", got)
	}

	if got := parser.ResolveFromCall(`getBean("alias")`, s.cfResolvers()); got != filepath.Join(dir, "Second.cfc") {
		t.Fatalf("editor differs: %s", got)
	}
}

func TestEditorUsesFW1ControllerSetters(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"Application.cfc": `component extends="framework" { function setupApplication() { setBeanFactory(factory); } }`,
		"framework.cfc": `component {
 function setBeanFactory(factory) {} function getBeanFactory() {} function getController() {} function getService() {}
 function autowire(cfc, factory) {} function getCachedComponent() { autowire(cfc, getBeanFactory()); }
}`,
		"beans/Service.cfc": `component { function run() {} }`,
	}
	for name, source := range files {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	s := NewServer(nil, cflog.NewLogger(false))
	s.WorkspaceFolders = []string{dir}
	s.BeanPaths = map[string]string{"": filepath.Join(dir, "beans")}
	source := `component accessors=true {
 property name="dependency";
 function setService(service) { variables.dependency=arguments.service; }
}`

	file := cfpath.ToURI(filepath.Join(dir, "controllers", "Consumer.cfc"))
	for _, pr := range []*parser.ParseResult{s.parseContentForIndex(file, source), s.parseContent(file, source)} {
		found := false

		for _, fn := range pr.Funcs {
			if fn.Name == "getDependency" {
				found = fn.ReturnComponent == filepath.Join(dir, "beans", "Service.cfc")
			}
		}

		if !found {
			t.Fatal("editor/index getter lost FW/1 injection")
		}
	}
}

func TestEditorUsesManagedSetterDependencies(t *testing.T) {
	dir := t.TempDir()

	beans := filepath.Join(dir, "beans")
	if err := os.MkdirAll(beans, 0o755); err != nil {
		t.Fatal(err)
	}

	dep := filepath.Join(beans, "Service.cfc")
	if err := os.WriteFile(dep, []byte(`component { function run() {} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	s.WorkspaceFolders = []string{dir}
	s.BeanPaths = map[string]string{"": beans}

	source := `component accessors=true {
 property name="dependency";
 function setService(service) { variables.dependency=arguments.service; }
 }`
	indexed := s.parseContentForIndex(cfpath.ToURI(filepath.Join(beans, "Consumer.cfc")), source)
	found := false

	for _, fn := range indexed.Funcs {
		if fn.Name == "getDependency" {
			found = fn.ReturnComponent == dep
		}
	}

	if !found {
		t.Fatal("closed-file index lost the injected getter")
	}

	for _, test := range []struct {
		file    string
		managed bool
	}{{filepath.Join(beans, "Consumer.cfc"), true}, {filepath.Join(dir, "Manual.cfc"), false}} {
		pr := s.parseContent(cfpath.ToURI(test.file), source)
		for _, fn := range pr.Funcs {
			if fn.Name == "getDependency" && (fn.ReturnComponent == dep) != test.managed {
				t.Errorf("%s getter type %q", test.file, fn.ReturnComponent)
			}
		}
	}

	other := filepath.Join(dir, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	replacement := filepath.Join(other, "Service.cfc")
	if err := os.WriteFile(replacement, []byte(`component { function save() {} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	s.BeanPaths = map[string]string{"": other}
	s.invalidateResolveCache()

	refreshed := s.parseContent(cfpath.ToURI(filepath.Join(other, "Consumer.cfc")), source)
	for _, fn := range refreshed.Funcs {
		if fn.Name == "getDependency" && fn.ReturnComponent != replacement {
			t.Errorf("stale bean map after config change: %q", fn.ReturnComponent)
		}
	}

	s.BeanPaths = nil
	s.invalidateResolveCache()
	s.parseContent(cfpath.ToURI(filepath.Join(dir, "Manual.cfc")), source)

	if got := s.index.LookupBean("service"); got != "" {
		t.Errorf("removed bean roots left stale type %q", got)
	}
}

func TestEditorUsesDI1LifetimeAndRefreshesFactoryConfig(t *testing.T) {
	dir := t.TempDir()
	for _, folder := range []string{"beans", "services"} {
		if err := os.MkdirAll(filepath.Join(dir, "model", folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	app := filepath.Join(dir, "Application.cfc")
	if err := os.WriteFile(app, []byte(`component extends="framework.one" {}`), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, folder := range map[string]string{"User": "beans", "Reporter": "services"} {
		if err := os.WriteFile(filepath.Join(dir, "model", folder, name+".cfc"), []byte(`component { function run() {} }`), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	s := NewServer(nil, cflog.NewLogger(false))
	s.WorkspaceFolders = []string{dir}
	s.BeanPaths = map[string]string{"": filepath.Join(dir, "model")}
	file := cfpath.ToURI(filepath.Join(dir, "model", "services", "Consumer.cfc"))

	source := `component accessors=true {property name="user"; property name="reporter"; function setUser(user) {variables.user=arguments.user;} function setReporter(reporter) {variables.reporter=arguments.reporter;}}`
	for _, pr := range []*parser.ParseResult{s.parseContentForIndex(file, source), s.parseContent(file, source)} {
		for _, fn := range pr.Funcs {
			if fn.Name == "getUser" && fn.ReturnComponent != "" {
				t.Fatal("transient getter typed")
			}

			if fn.Name == "getReporter" && fn.ReturnComponent == "" {
				t.Fatal("singleton getter not typed")
			}
		}
	}

	if err := os.WriteFile(app, []byte(`component extends="framework.one" {variables.framework={diConfig:{transients:["services"]}};}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s.invalidateResolveCache()

	pr := s.parseContent(file, source)
	for _, fn := range pr.Funcs {
		if fn.Name == "getReporter" && fn.ReturnComponent != "" {
			t.Fatal("stale factory lifetime after configuration refresh")
		}
	}
}

func TestEditorUsesDI1ConstructorDependenciesAndRefreshesDIEngine(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "model", "beans"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir, "model", "services"), 0o755); err != nil {
		t.Fatal(err)
	}

	app := filepath.Join(dir, "Application.cfc")
	if err := os.WriteFile(app, []byte(`component extends="framework.one" {}`), 0o600); err != nil {
		t.Fatal(err)
	}

	user := filepath.Join(dir, "model", "beans", "User.cfc")
	if err := os.WriteFile(user, []byte(`component {function run() {}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewServer(nil, cflog.NewLogger(false))
	s.WorkspaceFolders = []string{dir}
	s.BeanPaths = map[string]string{"": filepath.Join(dir, "model")}
	file := cfpath.ToURI(filepath.Join(dir, "model", "services", "Consumer.cfc"))

	source := `component accessors=true {property name="dependency";function init(user) {variables.dependency=arguments.user;return this;}}`
	for _, pr := range []*parser.ParseResult{s.parseContentForIndex(file, source), s.parseContent(file, source)} {
		found := false

		for _, fn := range pr.Funcs {
			if fn.Name == "getDependency" {
				found = true

				if fn.ReturnComponent != user {
					t.Fatalf("constructor getter lost: %+v", fn)
				}
			}
		}

		if !found {
			t.Fatal("missing constructor-backed getter")
		}
	}

	if err := os.WriteFile(app, []byte(`component extends="framework.one" {variables.framework={diEngine:"none"};}`), 0o600); err != nil {
		t.Fatal(err)
	}

	s.invalidateResolveCache()

	pr := s.parseContent(file, source)
	for _, fn := range pr.Funcs {
		if fn.Name == "getDependency" && fn.ReturnComponent != "" {
			t.Fatal("stale constructor type after DI engine change")
		}
	}
}

func TestEditorIndexesDI1ConstructorWithoutGenericBeanRoots(t *testing.T) {
	dir := t.TempDir()
	for _, folder := range []string{"framework", "model"} {
		if err := os.MkdirAll(filepath.Join(dir, folder), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	sources := map[string]string{
		"framework/ioc.cfc": `component {function init(folders,config={}) {} function getBean(beanName) {} function isSingleton(beanName) {} function findSetters(cfc,iocMeta) {} function beanIsTransient(singleDir,dir,beanName) {} function declareBean(beanName,dottedPath,isSingleton=true,overrides={}) {}}`,
		"model/User.cfc":    `component {function run() {}}`,
		"startup.cfm":       `<cfscript>f=new framework.ioc("/model");f.declareBean("user","model.User");</cfscript>`,
	}
	for name, source := range sources {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	s := NewServer(nil, cflog.NewLogger(false))
	s.WorkspaceFolders = []string{dir}
	s.StartupFiles = []string{filepath.Join(dir, "startup.cfm")}
	source := `component accessors=true {property name="dependency";function init(user) {variables.dependency=arguments.user;return this;}}`
	pr := s.parseContentForIndex(cfpath.ToURI(filepath.Join(dir, "model", "Consumer.cfc")), source)
	found := false

	for _, fn := range pr.Funcs {
		if fn.Name == "getDependency" {
			found = true

			if fn.ReturnComponent != filepath.Join(dir, "model", "User.cfc") {
				t.Fatalf("closed constructor getter lost: %+v", fn)
			}
		}
	}

	if !found {
		t.Fatal("constructor-only file received a shallow index")
	}
}
