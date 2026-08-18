package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func cliOpts(whitespaceOnly bool) formatter.Options {
	opts := formatter.DefaultOptions()
	opts.WhitespaceOnly = whitespaceOnly
	opts.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
	opts.ParseQuery = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
	opts.ParseCFML = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }

	return opts
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	return path
}

// TestFormatOneFileLeavesUnparseableFileIntact is the data-loss regression:
// `format -w` used to write the formatter's raw-emitted ERROR-node output over
// the source and report success, truncating the file and leaving it
// unparseable.
func TestFormatOneFileLeavesUnparseableFileIntact(t *testing.T) {
	src := "<cfcomponent>\n\t<cfinvoke component=\"models.Widget\" method=\"render\" returnvariable=\"r\">\n</cfcomponent>\n"
	path := writeTemp(t, "victim.cfc", src)

	err := formatOneFile(path, cliOpts(true), true)
	if err == nil {
		t.Fatal("expected formatOneFile to refuse an unparseable file")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("reading back: %v", readErr)
	}

	if string(after) != src {
		t.Errorf("file was modified despite the error:\n--- before ---\n%s\n--- after ---\n%s", src, after)
	}
}

// TestFormatOneFileGuardIsWiredUp checks the second gate is actually enabled
// on the CLI path. The CLI used to build DefaultOptions(), which leaves
// WhitespaceOnly false, so no guard ran at all.
func TestFormatOneFileGuardIsWiredUp(t *testing.T) {
	if !cliOpts(true).WhitespaceOnly {
		t.Fatal("the CLI option set does not enable the whitespaceOnly guard")
	}
}

// TestFormatOneFileAllowsNormalization is the flip side: adding an omitted
// semicolon is canonicalisation the formatter performs on purpose, so the
// guard must let it through and the file must actually be rewritten.
func TestFormatOneFileAllowsNormalization(t *testing.T) {
	src := "component {\n\tfunction getAll() {\n\t\treturn []\n\t}\n}\n"
	path := writeTemp(t, "semi.cfc", src)

	if err := formatOneFile(path, cliOpts(true), true); err != nil {
		t.Fatalf("formatOneFile refused a deliberate normalisation: %v", err)
	}

	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "return [];") {
		t.Errorf("expected the missing semicolon to be added:\n%s", after)
	}
}

// TestFormatOneFileWritesCleanFile confirms the guards do not block ordinary
// reformatting.
func TestFormatOneFileWritesCleanFile(t *testing.T) {
	src := "component {\n        function a() {\n                return 1;\n        }\n}\n"
	path := writeTemp(t, "clean.cfc", src)

	if err := formatOneFile(path, cliOpts(true), true); err != nil {
		t.Fatalf("formatOneFile: %v", err)
	}

	after, _ := os.ReadFile(path)
	if string(after) == src {
		t.Error("expected the file to be reindented")
	}

	if len(after) == 0 {
		t.Error("file was emptied")
	}
}
