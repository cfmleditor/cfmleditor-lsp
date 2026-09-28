package resolve_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// folderWorkspace lays out two workspace folders side by side, as tassweb's
// config does: app/ holds the components, other/ the code calling them. Their
// parent is not a workspace folder, so nothing but the folder's name can make
// app.packages.core.Kernel resolve.
func folderWorkspace(t *testing.T) (root, app, other string) {
	t.Helper()

	root = t.TempDir()
	app = filepath.Join(root, "app")
	other = filepath.Join(root, "other")

	for _, f := range []string{
		filepath.Join(app, "packages", "core", "Kernel.cfc"),
		filepath.Join(app, "includes", "header.cfm"),
		filepath.Join(other, "Caller.cfc"),
	} {
		if err := os.MkdirAll(filepath.Dir(f), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(f, []byte("component {}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return root, app, other
}

func writeFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("component {}"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestAWorkspaceFolderImpliesAMappingOfItsName is tassweb's config without its
// mappings. All three of them named a workspace folder by its own name, and
// removing them left 250,000 calls unresolved.
func TestAWorkspaceFolderImpliesAMappingOfItsName(t *testing.T) {
	_, app, other := folderWorkspace(t)

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{app, other}}

	want := filepath.Join(app, "packages", "core", "Kernel.cfc")

	for _, path := range []string{"app.packages.core.Kernel", "APP.packages.core.kernel", "/app/packages/core/Kernel"} {
		if got := r.ComponentPath(path, other); got != want {
			t.Errorf("ComponentPath(%q) = %q, want %q", path, got, want)
		}
	}

	if got := r.IncludePath("/app/includes/header.cfm", filepath.Join(other, "Caller.cfc")); got != filepath.Join(app, "includes", "header.cfm") {
		t.Errorf("IncludePath through the folder's name = %q", got)
	}

	// A name that is no folder's, and the folder's name alone, name nothing.
	for _, path := range []string{"nothere.packages.core.Kernel", "app"} {
		if got := r.ComponentPath(path, other); got != "" {
			t.Errorf("ComponentPath(%q) = %q, want no answer", path, got)
		}
	}
}

// TestTheFolderNameComesLast pins the precedence that makes the fallback safe
// to add: everything that resolved before resolves the same way, so an explicit
// mapping and a path relative to the calling file both win over it.
func TestTheFolderNameComesLast(t *testing.T) {
	root, app, other := folderWorkspace(t)

	elsewhere := filepath.Join(root, "elsewhere")
	writeFile(t, filepath.Join(elsewhere, "packages", "core", "Kernel.cfc"))

	mapped := &resolve.Resolver{
		FS: vfs.OS{}, Index: index.New(),
		WorkspaceFolders: []string{app, other},
		Mappings:         map[string]string{"app": elsewhere},
	}

	if got, want := mapped.ComponentPath("app.packages.core.Kernel", other), filepath.Join(elsewhere, "packages", "core", "Kernel.cfc"); got != want {
		t.Errorf("an explicit mapping lost to the folder's name: got %q, want %q", got, want)
	}

	// other/app/packages/core/Kernel.cfc, beside the caller.
	local := filepath.Join(other, "app", "packages", "core", "Kernel.cfc")
	writeFile(t, local)

	r := &resolve.Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{app, other}}

	if got := r.ComponentPath("app.packages.core.Kernel", other); got != local {
		t.Errorf("a path relative to the caller lost to the folder's name: got %q, want %q", got, local)
	}
}
