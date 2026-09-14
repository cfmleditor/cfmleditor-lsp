package index

import (
	"fmt"
	"sync"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Every accessor hands its result out and then releases the read lock, so what
// it returns must be storage no writer will touch. The index relies on that in
// the other direction too: because nothing escapes, removal compacts a bucket
// in place and ShiftLines writes the replacement pointer into it, instead of
// rebuilding a slice that on a shared name like init holds one entry per file
// in the workspace.
//
// This is the contract that makes those writes legal. Break it and the failure
// is a data race under load, not a test failure — so it is pinned here
// directly, and exercised under -race by the concurrent test below.
func TestAccessorsReturnStorageWritersDoNotTouch(t *testing.T) {
	idx := New()

	a := uri.File("/ws/A.cfc")
	b := uri.File("/ws/B.cfc")

	for _, u := range []uri.URI{a, b} {
		idx.IndexFileFromResult(u,
			[]parser.FunctionDef{{Name: "init", URI: u, Line: 1}},
			[]parser.ComponentRef{{Variable: "svc", URI: u, Component: "models.User", Line: 2}},
		)
		idx.SetThisVars(u, []string{"name"})
	}

	gotFuncs := idx.Lookup("init")
	gotRefs := idx.LookupComponentRef("svc")
	gotFile := idx.FunctionsForFile(a)
	gotVars := idx.ThisVarsForFile(a)

	if len(gotFuncs) != 2 || len(gotRefs) != 2 || len(gotFile) != 1 || len(gotVars) != 1 {
		t.Fatalf("setup: funcs %d refs %d file %d vars %d", len(gotFuncs), len(gotRefs), len(gotFile), len(gotVars))
	}

	// Every kind of write that touches those buckets.
	idx.RemoveFile(b)
	idx.ShiftLines(a, 0, 10)
	idx.IndexFileFromResult(a, []parser.FunctionDef{{Name: "init", URI: a, Line: 99}}, nil)

	if len(gotFuncs) != 2 {
		t.Errorf("returned slice was resized by a later write: %d", len(gotFuncs))
	}

	for i, d := range gotFuncs {
		if d == nil {
			t.Fatalf("entry %d in a returned slice was cleared by a later write", i)
		}

		if d.Line != 1 {
			t.Errorf("entry %d moved under the caller: line %d, want the 1 it was handed", i, d.Line)
		}
	}

	if len(gotRefs) != 2 || gotRefs[0] == nil || gotRefs[1] == nil {
		t.Error("component refs handed out were disturbed by a later write")
	}

	if len(gotFile) != 1 || gotFile[0] == nil || gotFile[0].Line != 1 {
		t.Error("per-file definitions handed out were disturbed by a later write")
	}

	// The caller owns what it was given, so scribbling on it must not reach the
	// index. Re-set the vars first: re-indexing a above dropped them, which is
	// what indexing a file means.
	idx.SetThisVars(a, []string{"name"})

	gotVars = idx.ThisVarsForFile(a)
	gotVars[0] = "clobbered"

	if again := idx.ThisVarsForFile(a); len(again) != 1 || again[0] != "name" {
		t.Errorf("writing to a returned slice reached the index: %v", again)
	}

	// And in reverse: the index stores its own copy, not the caller's slice.
	stored := []string{"kept"}
	idx.SetThisVars(a, stored)
	stored[0] = "changed by the caller"

	if again := idx.ThisVarsForFile(a); len(again) != 1 || again[0] != "kept" {
		t.Errorf("index stored the caller's slice rather than its own: %v", again)
	}
}

// The same contract, in the shape that actually broke: readers walking what
// they were handed while writers rebuild the buckets underneath. Meaningful
// under -race, which is where CI runs it.
func TestConcurrentReadsAndWrites(t *testing.T) {
	idx := New()

	const files = 40

	uris := make([]uri.URI, files)
	for i := range uris {
		uris[i] = uri.File(fmt.Sprintf("/ws/File%d.cfc", i))
		idx.IndexFileFromResult(uris[i],
			[]parser.FunctionDef{{Name: "init", URI: uris[i], Line: uint32(i)}},
			[]parser.ComponentRef{{Variable: "svc", URI: uris[i], Component: "models.User", Line: uint32(i)}},
		)
	}

	var wg sync.WaitGroup

	for r := range 4 {
		wg.Go(func() {
			for range 200 {
				for _, d := range idx.Lookup("init") {
					_ = d.Line
				}

				for _, ref := range idx.LookupComponentRef("svc") {
					_ = ref.Line
				}

				_ = idx.FunctionsForFile(uris[r])
				_ = idx.RefsForFile(uris[r])
				idx.LookupComponentRefInFile("svc", uris[r], 1000)
				idx.LookupPreferred("init", uris[r])
			}
		})
	}

	for w := range 2 {
		wg.Go(func() {
			u := uris[files-1-w]

			for i := range 200 {
				idx.ShiftLines(u, 0, 1)
				idx.IndexFileFromResult(u,
					[]parser.FunctionDef{{Name: "init", URI: u, Line: uint32(i)}},
					[]parser.ComponentRef{{Variable: "svc", URI: u, Component: "models.User", Line: uint32(i)}},
				)
				idx.SetFuncRefs(u, "1:9", []parser.ComponentRef{
					{Variable: "svc", URI: u, Component: "models.Other", Line: 3},
				})
			}
		})
	}

	wg.Wait()

	// The race detector is the point of this test, but a run without it should
	// still say something: after all that concurrent rebuilding the buckets
	// must still hold exactly one live entry per file.
	defs := idx.Lookup("init")
	if len(defs) != files {
		t.Errorf("init bucket holds %d entries after concurrent writes, want %d", len(defs), files)
	}

	for i, d := range defs {
		if d == nil {
			t.Fatalf("entry %d is nil: a bucket was compacted without being resized", i)
		}
	}

	seen := make(map[uri.URI]bool, files)
	for _, d := range defs {
		if seen[d.URI] {
			t.Errorf("%s appears twice in the init bucket", d.URI)
		}

		seen[d.URI] = true
	}
}
