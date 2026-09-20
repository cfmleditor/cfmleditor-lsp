package index

import (
	"fmt"
	"testing"

	"go.lsp.dev/uri"
)

// Both of these used to sweep a name bucket: an outer loop over every entry
// filed under a name, asking of each whether it was one of the handful this
// file owns. For the names real workspaces ask about — init, get, svc, dao —
// that bucket holds one entry per file, so the sweep was the whole workspace
// and the answer was one entry. Each comparison went through a closure, and for
// removal through a slices.Contains over a one-element group, so the per-entry
// cost was a call rather than a compare.
//
// They now search the bucket for the entries they are already holding pointers
// to — the same comparisons with the loops the other way round, and one call
// instead of one per entry.
//
// What that does not do is make the cost flat: finding a pointer in an
// unordered slice is still a scan, and both remain proportional to how many
// files declare the name. Making them flat needs a pointer-to-position map
// beside the buckets, which is a few megabytes held for the life of the index —
// not obviously the better trade against a cost that is now a fraction of the
// parse it accompanies. So what these tests pin is the constant, measured
// against the same operation on a bucket that holds one entry.
//
// The comparison is between two measurements taken in the same process, for the
// reason the other scaling tests here give: a busy runner moves both numbers
// together, so they fail on the regression rather than on the machine.
// Allocations cannot see either defect — neither sweep allocated — so these
// measure time, and the bound is loose enough to absorb that.
const (
	bucketScaleFiles = 4000

	// Measured: 5x for a re-index and 9.5x for a shift, against 25x and 36x
	// with the sweeps restored.
	bucketScaleMaxRatio = 16.0
)

// bucketScaleIndex builds an index and returns a file from the middle of it.
// Not the first file: its entries are at the front of every bucket, where a
// scan finds them at once, which would report any search as free.
func bucketScaleIndex(t *testing.T, sharedNames bool) (*Index, uri.URI) {
	t.Helper()

	idx, _ := benchIndex(bucketScaleFiles, 8, sharedNames)
	f := bucketScaleFiles / 2
	u := uri.File(fmt.Sprintf("/ws/pkg%d/File%d.cfc", f%20, f))

	if len(idx.FunctionsForFile(u)) == 0 {
		t.Fatalf("no entries indexed for %s — the URI does not match what benchIndex built", u)
	}

	return idx, u
}

func TestReindexDoesNotSweepASharedNameBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 4,000-file index and measures over many writes")
	}

	measure := func(sharedNames bool) float64 {
		idx, u := bucketScaleIndex(t, sharedNames)
		funcs, refs := benchFileEntries(u, bucketScaleFiles/2, 8, sharedNames)

		// Index once outside the measurement so what is measured is a
		// replacement — what every save and the startup scan perform.
		idx.IndexFileFromResult(u, funcs, refs)

		res := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				idx.IndexFileFromResult(u, funcs, refs)
			}
		})

		return float64(res.NsPerOp())
	}

	assertBucketConstant(t, "re-indexing one file", measure)
}

// ShiftLines runs under the write lock on every keystroke that adds or removes
// a line, so in daemon mode what it costs is paid by every other session's
// lookups too.
func TestShiftLinesDoesNotSweepASharedNameBucket(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 4,000-file index and measures over many writes")
	}

	measure := func(sharedNames bool) float64 {
		idx, u := bucketScaleIndex(t, sharedNames)

		res := testing.Benchmark(func(b *testing.B) {
			delta := 1
			for b.Loop() {
				// Alternating, so the lines stay put across the run and every
				// iteration has the same entries to move.
				idx.ShiftLines(u, 0, delta)
				delta = -delta
			}
		})

		return float64(res.NsPerOp())
	}

	assertBucketConstant(t, "shifting one file's lines", measure)
}

func assertBucketConstant(t *testing.T, what string, measure func(sharedNames bool) float64) {
	t.Helper()

	distinct := measure(false)
	shared := measure(true)

	t.Logf("%s over %d files: distinct names %.0fns, shared names %.0fns (ratio %.2f)",
		what, bucketScaleFiles, distinct, shared, shared/distinct)

	if distinct == 0 {
		t.Fatal("measured nothing — the benchmark did not run")
	}

	if ratio := shared / distinct; ratio > bucketScaleMaxRatio {
		t.Errorf("%s costs %.0fns where every file declares the same names against %.0fns where none do "+
			"(ratio %.2f, want <= %.2f) — the name bucket is being swept per entry",
			what, shared, distinct, ratio, bucketScaleMaxRatio)
	}
}
