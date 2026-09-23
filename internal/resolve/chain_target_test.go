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

// TestBareCallToAVariablesScopedFunctionIsDynamic covers
// `#VARIABLES._renderTemplate(...)#` in a string in a tag file, which the
// parser records as a bare call. CFML looks a bare name up in the variables
// scope, so a call to one the file assigns there is a call through that
// function-reference property, as the qualified form already was.
func TestBareCallToAVariablesScopedFunctionIsDynamic(t *testing.T) {
	src := "<cfcomponent>\n<cffunction name=\"init\">\n<cfargument name=\"render\">\n<cfset VARIABLES._render = ARGUMENTS.render>\n</cffunction>\n" +
		"<cffunction name=\"show\">\n<cfset out = '<div>#VARIABLES._render(1)#</div>'>\n</cffunction>\n</cfcomponent>"

	pr := parser.ParseWithOptions(cfpath.ToURI("/tmp/Thing.cfc"), src, parser.ParseOptions{ExtractCalls: true})
	r := &Resolver{FS: vfs.OS{}, Index: index.New()}

	target, reason := r.ResolveCallTarget(parser.CallSite{FuncName: "_render", Line: 6}, pr, "/tmp")
	if reason != "" || target.Kind != TargetDynamic {
		t.Errorf("got %q %q, want a dynamic target", target.Kind, reason)
	}

	if _, reason := r.ResolveCallTarget(parser.CallSite{FuncName: "neverAssigned", Line: 6}, pr, "/tmp"); reason == "" {
		t.Error("a bare call to a name nothing assigns must stay unresolved")
	}
}
