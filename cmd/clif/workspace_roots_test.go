package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cfmleditor/clif/internal/daemon"
	"github.com/cfmleditor/clif/internal/resolve"
	"github.com/cfmleditor/clif/internal/unresolved"
	"github.com/cfmleditor/clif/internal/vfs"
)

func workspaceFixture(t *testing.T) (root, tools string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "project")

	files := map[string]string{
		"models/User.cfc":         `component { function real(){} }`,
		"vendor/tools/Helper.cfc": `component { function help(){} }`,
		"svc/Sib.cfc":             `component { function real(){} }`,
		"svc/Main.cfc": `component { function run(){
 var user=new models.User(); user.real(); user.noUser();
 var tool=new tools.Helper(); tool.help(); tool.noTool();
 var sibling=new Sib(); sibling.real(); sibling.noSibling();
 } }`,
	}
	for rel, source := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return root, filepath.Join(root, "vendor", "tools")
}

func TestUnresolvedConfigRetainsArgumentWorkspace(t *testing.T) {
	root, tools := workspaceFixture(t)
	folders := []string{root, tools}
	fsys := vfs.OS{}
	before := unresolved.Scan(fsys, collectCFMLFiles(fsys, folders), nil, unresolvedOptions(nil, folders, &unresolvedFlags{}))

	for _, settings := range []string{`{}`, `{"frameworks":["testbox"]}`} {
		if err := os.WriteFile(filepath.Join(root, ".cfmleditor.json"), []byte(settings), 0o600); err != nil {
			t.Fatal(err)
		}

		cfg, err := daemon.FindConfig(root)
		if err != nil || cfg == nil {
			t.Fatalf("config not loaded: %v", err)
		}

		options := unresolvedOptions(cfg, folders, &unresolvedFlags{})

		after := unresolved.Scan(fsys, collectCFMLFiles(fsys, folders), nil, options)
		if !reflect.DeepEqual(before.Calls, after.Calls) || before.Resolved != after.Resolved || before.Indexed != after.Indexed || before.Scanned != after.Scanned {
			t.Fatalf("config without workspacePaths changed lookup or coverage: before=%+v after=%+v", before, after)
		}

		if len(after.Calls) != 3 || after.Indexed != 4 || after.Scanned != 4 {
			t.Fatalf("unexpected fixture coverage: %+v", after)
		}

		for _, c := range after.Calls {
			if !strings.Contains(c.Reason, "not found in") {
				t.Errorf("missing method became a missing receiver/component: %+v", c)
			}
		}
	}
}

func TestCLIWorkspaceRootsPreserveExplicitPathsAndFileTargets(t *testing.T) {
	root, tools := workspaceFixture(t)

	configPath := filepath.Join(root, ".cfmleditor.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &daemon.Config{Path: configPath}
	file := filepath.Join(root, "svc", "Main.cfc")
	folders := cliWorkspaceFolders(vfs.OS{}, cfg, []string{file, filepath.Dir(file), tools})

	want := []string{filepath.Dir(file), tools}
	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("file lookup roots=%v, want %v", folders, want)
	}
	// Lookup may use siblings, but a single-file report must stay single-file.
	if err := os.WriteFile(filepath.Join(root, "svc", "Unrelated.cfc"), []byte(`component { function other(){missingElsewhere();} }`), 0o600); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() { cmdUnresolved([]string{"--json", file}) })

	var calls []unresolved.Call
	if err := json.Unmarshal([]byte(out), &calls); err != nil {
		t.Fatal(err)
	}

	for _, call := range calls {
		if call.File != file {
			t.Fatalf("file target widened the report: %+v", call)
		}
	}

	if len(calls) == 0 {
		t.Fatal("single-file scan lost its calls")
	}

	log := captureStdout(t, func() {
		stderr := os.Stderr

		os.Stderr = os.Stdout
		defer func() { os.Stderr = stderr }()

		cmdUnresolved([]string{"--json", file})
	})
	if !strings.Contains(log, "Indexing 1 files") || !strings.Contains(log, "(1 files)") {
		t.Fatalf("file lookup roots widened indexing or scanning: %s", log)
	}

	if err := os.WriteFile(configPath, []byte(`{"workspacePaths":["vendor/tools"]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := cliWorkspaceFolders(vfs.OS{}, cfg, []string{root}); !reflect.DeepEqual(got, []string{tools}) {
		t.Fatalf("explicit workspacePaths replaced: %v", got)
	}
}

func TestCLIResolutionCommandsShareDefaultRoots(t *testing.T) {
	root, tools := workspaceFixture(t)
	if err := os.WriteFile(filepath.Join(root, ".cfmleditor.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	fsys := vfs.OS{}
	files := collectCFMLFiles(fsys, []string{root})
	deps, _ := depsResolver(fsys, []string{root}, files)
	mcp, _ := buildExplainResolver(root)

	scanRoots, fallback, shared, _ := routeWorkspace(fsys, root, &graphFlags{paths: []string{root, tools}, quiet: true})
	if !reflect.DeepEqual(scanRoots, []string{root, tools}) {
		t.Fatalf("graph scan roots changed: %v", scanRoots)
	}

	local := newConfigSet(fsys, shared, &fallback).For(filepath.Join(root, "svc", "Main.cfc"))
	for name, r := range map[string]*resolve.Resolver{"deps": deps, "mcp": mcp, "graph": fallback.Resolver, "per-file graph": local.Resolver} {
		if r.ResolveFunc("models.User", "real", filepath.Join(root, "svc")) == nil {
			t.Errorf("%s lost default workspace lookup", name)
		}

		if r.ResolveFunc("models.User", "noUser", filepath.Join(root, "svc")) != nil {
			t.Errorf("%s accepted a missing method", name)
		}
	}

	if local.Resolver.ResolveFunc("tools.Helper", "help", root) == nil {
		t.Fatal("per-file graph config lost the explicitly scanned package root")
	}
	// Exercise the real explain command: it must index files with a config that
	// only supplies settings, just as the scan does.
	out := captureStdout(t, func() { cmdExplain([]string{"--root", root, filepath.Join(root, "svc", "Main.cfc"), "2", "real"}) })
	if !strings.Contains(out, "=> resolved") {
		t.Fatalf("configured explain failed to resolve real(): %s", out)
	}
}
