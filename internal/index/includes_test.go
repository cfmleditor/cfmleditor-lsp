package index

import (
	"slices"
	"testing"

	"go.lsp.dev/uri"
)

// TestIncludeGenerationMovesOnlyOnChange pins what the resolver's include graph
// relies on: the generation moves when a file's includes change and at no other
// time. Re-indexing a file runs removeFileEntries first, and the first version
// cleared includes there, so every re-index of an unchanged file moved it and
// the resolver rebuilt the whole graph — thousands of stats, on every lazy
// index during a scan and on every keystroke in the editor.
func TestIncludeGenerationMovesOnlyOnChange(t *testing.T) {
	idx := New()
	u := uri.URI("file:///api.cfc")
	body := `<cfcomponent><cfinclude template="a.cfm"></cfcomponent>`

	idx.IndexFile(u, body)
	gen := idx.IncludeGeneration()

	idx.IndexFile(u, body)
	idx.IndexFileFromResult(u, nil, nil)
	idx.SetIncludes(u, []string{"a.cfm"})

	if got := idx.IncludeGeneration(); got != gen {
		t.Errorf("re-indexing with the same includes moved the generation %d -> %d", gen, got)
	}

	idx.IndexFile(u, `<cfcomponent><cfinclude template="b.cfm"></cfcomponent>`)

	if idx.IncludeGeneration() == gen {
		t.Error("changing an include did not move the generation")
	}

	gen = idx.IncludeGeneration()
	idx.RemoveFile(u)

	if idx.IncludeGeneration() == gen {
		t.Error("removing a file with includes did not move the generation")
	}

	var seen []string

	idx.ForEachInclude(func(_ uri.URI, paths []string) { seen = append(seen, paths...) })

	if !slices.Equal(seen, nil) {
		t.Errorf("a removed file's includes are still listed: %q", seen)
	}
}
