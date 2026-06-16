package resolve_test

import (
	"os"
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

// TestCanResolveCall_ExtendsChainRefLookupWithoutPreIndex covers the case where a parent
// component (in the extends chain) assigns a component ref (e.g. variables.$assert = new Assertion())
// but the parent file was NOT pre-indexed.  The lookup must trigger on-demand indexing of the
// parent so that the ref is visible to CanResolveCall.
func TestCanResolveCall_ExtendsChainRefLookupWithoutPreIndex(t *testing.T) {
	dir := testdataDir()

	// Deliberately do NOT index the parent (models/Base.cfc) before the test.
	idx := index.New()
	fsys := vfs.OS{}

	// Only index FileObject.cfc so methods on it are known.
	objPath := filepath.Join(dir, "models", "FileObject.cfc")

	data, err := fsys.ReadFile(objPath)
	if err != nil {
		t.Fatalf("read FileObject.cfc: %v", err)
	}

	idx.IndexFile(uri.URI("file://"+objPath), string(data))

	r := &resolve.Resolver{
		FS:               fsys,
		Index:            idx,
		WorkspaceFolders: []string{dir},
	}

	// A child component that extends a parent whose CFC lives in the testdata directory.
	// The parent assigns `variables.objFile = new models.FileObject()` (component ref).
	// The child calls `objFile.open()`.
	//
	// We use models/Base.cfc as the "parent" stand-in but write a new in-memory parent
	// that sets the ref. We use testdata/models/FileObject.cfc as the component type.
	parentContent := `component {
	function beforeEach() {
		variables.objFile = new models.FileObject();
	}
}`
	parentPath := filepath.Join(dir, "FakeParent.cfc")
	parentURI := uri.URI("file://" + parentPath)

	// Write temp parent file
	if err := os.WriteFile(parentPath, []byte(parentContent), 0o644); err != nil {
		t.Fatalf("write parent: %v", err)
	}

	defer func() { _ = os.Remove(parentPath) }()

	// Child file: extends FakeParent, calls objFile.open() — ref must come from parent.
	childContent := `component extends="FakeParent" {
	function doWork() {
		objFile.open();
	}
}`
	childURI := uri.URI("file://" + filepath.Join(dir, "FakeChild.cfc"))
	pr := parser.ParseWithOptions(childURI, childContent, parser.ParseOptions{ExtractCalls: true})

	// Confirm objFile has no ref in the child itself.
	if len(pr.ComponentRefs) != 0 {
		t.Logf("ComponentRefs: %+v", pr.ComponentRefs)
	}

	_ = parentURI

	baseDir := dir

	calls := pr.FuncCalls(0, 20)

	var openCall *parser.CallSite

	for i := range calls {
		if calls[i].FuncName == "open" && calls[i].Variable == "objFile" {
			openCall = &calls[i]

			break
		}
	}

	if openCall == nil {
		t.Fatal("expected objFile.open() call site")
	}

	// CanResolveCall must trigger on-demand indexing of FakeParent.cfc and find the ref.
	if reason := r.CanResolveCall(*openCall, pr, baseDir); reason != "" {
		t.Errorf("expected objFile.open() to resolve via parent ref, got: %s", reason)
	}
}

// TestCanResolveCall_ExtendsChainTwoLevels_ThisVarRef covers the case where a component
// ref is assigned two extends-levels up using the chained syntax:
//
//	variables.$assert = this.$assert = new models.FileObject()
//
// The grandparent (level 2) holds the ref; the parent (level 1) just passes extends
// through; the child calls $assert.open().  Neither parent nor grandparent is pre-indexed.
func TestCanResolveCall_ExtendsChainTwoLevels_ThisVarRef(t *testing.T) {
	dir := testdataDir()

	// Grandparent: holds the component ref via chained this./variables. assignment.
	grandParentContent := `component {
	function beforeEach() {
		variables.$assert = this.$assert = new models.FileObject();
	}
}`

	// Middle parent: simply extends grandparent, no refs of its own.
	middleParentContent := `component extends="GrandParentSpec" {}`

	grandParentPath := filepath.Join(dir, "GrandParentSpec.cfc")
	middleParentPath := filepath.Join(dir, "MiddleParentSpec.cfc")

	for path, content := range map[string]string{
		grandParentPath: grandParentContent,
		middleParentPath: middleParentContent,
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		defer func(p string) { _ = os.Remove(p) }(path)
	}

	// Only pre-index FileObject so its methods are known; parents are intentionally absent.
	idx := index.New()
	fsys := vfs.OS{}

	objPath := filepath.Join(dir, "models", "FileObject.cfc")

	data, err := fsys.ReadFile(objPath)
	if err != nil {
		t.Fatalf("read FileObject.cfc: %v", err)
	}

	idx.IndexFile(uri.URI("file://"+objPath), string(data))

	r := &resolve.Resolver{
		FS:               fsys,
		Index:            idx,
		WorkspaceFolders: []string{dir},
	}

	// Child: extends MiddleParentSpec, calls $assert.open() with no local ref.
	childContent := `component extends="MiddleParentSpec" {
	function doWork() {
		$assert.open();
	}
}`
	childURI := uri.URI("file://" + filepath.Join(dir, "ChildSpec.cfc"))
	pr := parser.ParseWithOptions(childURI, childContent, parser.ParseOptions{ExtractCalls: true})

	calls := pr.FuncCalls(0, 10)

	var assertCall *parser.CallSite

	for i := range calls {
		if calls[i].FuncName == "open" && calls[i].Variable == "$assert" {
			assertCall = &calls[i]

			break
		}
	}

	if assertCall == nil {
		t.Fatal("expected $assert.open() call site in child")
	}

	// Resolution must walk up two extends levels to find $assert in GrandParentSpec.
	if reason := r.CanResolveCall(*assertCall, pr, dir); reason != "" {
		t.Errorf("expected $assert.open() to resolve via two-level extends chain, got: %s", reason)
	}
}

// TestComponentPath_BareNameIndexFallback covers the case where a component is referenced
// by a bare name (no dots, e.g. extends="BaseAssertionsTest") and the file is not in the
// same directory or workspace root, but IS indexed in the workspace.
func TestComponentPath_BareNameIndexFallback(t *testing.T) {
	dir := testdataDir()

	idx := index.New()
	fsys := vfs.OS{}

	// Index FileObject which lives under testdata/models/
	cfcPath := filepath.Join(dir, "models", "FileObject.cfc")

	data, err := fsys.ReadFile(cfcPath)
	if err != nil {
		t.Fatalf("read FileObject.cfc: %v", err)
	}

	idx.IndexFile(uri.URI("file://"+cfcPath), string(data))

	r := &resolve.Resolver{
		FS:               fsys,
		Index:            idx,
		WorkspaceFolders: []string{dir},
	}

	// Resolve from a directory that is NOT models/ — normal lookup will fail,
	// but the index fallback should find the file.
	got := r.ComponentPath("FileObject", dir)
	if got != cfcPath {
		t.Errorf("ComponentPath(\"FileObject\", dir) = %q, want %q", got, cfcPath)
	}
}
