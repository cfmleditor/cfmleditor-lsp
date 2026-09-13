package index

import (
	"fmt"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Re-indexing one file must cost what that file holds, not what the workspace
// holds. Every index write goes through removeFileEntries, so a cost
// proportional to the index makes the startup scan quadratic — a 5,624-file
// corpus took 34s and allocated 6.6GB to index, against 0.22s and 95MB once
// removal reached the name buckets through fileFuncs/fileRefs instead of
// walking every bucket and asking each entry which file it came from.
//
// Allocations rather than wall clock, for the reason
// TestRangeFormattingCoalesceIsLinear gives: the number is the shape of the
// algorithm, and it does not move when the machine is busy. Growth is measured
// against a smaller index rather than against an absolute budget, since what is
// being pinned is that the cost does not follow the index size.
func TestIndexFileFromResultDoesNotScaleWithIndexSize(t *testing.T) {
	const perFile = 8

	measure := func(files int) float64 {
		idx, u := benchIndex(files, perFile, false)
		funcs, refs := benchFileEntries(u, 0, perFile, false)

		// Index once outside the measurement so the run being measured is a
		// replacement, which is what the startup scan and every save perform.
		idx.IndexFileFromResult(u, funcs, refs)

		res := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				idx.IndexFileFromResult(u, funcs, refs)
			}
		})

		return float64(res.AllocsPerOp())
	}

	small := measure(200)
	large := measure(4000)

	t.Logf("allocs/op: 200 files %.0f, 4000 files %.0f", small, large)

	// A 20x larger index. Anything proportional to it lands near 20x; the
	// bound leaves room for the map bookkeeping that genuinely does grow a
	// little without leaving room for a per-entry sweep.
	if large > small*2 {
		t.Errorf("re-indexing one file scales with the index: %.0f allocs at 200 files, %.0f at 4000 (20x the files)", small, large)
	}
}

// A shared name bucket is the case pointer-identity removal has to get right:
// funcs is keyed by lowercased name, so every component's init() lives in one
// bucket, and removing one file's entry must leave the other files' entries in
// that same bucket alone.
func TestReindexLeavesOtherFilesInASharedBucket(t *testing.T) {
	idx := New()

	uris := make([]uri.URI, 3)
	for i := range uris {
		uris[i] = uri.File(fmt.Sprintf("/ws/File%d.cfc", i))
		idx.IndexFileFromResult(uris[i],
			[]parser.FunctionDef{{Name: "init", URI: uris[i], Line: uint32(i)}},
			[]parser.ComponentRef{{Variable: "svc", Component: "models.User", URI: uris[i], Line: uint32(i)}},
		)
	}

	// Re-index the middle file with a different function name, which empties
	// its slot in the shared bucket.
	idx.IndexFileFromResult(uris[1],
		[]parser.FunctionDef{{Name: "configure", URI: uris[1], Line: 99}},
		nil,
	)

	got := idx.Lookup("init")
	if len(got) != 2 {
		t.Fatalf("init bucket = %d entries, want 2 (files 0 and 2)", len(got))
	}

	for _, d := range got {
		if d.URI == uris[1] {
			t.Errorf("re-indexed file still in the init bucket at line %d", d.Line)
		}
	}

	if refs := idx.LookupComponentRef("svc"); len(refs) != 2 {
		t.Errorf("svc bucket = %d refs, want 2", len(refs))
	}

	if fns := idx.FunctionsForFile(uris[1]); len(fns) != 1 || fns[0].Name != "configure" {
		t.Errorf("FunctionsForFile after re-index = %+v, want one configure", fns)
	}
}

// RemoveFilesUnder has to clear the per-file views as well as the name buckets.
// Leaving them behind made FunctionsForFile answer with definitions no bucket
// held any more, and left HasFile reporting a removed file as indexed — which
// is what Resolver.EnsureIndexed consults before re-reading from disk.
func TestRemoveFilesUnderClearsPerFileViews(t *testing.T) {
	idx := New()

	gone := uri.File("/ws/removed/Svc.cfc")
	kept := uri.File("/ws/stays/Svc.cfc")

	for _, u := range []uri.URI{gone, kept} {
		idx.IndexFileFromResult(u,
			[]parser.FunctionDef{{Name: "init", URI: u, Line: 1}},
			[]parser.ComponentRef{{Variable: "svc", Component: "models.User", URI: u, Line: 2}},
		)
		idx.SetThisVars(u, []string{"name"})
	}

	idx.RemoveFilesUnder(string(uri.File("/ws/removed")))

	if idx.HasFile(gone) {
		t.Error("HasFile still reports the removed file as indexed")
	}

	if fns := idx.FunctionsForFile(gone); len(fns) != 0 {
		t.Errorf("FunctionsForFile(removed) = %d, want 0", len(fns))
	}

	if refs := idx.RefsForFile(gone); len(refs) != 0 {
		t.Errorf("RefsForFile(removed) = %d, want 0", len(refs))
	}

	if vars := idx.ThisVarsForFile(gone); len(vars) != 0 {
		t.Errorf("ThisVarsForFile(removed) = %v, want none", vars)
	}

	if !idx.HasFile(kept) || len(idx.FunctionsForFile(kept)) != 1 {
		t.Error("the folder that was not removed lost its entries")
	}

	if len(idx.Lookup("init")) != 1 {
		t.Errorf("init bucket = %d, want 1", len(idx.Lookup("init")))
	}
}

// The same rule as TestIndexFileFromResultDoesNotScaleWithIndexSize, for the
// lookup on the other side. Hover, definition and completion each ask this on
// the keystroke (hover twice), and it used to search the variable name's
// bucket — which for the names that get asked about holds one entry per file in
// the workspace, each costing a uriKey to reject. On 5,000 files that was
// 0.98ms and 160KB per lookup.
func TestLookupComponentRefInFileDoesNotScaleWithIndexSize(t *testing.T) {
	measure := func(files int) float64 {
		idx, u := benchIndex(files, 8, true)

		if idx.LookupComponentRefInFile("svc3", u, 100) == nil {
			t.Fatalf("no ref found with %d files indexed", files)
		}

		res := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				idx.LookupComponentRefInFile("svc3", u, 100)
			}
		})

		return float64(res.AllocsPerOp())
	}

	small := measure(200)
	large := measure(4000)

	t.Logf("allocs/op: 200 files %.0f, 4000 files %.0f", small, large)

	if large > small*2 {
		t.Errorf("lookup scales with the index: %.0f allocs at 200 files, %.0f at 4000 (20x the files)", small, large)
	}
}
