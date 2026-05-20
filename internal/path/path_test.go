package path

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseApplicationMappings_ExpandPath(t *testing.T) {
	content := `component {
		this.mappings["/models"] = expandPath("./src/models");
		this.mappings["/lib"] = expandPath("./lib");
	}`
	appDir := "/project"
	got := ParseApplicationMappings(content, appDir)
	if got["models"] != filepath.Join(appDir, "src/models") {
		t.Errorf("models = %q, want %q", got["models"], filepath.Join(appDir, "src/models"))
	}
	if got["lib"] != filepath.Join(appDir, "lib") {
		t.Errorf("lib = %q, want %q", got["lib"], filepath.Join(appDir, "lib"))
	}
}

func TestParseApplicationMappings_PlainString(t *testing.T) {
	content := `this.mappings["/vendor"] = "/opt/cfml/vendor";`
	got := ParseApplicationMappings(content, "/project")
	if got["vendor"] != "/opt/cfml/vendor" {
		t.Errorf("vendor = %q, want %q", got["vendor"], "/opt/cfml/vendor")
	}
}

func TestParseApplicationMappings_SingleQuotes(t *testing.T) {
	content := `this.mappings['/utils'] = expandPath('./utils');`
	got := ParseApplicationMappings(content, "/app")
	if got["utils"] != filepath.Join("/app", "utils") {
		t.Errorf("utils = %q, want %q", got["utils"], filepath.Join("/app", "utils"))
	}
}

func TestParseApplicationMappings_NoMappings(t *testing.T) {
	content := `component { this.name = "myApp"; }`
	got := ParseApplicationMappings(content, "/project")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseApplicationMappings_RelativeStringValue(t *testing.T) {
	content := `this.mappings["/shared"] = "./shared/lib";`
	got := ParseApplicationMappings(content, "/project")
	if got["shared"] != filepath.Join("/project", "shared/lib") {
		t.Errorf("shared = %q, want %q", got["shared"], filepath.Join("/project", "shared/lib"))
	}
}

func TestResolvePath_Found(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "models")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "User.cfc"), []byte("component {}"), 0o644)

	got := ResolvePath("models.User", dir, nil)
	want := filepath.Join(sub, "User.cfc")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePath_NotFound(t *testing.T) {
	got := ResolvePath("no.Such", t.TempDir(), nil)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolvePath_Mapping(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "Helper.cfc"), []byte("component {}"), 0o644)

	mappings := map[string]string{"mylib": libDir}
	got := ResolvePath("mylib.Helper", t.TempDir(), mappings)
	want := filepath.Join(libDir, "Helper.cfc")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolvePath_MappingNestedPath(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib", "sub")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "Thing.cfc"), []byte("component {}"), 0o644)

	mappings := map[string]string{"mylib": filepath.Join(dir, "lib")}
	got := ResolvePath("mylib.sub.Thing", t.TempDir(), mappings)
	want := filepath.Join(libDir, "Thing.cfc")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
