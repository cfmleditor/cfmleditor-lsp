package resolve

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func TestReviewAppLessCallersShareView(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"a/Helper.cfc": `component {function a(){}}`, "b/Helper.cfc": `component {function b(){}}`})
	r := &Resolver{FS: vfs.OS{}, Index: index.New(), WorkspaceFolders: []string{dir}}

	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if r.forCaller(a) != r.forCaller(b) {
		t.Fatal("app-less callers allocated distinct mapping views")
	}

	if r.forCaller(a).ResolveFunc("Helper", "a", a) == nil || r.forCaller(b).ResolveFunc("Helper", "b", b) == nil {
		t.Fatal("sharing a view lost physical relative lookup")
	}
}
