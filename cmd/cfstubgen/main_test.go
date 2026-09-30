package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
)

func TestArtifactPathReturnInference(t *testing.T) {
	root := t.TempDir()

	files := map[string]string{
		"ConfigService.cfc": `component { function getSetting() { return ""; } }`,
		"ArtifactService.cfc": `component {
	property name="configService" inject="ConfigService";
	function getPackagePath(required packageName) {
		var path = getArtifactsDirectory() & arguments.packageName;
		return path;
	}
	string function getArtifactsDirectory() {
		var path = configService.getSetting("artifactsDirectory");
		return path;
	}
}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	g := &generator{src: &frameworkapi.Source{Prefix: "commandbox"}, root: root}

	src, err := g.load(filepath.Join(root, "ArtifactService.cfc"))
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for i := range src.pr.Funcs {
		if f := &src.pr.Funcs[i]; f.Name == "getPackagePath" {
			found = true

			if got := g.returnType(src, f); got != "" {
				t.Errorf("getPackagePath return type = %q, want no component type", got)
			}
		}
	}

	if !found {
		t.Fatal("getPackagePath not parsed")
	}

	if got := g.funcLookup(root)("ArtifactService", "getArtifactsDirectory"); got != "" {
		t.Errorf("string method lookup = %q, want no component type", got)
	}
}
