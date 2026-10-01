package path

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationMappingSources(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"Application.cfc": `component extends="Base" {
 include "config/paths.cfm";
 this.mappings["/override"]=expandPath("./local");
 this.mappings.dot=variables.root & "/dot";
 }`,
		"Base.cfc": `component { this.mappings["/parent"]=expandPath("./inherited"); }`,
		"config/paths.cfm": `<cfscript>
 variables.here=getDirectoryFromPath(getCurrentTemplatePath());
 variables.root=left(variables.here,len(variables.here)-7);
 this.mappings["/included"]=variables.root & "/included";
 this.mappings["/override"]=variables.root & "/other";
 include "cycle.cfm";
 </cfscript>`,
		"config/cycle.cfm": `<cfinclude template="paths.cfm">`,
		".cfconfig.json":   `{"mappings":{"/server":{"physical":"./server"},"/override":{"physical":"./serverOverride"},"/dynamic":{"physical":"${ENV}"},"/archive":{"archive":"x.lar","primary":"archive"}}}`,
	}
	for name, content := range files {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	InvalidateAppMappingsCache()

	got := LoadAppMappings(dir)
	for key, rel := range map[string]string{"server": "server", "parent": "inherited", "included": "included", "override": "local", "dot": "dot"} {
		if got[key] != filepath.Join(dir, rel) {
			t.Errorf("%s=%q want %q", key, got[key], filepath.Join(dir, rel))
		}
	}

	if got["dynamic"] != "" || got["archive"] != "" {
		t.Fatal("runtime/archive mappings guessed")
	}

	if err := os.WriteFile(filepath.Join(dir, ".cfconfig.json"), []byte(`{"mappings":{"/server":{"physical":"./changed"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	InvalidateAppMappingsCache()

	if got := LoadAppMappings(dir)["server"]; got != filepath.Join(dir, "changed") {
		t.Fatalf("stale mapping %q", got)
	}
}

func TestSelectedCFConfigAndNestedApplication(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"server.json":            `{"cfconfig":{"file":"config/cf.json"}}`,
		"config/cf.json":         `{"mappings":{"/shared":"../shared"}}`,
		".cfconfig.json":         `{"mappings":{"/wrong":"wrong"}}`,
		"Application.cfc":        `component { this.mappings["/outer"]="./outer"; }`,
		"nested/Application.cfc": `component { this.mappings={"/child":expandPath("./child")}; }`,
	} {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	InvalidateAppMappingsCache()

	got := LoadAppMappings(filepath.Join(dir, "nested"))
	if got["shared"] != filepath.Join(dir, "shared") || got["child"] != filepath.Join(dir, "nested", "child") || got["wrong"] != "" || got["outer"] != "" {
		t.Fatalf("wrong selection/isolation: %v", got)
	}
}

func TestMappingsIgnoreCommentsAndReadServerMappedIncludes(t *testing.T) {
	dir := t.TempDir()

	files := map[string]string{
		"Application.cfc": `/* component extends="Wrong" {} */ component {
   include "/settings/paths.cfm";
   this.mappings={"/replacement":variables.replacement};
   this.mappings["/stale"]=this.mappings["/old"];
  }`,
		"Wrong.cfc":      `component {this.mappings["/wrong"]="./wrong";}`,
		"cfg/paths.cfm":  `<cfscript>variables.replacement=expandPath("../replacement");this.mappings["/old"]=expandPath("./old");</cfscript>`,
		".cfconfig.json": `{"mappings":{"/settings":{"physical":"cfg"},"/default":"default"}}`,
	}
	for name, content := range files {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	InvalidateAppMappingsCache()

	got := LoadAppMappings(dir)
	if got["replacement"] != filepath.Join(dir, "replacement") || got["default"] != filepath.Join(dir, "default") || got["old"] != "" || got["stale"] != "" || got["wrong"] != "" {
		t.Fatalf("wrong mappings: %v", got)
	}
}

func TestWholeMappingLiteralUsesPreviousStruct(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Application.cfc")

	write := `component {this.mappings["/old"]=expandPath("./old");this.mappings={"/new":this.mappings["/old"],"/stale":this.mappings["/new"]};}`
	if err := os.WriteFile(file, []byte(write), 0o600); err != nil {
		t.Fatal(err)
	}

	InvalidateAppMappingsCache()

	got := LoadAppMappings(dir)
	if got["new"] != filepath.Join(dir, "old") || got["old"] != "" || got["stale"] != "" {
		t.Fatalf("RHS saw replacement keys: %v", got)
	}
}
