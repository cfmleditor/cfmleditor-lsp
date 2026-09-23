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

// kernelFixture is a component whose getSandBox returns a dynamic value, the
// shape of kernel2.getSandBox, which builds its component path at runtime.
func kernelFixture(t *testing.T) (dir string, r *Resolver) {
	t.Helper()

	dir = t.TempDir()
	p := filepath.Join(dir, "Kernel.cfc")

	if err := os.WriteFile(p, []byte("<cfcomponent></cfcomponent>"), 0o600); err != nil {
		t.Fatal(err)
	}

	u := cfpath.ToURI(p)
	r = &Resolver{FS: vfs.OS{}, Index: index.New()}
	r.Index.IndexFileFromResult(u, []parser.FunctionDef{
		{Name: "getSandBox", URI: u, ReturnComponent: "$any"},
		{Name: "getName", URI: u},
		{Name: "init", URI: u},
	}, nil)

	return dir, r
}

// TestChainThroughDynamicHopIsDynamic covers
// kernel.getSandBox("x").getAttendanceObj().getLog(): once a hop returns $any,
// the walk used to ask $any for the next method and report
// "method 'getAttendanceObj' not found in $any".
func TestChainThroughDynamicHopIsDynamic(t *testing.T) {
	dir, r := kernelFixture(t)

	call := parser.CallSite{
		FuncName:  "getLog",
		Component: "Kernel",
		Chain:     []string{"getSandBox", "getAttendanceObj"},
	}

	target, reason := r.ResolveCallTarget(call, &parser.ParseResult{}, dir)
	if reason != "" {
		t.Fatalf("unresolved: %s", reason)
	}

	if target.Kind != TargetDynamic {
		t.Errorf("landed as %q, want %q", target.Kind, TargetDynamic)
	}
}

// TestMissingComponentIsReportedAsMissing separates a component that names no
// file from a method a real component lacks. The first is nearly always a
// componentResolver producing a path that does not exist, and reporting it as
// the second sent people looking for a method in a file that was never there.
func TestMissingComponentIsReportedAsMissing(t *testing.T) {
	dir, r := kernelFixture(t)

	for _, tc := range []struct {
		name string
		call parser.CallSite
		want string
	}{
		{
			name: "real component, missing method",
			call: parser.CallSite{FuncName: "nowhere", Component: "Kernel"},
			want: "method 'nowhere' not found in Kernel",
		},
		{
			name: "invented component",
			call: parser.CallSite{FuncName: "hasFunction", Component: "packages.tass.injectorcontroller"},
			want: "component 'packages.tass.injectorcontroller' does not exist",
		},
		{
			name: "invented component mid-chain",
			call: parser.CallSite{FuncName: "getData", Component: "packages.tass.debuggingservice", Chain: []string{"getDebugger"}},
			want: "component 'packages.tass.debuggingservice' does not exist (chain hop 'getDebugger'",
		},
		{
			name: "one real alternative is enough",
			call: parser.CallSite{FuncName: "nowhere", Component: "packages.gone.service|Kernel"},
			want: "method 'nowhere' not found in packages.gone.service|Kernel",
		},
	} {
		_, reason := r.ResolveCallTarget(tc.call, &parser.ParseResult{}, dir)
		if !strings.HasPrefix(reason, tc.want) {
			t.Errorf("%s: reason %q, want prefix %q", tc.name, reason, tc.want)
		}
	}
}

// TestChainThroughUntypedInitKeepsTheObject covers
// createObject("java", "CategoryChartBuilder").init().width(1): a stub's or a
// CFC's init() often declares no return type, and the walk offered init() to
// the resolvers, which cannot answer it, instead of keeping the object.
func TestChainThroughUntypedInitKeepsTheObject(t *testing.T) {
	dir, r := kernelFixture(t)

	call := parser.CallSite{FuncName: "getName", Component: "Kernel", Chain: []string{"init"}}

	target, reason := r.ResolveCallTarget(call, &parser.ParseResult{}, dir)
	if reason != "" {
		t.Fatalf("unresolved: %s", reason)
	}

	if target.Kind != TargetComponent || target.FuncName != "getName" {
		t.Errorf("landed on %s %s, want the component's getName", target.Kind, target.FuncName)
	}
}

// TestDynamicIfMissingSilencesOnlyItsOwnGuesses covers the config switch for
// a broad catch-all such as get$1(). A component it produces that names no
// file is its guess failing, and is accepted as dynamic. A missing component
// named any other way — a service resolver's, a literal createObject — is
// still a finding, and so is a missing method on one the catch-all got right.
func TestDynamicIfMissingSilencesOnlyItsOwnGuesses(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "packages", "tass"), 0o750); err != nil {
		t.Fatal(err)
	}

	tools := filepath.Join(dir, "packages", "tass", "pagetools.cfc")
	if err := os.WriteFile(tools, []byte(`<cfcomponent><cffunction name="render"></cffunction></cfcomponent>`), 0o600); err != nil {
		t.Fatal(err)
	}

	resolvers := []parser.Resolver{
		{Match: `getService("$1")`, Resolve: "packages.$1.service", Prefix: "getService"},
		{Match: `get([A-Za-z]+)\(\)`, Resolve: "packages.tass.${1:lower}", Prefix: "get", DynamicIfMissing: true},
	}

	src := `component {
	function work() {
		var t = obj.getTable();
		t.setWidth(1);
		var s = getService("nope");
		s.run();
		var c = createObject("component", "packages.tass.gone");
		c.go();
		var p = obj.getPageTools();
		p.render();
		p.missingMethod();
	}
}`

	file := filepath.Join(dir, "Work.cfc")
	pr := parser.ParseWithOptions(cfpath.ToURI(file), src, parser.ParseOptions{Resolvers: resolvers, ExtractCalls: true, ScanAllScopes: true})
	r := &Resolver{FS: vfs.OS{}, Index: index.New(), Resolvers: resolvers, Mappings: map[string]string{"packages": filepath.Join(dir, "packages")}}

	want := map[string]string{
		"setWidth":      "", // the catch-all's guess names nothing: dynamic
		"run":           "component 'packages.nope.service' does not exist",
		"go":            "component 'packages.tass.gone' does not exist",
		"render":        "", // the catch-all got it right
		"missingMethod": "method 'missingMethod' not found in packages.tass.pagetools",
	}

	for _, c := range pr.AllCalls() {
		w, ok := want[c.FuncName]
		if !ok {
			continue
		}

		delete(want, c.FuncName)

		if _, reason := r.ResolveCallTarget(c, pr, dir); !strings.HasPrefix(reason, w) || (w == "" && reason != "") {
			t.Errorf("%s: reason %q, want prefix %q", c.FuncName, reason, w)
		}
	}

	if len(want) > 0 {
		t.Errorf("calls not recorded: %v", want)
	}
}
