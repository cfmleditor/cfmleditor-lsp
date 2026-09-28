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

// TestACallInAClosureSeesTheClosuresOwnLocals: two tests in one spec each
// declare a `t` of their own, and each call resolves against its own. A
// closure still sees the enclosing function's locals. Before closure bodies
// were read as statements every one of these was "variable 't' has no
// component ref".
func TestACallInAClosureSeesTheClosuresOwnLocals(t *testing.T) {
	dir := t.TempDir()

	for name, src := range map[string]string{
		"A.cfc": `component { function onlyA() {} }`,
		"B.cfc": `component { function onlyB() {} }`,
		"Spec.cfc": `component {
	function run() {
		var shared = new A();
		it( "a", () => {
			var t = new A();
			t.onlyA();
			shared.onlyA();
		} );
		it( "b", function() {
			var t = new B();
			t.onlyB();
		} );
	}
}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	file := filepath.Join(dir, "Spec.cfc")

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	pr := parser.ParseWithOptions(cfpath.ToURI(file), string(data), &parser.ParseOptions{ExtractCalls: true})
	r := &Resolver{FS: vfs.OS{}, Index: index.New()}

	calls := 0

	for _, c := range pr.AllCalls() {
		if c.Variable == "" {
			continue
		}

		calls++

		if reason := r.CanResolveCall(&c, pr, dir); reason != "" {
			t.Errorf("%s.%s on line %d: %s", c.Variable, c.FuncName, c.Line+1, reason)
		}
	}

	if calls != 3 {
		t.Errorf("checked %d qualified calls, want 3", calls)
	}
}
