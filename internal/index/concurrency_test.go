package index

import (
	"fmt"
	"sync"
	"testing"

	"go.lsp.dev/uri"
)

// Every accessor here returns the map's own slice and then drops the read lock,
// so the caller walks a backing array the index still owns. Compacting that
// array in place on removal — `entries[:0]` plus appends — writes over entries
// the caller can still see. Run under -race this reported a write in IndexFile
// against a read in the walk below; without the detector it shows up as one
// file's definitions appearing under another file's name.
func TestConcurrentLookupDuringReindex(t *testing.T) {
	idx := New()

	const (
		files  = 8
		rounds = 200
	)

	srcs := make([]uri.URI, 0, files)

	for i := range files {
		u := uri.URI(fmt.Sprintf("file:///concurrent%d.cfc", i))
		srcs = append(srcs, u)
		idx.IndexFile(u, "component { function shared() {} }")
	}

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		for range rounds {
			for _, u := range srcs {
				idx.IndexFile(u, "component { function shared() {} }")
			}
		}
	}()

	var (
		mismatchMu sync.Mutex
		mismatches []string
	)

	go func() {
		defer wg.Done()

		for range rounds {
			for _, d := range idx.Lookup("shared") {
				if d.Name != "shared" {
					mismatchMu.Lock()

					mismatches = append(mismatches, "Lookup returned "+d.Name)
					mismatchMu.Unlock()
				}
			}

			for _, u := range srcs {
				for _, d := range idx.FunctionsForFile(u) {
					if d.URI != u {
						mismatchMu.Lock()

						mismatches = append(mismatches, string(u)+" yielded a def from "+string(d.URI))
						mismatchMu.Unlock()
					}
				}
			}
		}
	}()

	wg.Wait()

	mismatchMu.Lock()
	defer mismatchMu.Unlock()

	if len(mismatches) > 0 {
		t.Errorf("index handed out entries belonging to another file (%d times), first: %s",
			len(mismatches), mismatches[0])
	}
}
