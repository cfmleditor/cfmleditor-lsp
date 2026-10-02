package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// reasonsIn indexes every .cfc under dir, then resolves each call page makes,
// keyed by its receiver, chain and name ("q.execute", "d.setUrl.send").
func reasonsIn(t *testing.T, dir, page string) map[string]string {
	t.Helper()

	return reasonsWith(t, &Resolver{}, dir, page)
}

// reasonsWith is reasonsIn with a resolver carrying settings of its own; its
// file system, index and workspace folder are filled in.
func reasonsWith(t *testing.T, r *Resolver, dir, page string) map[string]string {
	t.Helper()

	r.FS, r.WorkspaceFolders = vfs.OS{}, []string{dir}
	if r.Index == nil {
		r.Index = index.New()
	}

	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(p, ".cfc") {
			return err
		}

		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}

		r.Index.IndexFile(cfpath.ToURI(p), string(data))

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, filepath.FromSlash(page))

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	// FuncLookup as the unresolved scan passes it, so a variable assigned
	// from a call is typed by what the call returns.
	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{
		ExtractCalls: true, FuncLookup: r.FuncLookup(filepath.Dir(file)), BeanLookup: r.Index.LookupBean,
	})
	got := map[string]string{}

	calls := pr.AllCalls()
	for i := range calls {
		c := &calls[i]
		key := strings.Join(append(append([]string{c.Variable}, hopNames(c.Chain)...), c.FuncName), ".")
		key = strings.TrimPrefix(key, ".")

		got[key] = r.CanResolveCall(c, pr, filepath.Dir(file))
	}

	return got
}

func expectReasons(t *testing.T, got, want map[string]string) {
	t.Helper()

	for call, w := range want {
		if g, ok := got[call]; !ok {
			t.Errorf("%s: not recorded; got %v", call, got)
		} else if g != w {
			t.Errorf("%s: %q, want %q", call, g, w)
		}
	}
}

// TestAnEngineComponentWithoutItsSourceIsDynamic: `new Query()` and
// `new http()` are components the engine ships, imported without a path, and
// an application never has their source. They were reported as components
// that do not exist. So are Adobe's com.adobe.coldfusion.* and Lucee's
// org.lucee.cfml.* named in full. A component that is not the engine's still is.
func TestAnEngineComponentWithoutItsSourceIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"app/Page.cfc": `component {
	function f() {
		var q = new Query( datasource = "x" );
		q.execute();
		var h = new http( url = "u" );
		h.send();
		var a = new com.adobe.coldfusion.mail();
		a.send();
		var o = new org.lucee.cfml.Administrator( "web", "pw" );
		o.getMappings();
		var m = new Missing();
		m.go();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "app/Page.cfc"), map[string]string{
		"q.execute":     "",
		"h.send":        "",
		"a.send":        "",
		"o.getMappings": "",
		"m.go":          "component 'Missing' does not exist (calling 'go')",
	})
}

// TestAnEngineComponentIsCheckedAgainstItsSourceWhenTheWorkspaceHasIt: a
// Lucee checkout holds the engine's components at
// core/src/main/java/resource/component/org/lucee/cfml/, and its tests extend
// org.lucee.cfml.test.LuceeTestCase from there. Found by the end of its path,
// a call on one is checked against its real methods — bare `Query` included,
// since that is org.lucee.cfml.Query.
func TestAnEngineComponentIsCheckedAgainstItsSourceWhenTheWorkspaceHasIt(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"resource/component/org/lucee/cfml/Query.cfc":              `component { function execute() {} }`,
		"resource/component/org/lucee/cfml/test/LuceeTestCase.cfc": `component { function assertTrue() {} }`,
		"test/Page.cfc": `component {
	function f() {
		var q = new Query();
		q.execute();
		q.nope();
		var t = new org.lucee.cfml.test.LuceeTestCase();
		t.assertTrue();
		t.assertNope();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "test/Page.cfc"), map[string]string{
		"q.execute":    "",
		"q.nope":       "method 'nope' not found in Query",
		"t.assertTrue": "",
		"t.assertNope": "method 'assertNope' not found in org.lucee.cfml.test.LuceeTestCase",
	})
}

// TestAnEngineComponentIsNotAnyFileOfItsName: the last resort for a bare name
// is any indexed file of that name, which for `new dbinfo()` found whatever
// dbinfo.cfc the workspace held and reported the engine's methods missing
// from it — 28 "method 'columns' not found in dbInfo" in ContentBox. The
// engine's component is not that file; one beside the caller still wins.
func TestAnEngineComponentIsNotAnyFileOfItsName(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lib/deep/dbinfo.cfc": `component { function unrelated() {} }`,
		"app/Page.cfc": `component {
	function f() {
		var d = new dbinfo( type = "version" );
		d.version();
	}
}`,
		"lib/Own.cfc":   `component { function f() { var q = new Query(); q.nope(); } }`,
		"lib/Query.cfc": `component { function mine() {} }`,
	})

	expectReasons(t, reasonsIn(t, dir, "app/Page.cfc"), map[string]string{"d.version": ""})
	expectReasons(t, reasonsIn(t, dir, "lib/Own.cfc"), map[string]string{"q.nope": "method 'nope' not found in Query"})
}

// TestAChainHopOnMissingMethodAnswersIsAccepted: a component declaring
// onMissingMethod answers any call, and a call it answers was accepted only as
// the last call of a chain. As a hop it was "not found": Lucee's Http builds
// its setters that way, and `new Http().setUrl( u ).send()` failed on setUrl.
// What the dispatcher returns is unknown, so the rest of the chain is dynamic.
func TestAChainHopOnMissingMethodAnswersIsAccepted(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Dispatch.cfc": `component { function onMissingMethod( name, args ) { return this; } }`,
		"Page.cfc": `component {
	function f() {
		var d = new Dispatch();
		d.setUrl( "u" ).send();
	}
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "Page.cfc"), map[string]string{"d.setUrl.send": ""})
}
