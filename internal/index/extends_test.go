package index

import (
	"testing"

	"go.lsp.dev/uri"
)

// TestExtendsForFileDistinguishesUnknownFromNone pins the tri-state, which is
// the whole reason ExtendsForFile returns a second value.
//
// Only one of the two doors into the index can fill this in for free: IndexFile
// parses, so it knows; IndexFileFromResult is handed funcs and refs by a caller
// that already parsed. Collapsing "nobody has looked" into "extends nothing"
// would stop the resolver walking the chain for every file indexed through the
// second door — which loses resolutions silently rather than slowly.
func TestExtendsForFileDistinguishesUnknownFromNone(t *testing.T) {
	t.Parallel()

	idx := New()
	u := uri.File("/ws/Never.cfc")

	if _, ok := idx.ExtendsForFile(u); ok {
		t.Error("a file nobody has indexed reports a known extends")
	}

	// A component that extends nothing: known, and empty.
	base := uri.File("/ws/Base.cfc")
	idx.IndexFile(base, "component {\n\tfunction go() {}\n}\n")

	ext, ok := idx.ExtendsForFile(base)
	if !ok {
		t.Error("IndexFile did not record extends")
	}

	if ext != "" {
		t.Errorf("extends = %q, want empty", ext)
	}

	// A component that extends something.
	child := uri.File("/ws/Child.cfc")
	idx.IndexFile(child, "component extends=\"Base\" {\n}\n")

	if ext, ok := idx.ExtendsForFile(child); !ok || ext != "Base" {
		t.Errorf("ExtendsForFile = (%q, %v), want (\"Base\", true)", ext, ok)
	}
}

// TestExtendsIsForgottenWithTheRestOfTheFile is what removes the need for any
// invalidation logic of its own. The per-file views all have to be cleared
// together; an extends record left behind by removeFileEntries would outlive
// the parse it came from and answer for a file that no longer says that.
func TestExtendsIsForgottenWithTheRestOfTheFile(t *testing.T) {
	t.Parallel()

	idx := New()
	u := uri.File("/ws/Child.cfc")

	idx.IndexFile(u, "component extends=\"Base\" {\n}\n")

	if _, ok := idx.ExtendsForFile(u); !ok {
		t.Fatal("extends not recorded")
	}

	// The other door: re-indexed from a parse the caller already had, which
	// cannot supply extends. The record must be gone, not stale.
	idx.IndexFileFromResult(u, nil, nil)

	if ext, ok := idx.ExtendsForFile(u); ok {
		t.Errorf("extends survived a re-index as (%q, true); it would answer for a parse that no longer exists", ext)
	}
}
