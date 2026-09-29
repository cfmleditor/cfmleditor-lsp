package resolve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// TestAnInheritedCallIntoAMissingBaseBlamesTheBase: a bare, this. or super.
// call that the file does not declare is looked for up the extends chain, and
// when a link of that chain names no file the method was never looked for.
// Reporting "not found in extends chain" blamed the call, once per call — 56,000
// entries over the corpus for about a dozen missing bases. The answer names the
// first link that does not resolve, however far up the chain it is, and a
// chain that resolves throughout still reports the method missing.
func TestAnInheritedCallIntoAMissingBaseBlamesTheBase(t *testing.T) {
	dir := t.TempDir()

	for name, src := range map[string]string{
		"Direct.cfc":   `component extends="vendor.Missing" { function a() { helper(); this.helper(); super.helper(); } }`,
		"Middle.cfc":   `component extends="vendor.AlsoMissing" { function mid() {} }`,
		"Indirect.cfc": `component extends="Middle" { function a() { helper(); } }`,
		"Base.cfc":     `component { function base() {} }`,
		"Whole.cfc":    `component extends="Base" { function a() { helper(); } }`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	r := &Resolver{FS: vfs.OS{}, Index: index.New()}

	reasons := func(name string) []string {
		file := filepath.Join(dir, name)

		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}

		pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true})

		var out []string

		for _, c := range pr.AllCalls() {
			out = append(out, r.CanResolveCall(&c, pr, dir))
		}

		return out
	}

	for _, tc := range []struct {
		file, base string
		calls      int
	}{
		{"Direct.cfc", "vendor.Missing", 3},
		{"Indirect.cfc", "vendor.AlsoMissing", 1},
	} {
		got := reasons(tc.file)
		if len(got) != tc.calls {
			t.Fatalf("%s: %d calls, want %d: %q", tc.file, len(got), tc.calls, got)
		}

		for _, reason := range got {
			if base, ok := MissingBaseOf(reason); !ok || base != tc.base {
				t.Errorf("%s: reason %q, want the chain to break at %s", tc.file, reason, tc.base)
			}
		}
	}

	if got := reasons("Whole.cfc"); len(got) != 1 || got[0] != "not found in extends chain" {
		t.Errorf("a chain that resolves throughout: got %q, want the method reported missing", got)
	}
}

// TestAReceiverTheFileNeverDeclaresIsTheMissingBases: `print` in a CommandBox
// command and `$assert` in a TestBox spec are declared or injected by the base,
// so with the base missing a call on one is as unchecked as an inherited
// method. A receiver the file declares — however it declares it — is the
// file's, and is still reported as having no component. The arrow-function
// `var` and the closure parameter are the parser's to declare: a text check
// did it once, and read `f( print )` as declaring `print`.
func TestAReceiverTheFileNeverDeclaresIsTheMissingBases(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Spec.cfc")
	// A base no path answers; testbox.system.BaseSpec itself now resolves to
	// the bundled TestBox API (frameworkapi.Namespaced).
	src := `component extends="vendor.missing.BaseSpec" {
	function run( arg ) {
		print.line( "x" );
		variables.$assert.isTrue( true );
		arg.go();
		arguments.arg.go();
		it( "a", () => { var t = prepareMock( 1 ); t.go(); } );
		it( "b", function( ctx ) { ctx.go(); } );
		try { f(); } catch ( any e ) { e.getMessage(); }
		local.q = 1;
		local.q.go();
	}
}`

	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), src, &parser.ParseOptions{ExtractCalls: true})
	r := &Resolver{FS: vfs.OS{}, Index: index.New()}
	got := map[string]bool{}

	for _, c := range pr.AllCalls() {
		if c.Variable == "" {
			continue
		}

		_, missing := MissingBaseOf(r.CanResolveCall(&c, pr, dir))
		got[c.Variable] = missing
	}

	for v, want := range map[string]bool{
		"print":             true,
		"variables.$assert": true,
		"arg":               false,
		"arguments.arg":     false,
		"t":                 false,
		"ctx":               false,
		"e":                 false,
		"local.q":           false,
	} {
		if missing, ok := got[v]; !ok {
			t.Errorf("no call recorded on %s: %v", v, got)
		} else if missing != want {
			t.Errorf("%s: blamed on the missing base = %v, want %v", v, missing, want)
		}
	}
}

// TestACallOnAComponentWithAMissingBaseBlamesTheBase: a method a component
// does not declare may be its base's, so when the component's own chain
// breaks the method was never looked for. ContentBox's services extend
// cborm's VirtualEntityService, and without cborm every findWhere and save
// on one was "method not found" — the call and a chain hop alike. A method
// missing from a component whose chain resolves is still reported.
func TestACallOnAComponentWithAMissingBaseBlamesTheBase(t *testing.T) {
	dir := t.TempDir()

	for name, src := range map[string]string{
		"Service.cfc": `component extends="cborm.models.VirtualEntityService" { function own() {} }`,
		"Plain.cfc":   `component { function own() {} }`,
		"Caller.cfc": `component {
	function f() {
		var svc = new Service();
		svc.findWhere();
		svc.own();
		var plain = new Plain();
		plain.missing();
	}
}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	file := filepath.Join(dir, "Caller.cfc")

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true})
	r := &Resolver{FS: vfs.OS{}, Index: index.New()}
	got := map[string]string{}

	for _, c := range pr.AllCalls() {
		if c.Variable != "" {
			got[c.Variable+"."+c.FuncName] = r.CanResolveCall(&c, pr, dir)
		}
	}

	if base, ok := MissingBaseOf(got["svc.findWhere"]); !ok || base != "cborm.models.VirtualEntityService" {
		t.Errorf("svc.findWhere: %q, want the chain to break at cborm.models.VirtualEntityService", got["svc.findWhere"])
	}

	if got["svc.own"] != "" {
		t.Errorf("svc.own: %q, want resolved", got["svc.own"])
	}

	if want := "method 'missing' not found in Plain"; got["plain.missing"] != want {
		t.Errorf("plain.missing: %q, want %q", got["plain.missing"], want)
	}
}
