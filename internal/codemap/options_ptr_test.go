package codemap_test

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// Build takes its options by pointer and fills in Workers and ConfigFor when
// they are unset. It must do that on a copy: a caller building two maps from
// one options value would otherwise get the second built with the first's
// defaults, and ConfigFor fixed to a single config it never asked for.
func TestBuildDoesNotWriteToTheCallersOptions(t *testing.T) {
	root := testdataDir(t)
	fsys := vfs.OS{}

	opts := codemap.Options{
		Root:     root,
		FS:       fsys,
		Resolver: &resolve.Resolver{FS: fsys, Index: index.New(), WorkspaceFolders: []string{root}},
	}

	codemap.Build(&opts)

	if opts.Workers != 0 {
		t.Errorf("Build set the caller's Workers to %d", opts.Workers)
	}

	if opts.ConfigFor != nil {
		t.Error("Build set the caller's ConfigFor")
	}
}
