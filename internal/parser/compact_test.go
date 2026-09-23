package parser

import "testing"

// A function with no arguments must not carry the parser's argument array into
// the index. parseArgList allocates room for four up front, so the parse hands
// over an empty slice with capacity behind it, and copying the FunctionDef kept
// that array alive for every argument-less function in the workspace.
func TestCompactDefsDropsAnEmptyArgumentArray(t *testing.T) {
	pr := Parse("file:///noargs.cfc", "component {\n\tfunction a() {}\n\tfunction b(x) {}\n}\n")
	if len(pr.Funcs) != 2 {
		t.Fatalf("parsed %d functions, want 2", len(pr.Funcs))
	}

	for _, d := range CompactDefs(pr.Funcs) {
		if len(d.Arguments) == 0 && cap(d.Arguments) != 0 {
			t.Errorf("%s: compacted with an empty argument slice of capacity %d", d.Name, cap(d.Arguments))
		}

		if d.Name == "b" && len(d.Arguments) != 1 {
			t.Errorf("b: %d arguments, want 1", len(d.Arguments))
		}
	}
}
