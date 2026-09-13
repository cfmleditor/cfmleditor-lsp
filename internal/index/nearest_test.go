package index

import (
	"fmt"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// When several components declare the same method and none of them is the file
// the request came from, LookupPreferred has to choose. It used to return
// whichever the bucket held first — the order eight parallel indexing
// goroutines finished in, so the answer differed between restarts.
func TestLookupPreferredPicksTheNearestDefinition(t *testing.T) {
	// Ordered so the nearest is deliberately *not* the first indexed: taking
	// defs[0] would return the far one.
	far := uri.File("/ws/other/branch/deep/Far.cfc")
	mid := uri.File("/ws/app/Mid.cfc")
	near := uri.File("/ws/app/models/Near.cfc")

	asking := uri.File("/ws/app/models/Asking.cfc")

	idx := New()
	for _, u := range []uri.URI{far, mid, near} {
		idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "save", URI: u, Line: 1}}, nil)
	}

	got, inFile, total := idx.LookupPreferred("save", asking)

	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}

	if inFile {
		t.Error("inFile = true, but the asking file declares nothing")
	}

	if got.URI != near {
		t.Errorf("picked %s, want the nearest (%s)", got.URI, near)
	}
}

// The file's own definition still wins outright, however far the others are.
func TestLookupPreferredStillPrefersTheOwnFile(t *testing.T) {
	own := uri.File("/ws/deeply/nested/somewhere/Own.cfc")
	other := uri.File("/ws/Other.cfc")

	idx := New()
	for _, u := range []uri.URI{other, own} {
		idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "save", URI: u, Line: 1}}, nil)
	}

	got, inFile, _ := idx.LookupPreferred("save", own)
	if !inFile || got.URI != own {
		t.Errorf("picked %s (inFile=%v), want the asking file's own", got.URI, inFile)
	}
}

// Equally distant candidates need a tie-break of their own, or the answer falls
// back to bucket order and the determinism is only partial.
func TestLookupPreferredIsStableAmongEqualDistances(t *testing.T) {
	asking := uri.File("/ws/app/Asking.cfc")

	a := uri.File("/ws/app/Alpha.cfc")
	z := uri.File("/ws/app/Zulu.cfc")

	// Index in each order; the answer must not depend on it.
	for _, order := range [][]uri.URI{{a, z}, {z, a}} {
		idx := New()
		for _, u := range order {
			idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "save", URI: u, Line: 1}}, nil)
		}

		got, _, _ := idx.LookupPreferred("save", asking)
		if got.URI != a {
			t.Errorf("indexed %v first: picked %s, want %s consistently", order[0], got.URI, a)
		}
	}
}

// The whole point is that the answer does not move when the index is built in a
// different order, which is what a parallel workspace scan produces.
func TestLookupPreferredDoesNotDependOnIndexingOrder(t *testing.T) {
	asking := uri.File("/ws/app/models/Asking.cfc")

	files := []uri.URI{
		uri.File("/ws/app/models/Near.cfc"),
		uri.File("/ws/app/Mid.cfc"),
		uri.File("/ws/other/Far.cfc"),
		uri.File("/ws/other/deep/Deeper.cfc"),
	}

	var want uri.URI

	// Every rotation of the indexing order must produce the same answer.
	for start := range files {
		idx := New()

		for i := range files {
			u := files[(start+i)%len(files)]
			idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "save", URI: u, Line: 1}}, nil)
		}

		got, _, _ := idx.LookupPreferred("save", asking)

		if start == 0 {
			want = got.URI

			continue
		}

		if got.URI != want {
			t.Errorf("rotation %d picked %s, rotation 0 picked %s", start, got.URI, want)
		}
	}

	if want != files[0] {
		t.Errorf("settled on %s, want the nearest (%s)", want, files[0])
	}
}

func TestNearestToIsLinear(t *testing.T) {
	asking := uri.File("/ws/app/Asking.cfc")

	idx := New()

	for f := range 200 {
		u := uri.File(fmt.Sprintf("/ws/pkg%d/File%d.cfc", f%20, f))
		idx.IndexFileFromResult(u, []parser.FunctionDef{{Name: "save", URI: u, Line: 1}}, nil)
	}

	res := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			idx.LookupPreferred("save", asking)
		}
	})

	// One allocation for uriKey; the scan itself must add none.
	if res.AllocsPerOp() > 2 {
		t.Errorf("LookupPreferred allocates %d per call over a 200-entry bucket, want at most 2", res.AllocsPerOp())
	}
}
