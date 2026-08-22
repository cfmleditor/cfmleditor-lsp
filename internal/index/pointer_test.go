package index

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Accessors hand out entry pointers and then release the read lock, so a caller
// reading def.Line is doing it unsynchronised. ShiftLines used to write that
// field in place, which raced. In daemon mode one Index is shared by every
// connection, so a keystroke in one editor and a hover in another are
// genuinely concurrent.
func TestShiftLinesDoesNotRaceReaders(t *testing.T) {
	idx := New()

	const (
		files  = 6
		rounds = 500
	)

	srcs := make([]uri.URI, 0, files)

	for i := range files {
		u := uri.URI(fmt.Sprintf("file:///shift%d.cfc", i))
		srcs = append(srcs, u)
		idx.IndexFileFromResult(u,
			[]parser.FunctionDef{{Name: "shared", URI: u, Line: 10}},
			[]parser.ComponentRef{{Variable: "svc", Component: "models.User", URI: u, Line: 12}},
		)
	}

	var (
		wg   sync.WaitGroup
		sink uint32
	)

	wg.Add(2)

	go func() {
		defer wg.Done()

		for range rounds {
			for _, u := range srcs {
				idx.ShiftLines(u, 0, 1)
			}
		}
	}()

	go func() {
		defer wg.Done()

		for range rounds {
			for _, d := range idx.Lookup("shared") {
				sink += d.Line
			}

			for _, r := range idx.LookupComponentRef("svc") {
				sink += r.Line
			}
		}
	}()

	wg.Wait()

	if sink == 0 {
		t.Error("reader never observed an entry; the test is not exercising anything")
	}
}

// The index must not share entry structs with the caller that supplied them:
// for the LSP server that slice is a live ParseResult, whose ApplyEdit shifts
// line numbers on every in-function keystroke without the index lock.
func TestIndexDoesNotAliasCallerEntries(t *testing.T) {
	idx := New()
	u := uri.URI("file:///alias.cfc")

	funcs := []parser.FunctionDef{{Name: "f", URI: u, Line: 3}}
	refs := []parser.ComponentRef{{Variable: "svc", Component: "models.User", URI: u, Line: 4}}

	idx.IndexFileFromResult(u, funcs, refs)

	// The caller mutates its own slice, exactly as ApplyEdit does.
	funcs[0].Line = 99
	refs[0].Line = 99

	got := idx.Lookup("f")
	if len(got) != 1 {
		t.Fatalf("expected 1 indexed def, got %d", len(got))
	}

	if got[0].Line != 3 {
		t.Errorf("index aliased the caller's slice: line moved to %d", got[0].Line)
	}

	gotRefs := idx.LookupComponentRef("svc")
	if len(gotRefs) != 1 {
		t.Fatalf("expected 1 indexed ref, got %d", len(gotRefs))
	}

	if gotRefs[0].Line != 4 {
		t.Errorf("index aliased the caller's ref slice: line moved to %d", gotRefs[0].Line)
	}
}

// ShiftLines has to update every map that holds an entry, or a file's entries
// end up disagreeing about where its functions are.
func TestShiftLinesKeepsMapsConsistent(t *testing.T) {
	idx := New()
	u := uri.URI("file:///consistent.cfc")

	idx.IndexFileFromResult(u,
		[]parser.FunctionDef{{Name: "f", URI: u, Line: 10}},
		[]parser.ComponentRef{{Variable: "svc", Component: "models.User", URI: u, Line: 12}},
	)

	idx.ShiftLines(u, 0, 5)

	byName := idx.Lookup("f")
	byFile := idx.FunctionsForFile(u)

	if len(byName) != 1 || len(byFile) != 1 {
		t.Fatalf("expected one entry in each map, got %d and %d", len(byName), len(byFile))
	}

	if byName[0].Line != 15 || byFile[0].Line != 15 {
		t.Errorf("funcs and fileFuncs disagree after a shift: %d vs %d", byName[0].Line, byFile[0].Line)
	}

	refByName := idx.LookupComponentRef("svc")
	refByFile := idx.RefsForFile(u)

	if len(refByName) != 1 || len(refByFile) != 1 {
		t.Fatalf("expected one ref in each map, got %d and %d", len(refByName), len(refByFile))
	}

	if refByName[0].Line != 17 || refByFile[0].Line != 17 {
		t.Errorf("comprefs and fileRefs disagree after a shift: %d vs %d", refByName[0].Line, refByFile[0].Line)
	}
}

// The aliasing had a visible consequence beyond the race. An edit inside a
// function body shifted the index twice: once because ParseResult.ApplyEdit
// moves pr.Funcs[i].Line and the index pointed at those very structs, and again
// because didChange's EditInFunc branch calls ShiftLines. Every function below
// the edit drifted by double the delta, and the drift compounded with each
// keystroke, so go-to-definition walked steadily further from the target.
func TestInFunctionEditShiftsIndexExactlyOnce(t *testing.T) {
	u := uri.URI("file:///drift.cfc")
	src := "component {\n\tfunction a() {\n\t\tvar x = 1;\n\t}\n\tfunction b() {\n\t\treturn 2;\n\t}\n}\n"

	pr := parser.Parse(u, src)

	idx := New()
	idx.IndexFileFromResult(u, pr.Funcs, pr.ComponentRefs)

	before := idx.Lookup("b")
	if len(before) != 1 {
		t.Fatalf("expected one definition of b, got %d", len(before))
	}

	start := before[0].Line

	// One line inserted inside a's body, then the shift didChange performs.
	if kind := pr.ApplyEdit(2, 12, 2, 12, "\n\t\tvar y = 2;"); kind != parser.EditInFunc {
		t.Fatalf("expected an in-function edit, got %v", kind)
	}

	idx.ShiftLines(u, 2, 1)

	after := idx.Lookup("b")
	if got := int(after[0].Line) - int(start); got != 1 {
		t.Errorf("b moved %d lines after a one-line insertion, want 1", got)
	}
}
