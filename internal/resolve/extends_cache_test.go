package resolve

import (
	"os"
	"path/filepath"
	"testing"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
)

// writeCFC is a small helper so the chain below reads as a chain.
func writeCFC(t *testing.T, dir, name, body string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return p
}

// TestLookupFuncWithExtendsReadsEachComponentOnce is the fix for the most
// expensive thing textDocument/didOpen did.
//
// Walking the extends chain needs one field, Extends, and used to get it by
// reading the component off disk and running a full parse — throwing away
// everything but that field. LookupFuncWithExtends runs once per unresolved
// call site, so a component with hundreds of them re-read and re-parsed the
// same bases hundreds of times. It is why allocation tracked the number of
// call sites rather than file size: a 540KB service.cfc cost 35.8MB to open
// while a persist.cfc three times larger cost a third as much per byte.
//
// The count is what matters, not the clock — reads per lookup is the thing
// that was wrong, and it is exact.
func TestLookupFuncWithExtendsReadsEachComponentOnce(t *testing.T) {
	dir := t.TempDir()

	base := writeCFC(t, dir, "Base.cfc", "component {\n\tfunction baseOnly() {}\n}\n")
	child := writeCFC(t, dir, "Child.cfc", "component extends=\"Base\" {\n\tfunction childOnly() {}\n}\n")

	cfs := newCountingFS()
	r := &Resolver{FS: cfs, Index: index.New()}

	const lookups = 6

	for range lookups {
		if def := r.LookupFuncWithExtends(child, "baseOnly"); def == nil {
			t.Fatal("baseOnly not found through the extends chain")
		}
	}

	// One read each, for every lookup after the first: EnsureIndexed parses
	// the file on the way in and the extends goes into the index with it.
	if got := cfs.count(child); got != 1 {
		t.Errorf("Child.cfc read %d times over %d lookups, want 1", got, lookups)
	}

	if got := cfs.count(base); got != 1 {
		t.Errorf("Base.cfc read %d times over %d lookups, want 1", got, lookups)
	}
}

// TestLookupFuncWithExtendsStillWalksTheChain is the control. Reading less is
// only an improvement if the answer is the same, and the failure mode of
// getting this wrong is silent: a chain that stops early resolves nothing and
// looks like a missing method.
func TestLookupFuncWithExtendsStillWalksTheChain(t *testing.T) {
	dir := t.TempDir()

	writeCFC(t, dir, "A.cfc", "component {\n\tfunction deepMethod() {}\n}\n")
	writeCFC(t, dir, "B.cfc", "component extends=\"A\" {\n}\n")
	c := writeCFC(t, dir, "C.cfc", "component extends=\"B\" {\n\tfunction own() {}\n}\n")

	r := &Resolver{FS: newCountingFS(), Index: index.New()}

	for _, tc := range []struct {
		name string
		want bool
	}{
		{"own", true},        // on C itself
		{"deepMethod", true}, // two hops up
		{"nowhere", false},
	} {
		def := r.LookupFuncWithExtends(c, tc.name)
		if got := def != nil; got != tc.want {
			t.Errorf("LookupFuncWithExtends(C, %q) found=%v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestExtendsRecordIsDroppedWhenTheFileIsReindexed is what lets the record live
// in the index with no invalidation logic of its own: removeFileEntries drops
// it with everything else, so a file whose extends changed is re-read once and
// the new answer used. A cache that missed this would keep resolving against
// the old parent until the daemon restarted.
func TestExtendsRecordIsDroppedWhenTheFileIsReindexed(t *testing.T) {
	dir := t.TempDir()

	writeCFC(t, dir, "Old.cfc", "component {\n\tfunction fromOld() {}\n}\n")
	writeCFC(t, dir, "New.cfc", "component {\n\tfunction fromNew() {}\n}\n")
	child := writeCFC(t, dir, "Child.cfc", "component extends=\"Old\" {\n}\n")

	idx := index.New()
	r := &Resolver{FS: newCountingFS(), Index: idx}

	if def := r.LookupFuncWithExtends(child, "fromOld"); def == nil {
		t.Fatal("fromOld not found before the edit")
	}

	if _, ok := idx.ExtendsForFile(cfpath.ToURI(child)); !ok {
		t.Fatal("extends was not recorded in the index")
	}

	// The edit, as the server would apply it: rewrite the file and re-index.
	writeCFC(t, dir, "Child.cfc", "component extends=\"New\" {\n}\n")
	idx.IndexFile(cfpath.ToURI(child), "component extends=\"New\" {\n}\n")

	if def := r.LookupFuncWithExtends(child, "fromNew"); def == nil {
		t.Error("fromNew not found after re-index: the stale extends is still being used")
	}

	if def := r.LookupFuncWithExtends(child, "fromOld"); def != nil {
		t.Error("fromOld still resolves after the parent changed")
	}
}
