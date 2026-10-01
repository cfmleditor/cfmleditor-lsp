package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/frameworkapi"
)

func TestGeneratedStubFollowsArgumentComponent(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()

	event := filepath.Join(root, "LogEvent.cfc")
	if err := os.WriteFile(event, []byte(`component { function getMessage() {} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	appender := filepath.Join(root, "Appender.cfc")
	if err := os.WriteFile(appender, []byte(`component { function logMessage(required test.LogEvent event) {} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	g := &generator{src: &frameworkapi.Source{Prefix: "test"}, root: root, out: out}
	if err := g.emit(appender); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(out, "Appender.cfc"))
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(g.todo, event) || !strings.Contains(string(data), "required test.LogEvent event") {
		t.Fatalf("argument component was not queued and qualified: todo=%v stub=%s", g.todo, data)
	}
}

func TestGeneratedStubPreservesInterface(t *testing.T) {
	for _, source := range []string{
		`interface { function getName(); }`,
		`<cfinterface><cffunction name="getName"></cffunction></cfinterface>`,
	} {
		t.Run(source, func(t *testing.T) {
			root, out := t.TempDir(), t.TempDir()

			path := filepath.Join(root, "ICache.cfc")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}

			g := &generator{src: &frameworkapi.Source{Prefix: "test"}, root: root, out: out}
			if err := g.emit(path); err != nil {
				t.Fatal(err)
			}

			data, err := os.ReadFile(filepath.Join(out, "ICache.cfc"))
			if err != nil {
				t.Fatal(err)
			}

			if !interfaceRe.Match(data) || !strings.Contains(string(data), "function getName();") {
				t.Fatalf("stub lost interface declaration or method: %s", data)
			}
		})
	}
}

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
