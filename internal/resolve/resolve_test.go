package resolve_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/uri"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

// TestCanResolveCall_ResolverFallbackWhenRefDerivesWrongComponent covers the case where
// a variable (objFile) receives a component ref via a broad resolver matching its RHS
// expression (e.g. the "_parent" resolver matches "_parent.getFile()"), but that
// component does not have the called method. The variable-name resolver ("objFile" →
// FileObject) should be tried as a fallback and resolve the call correctly.
func TestCanResolveCall_ResolverFallbackWhenRefDerivesWrongComponent(t *testing.T) {
	dir := testdataDir()

	resolvers := []parser.Resolver{
		// Broad resolver: any expression containing "_parent" → models.Base.
		// This also matches "var objFile = _parent.getFile()" RHS, giving objFile
		// a component ref of models.Base even though that is the wrong type.
		{Match: `_parent`, Resolve: "models.Base", Prefix: "_parent"},
		// Specific variable-name resolver: objFile → models.FileObject.
		{Match: `objFile`, Resolve: "models.FileObject", Prefix: "objFile"},
	}

	// Tag-based CFC: var objFile = _parent.getFile() followed by objFile.open().
	content := `<cfcomponent>
<cffunction name="testMethod">
	<cfset var objFile = _parent.getFile()>
	<cfset objFile.open()>
</cffunction>
</cfcomponent>`

	fileURI := uri.URI("file://" + filepath.Join(dir, "test_resolve_fallback.cfc"))
	pr := parser.ParseWithOptions(fileURI, content, parser.ParseOptions{
		Resolvers:    resolvers,
		ExtractCalls: true,
	})

	// Confirm the parser assigned the wrong component to objFile via the _parent resolver.
	var objFileComp string

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "objFile" {
			objFileComp = ref.Component

			break
		}
	}

	if objFileComp == "" {
		// Also check function-scoped refs.
		for _, scope := range pr.Scopes {
			for _, ref := range pr.FuncComponentRefs(scope.Start, scope.End) {
				if ref.Variable == "objFile" {
					objFileComp = ref.Component

					break
				}
			}
		}
	}

	if objFileComp != "models.Base" {
		t.Fatalf("expected objFile to have ref models.Base (from broad _parent resolver), got %q", objFileComp)
	}

	// Set up a Resolver with the real testdata files indexed.
	idx := index.New()
	fsys := vfs.OS{}

	for _, rel := range []string{"models/Base.cfc", "models/FileObject.cfc"} {
		abs := filepath.Join(dir, rel)

		data, err := fsys.ReadFile(abs)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}

		idx.IndexFile(uri.URI("file://"+abs), string(data))
	}

	r := &resolve.Resolver{
		FS:               fsys,
		Index:            idx,
		Resolvers:        resolvers,
		WorkspaceFolders: []string{dir},
	}

	baseDir := dir

	// Find the objFile.open() call site.
	calls := pr.FuncCalls(0, 10)

	var openCall *parser.CallSite

	for i := range calls {
		if calls[i].FuncName == "open" && calls[i].Variable == "objFile" {
			openCall = &calls[i]

			break
		}
	}

	if openCall == nil {
		t.Fatal("expected to find objFile.open() call site")
	}

	// models.Base does NOT have open(); the fallback to the objFile resolver
	// (→ models.FileObject) should make this resolvable.
	if reason := r.CanResolveCall(*openCall, pr, baseDir); reason != "" {
		t.Errorf("expected objFile.open() to resolve, got: %s", reason)
	}
}
