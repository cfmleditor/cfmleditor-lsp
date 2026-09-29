package index

import (
	"testing"

	"go.lsp.dev/uri"
)

// TestDelegatesAreForgottenWithTheExtends: a file's WireBox delegations are
// recorded by IndexFile and dropped by any re-index, as its extends is, so an
// edit that removes a delegation is seen by the next lookup rather than kept
// from the version first indexed.
func TestDelegatesAreForgottenWithTheExtends(t *testing.T) {
	idx := New()
	u := uri.URI("file:///w/Computer.cfc")

	idx.IndexFile(u, `component { property name="memory" inject delegate; }`)

	if ds, ok := idx.DelegatesForFile(u); !ok || len(ds) != 1 || ds[0].Target != "memory" {
		t.Fatalf("after IndexFile: %+v, %v", ds, ok)
	}

	idx.IndexFileFromResult(u, nil, nil)

	if ds, ok := idx.DelegatesForFile(u); ok {
		t.Errorf("after a re-index: %+v still held", ds)
	}

	idx.SetDelegates(u, nil)
	idx.RemoveFile(u)

	if _, ok := idx.DelegatesForFile(u); ok {
		t.Error("after RemoveFile: still held")
	}
}
