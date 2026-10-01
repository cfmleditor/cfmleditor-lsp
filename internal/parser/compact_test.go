package parser

import (
	"strings"
	"testing"
	"unsafe"
)

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

func TestCompactCollectionSourcesOwnStorage(t *testing.T) {
	source := strings.Repeat("padding", 1000) + "models.DAOread"
	component := source[len(source)-14 : len(source)-4]
	method := source[len(source)-4:]
	defs := []FunctionDef{{ReturnSources: []ReturnSource{{Component: component, Methods: []string{method}}}}}
	compact := CompactDefs(defs)

	got := &compact[0].ReturnSources[0]
	if got.Component != component || got.Methods[0] != method {
		t.Fatal("lost source contract")
	}

	if unsafe.StringData(got.Component) == unsafe.StringData(component) || unsafe.StringData(got.Methods[0]) == unsafe.StringData(method) {
		t.Fatal("retained parse buffer")
	}

	defs[0].ReturnSources[0].Methods[0] = "changed"

	defs[0].ReturnSources[0].Component = "changed"
	if got.Component != component || got.Methods[0] != method {
		t.Fatal("source arrays shared with parser")
	}
}
