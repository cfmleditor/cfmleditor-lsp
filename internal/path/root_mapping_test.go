package path

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootMappingDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"root/models/Product.cfc", "local/models/Product.cfc", "explicit/Product.cfc", "app/models/Product.cfc"} {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(file, []byte("component {}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write := func(name, content string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".cfconfig.json", `{"mappings":{"/":{"physical":"root"},"/models":"explicit"}}`)
	InvalidateAppMappingsCache()
	t.Cleanup(InvalidateAppMappingsCache)

	mappings := LoadAppMappings(dir)
	if got := mappings[""]; got != filepath.Join(dir, "root") {
		t.Fatalf("root default %q", got)
	}

	if got := ResolvePath("models.Product", dir, mappings); got != filepath.Join(dir, "explicit", "Product.cfc") {
		t.Fatalf("named precedence %q", got)
	}

	delete(mappings, "models")

	if got := ResolvePath("models.Product", filepath.Join(dir, "local"), mappings); got != filepath.Join(dir, "local", "models", "Product.cfc") {
		t.Fatalf("relative precedence %q", got)
	}

	if got := ResolvePath("models.Product", dir, mappings); got != filepath.Join(dir, "root", "models", "Product.cfc") {
		t.Fatalf("root fallback %q", got)
	}

	write("Application.cfc", `component {this.mappings["/"]=expandPath("./app");}`)
	InvalidateAppMappingsCache()

	mappings = LoadAppMappings(dir)
	delete(mappings, "models")

	if got := ResolvePath("models.Product", dir, mappings); got != filepath.Join(dir, "app", "models", "Product.cfc") {
		t.Fatalf("application override %q", got)
	}

	write("Application.cfc", `component {this.mappings={"/":expandPath("./local")};}`)
	InvalidateAppMappingsCache()

	mappings = LoadAppMappings(dir)
	delete(mappings, "models")

	if got := ResolvePath("models.Product", dir, mappings); got != filepath.Join(dir, "local", "models", "Product.cfc") {
		t.Fatalf("literal root %q", got)
	}

	write("Application.cfc", `component {this.mappings["/"]=runtimeValue;}`)
	InvalidateAppMappingsCache()

	if root := LoadAppMappings(dir)[""]; root != "" {
		t.Fatal("dynamic root kept stale default")
	}

	write("Application.cfc", `component {this.mappings={"/":runtimeValue};}`)
	InvalidateAppMappingsCache()

	if root := LoadAppMappings(dir)[""]; root != "" {
		t.Fatal("dynamic literal root kept stale default")
	}
}
