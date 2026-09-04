package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// TestFormatOneFilePreservesMode is the mode-clobbering regression: the write
// used a hardcoded 0o644, so any file with a different mode silently changed
// permissions as a side effect of being formatted.
func TestFormatOneFilePreservesMode(t *testing.T) {
	src := "component {\n        function a() {\n                return 1;\n        }\n}\n"
	path := writeTemp(t, "modes.cfc", src)

	const mode = 0o750
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if err := formatOneFile(path, cliOpts(true), true); err != nil {
		t.Fatalf("formatOneFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if got := info.Mode().Perm(); got != mode {
		t.Errorf("mode changed: want %o, got %o", mode, got)
	}
}

// TestFormatOneFileSkipsUnchangedFile is the no-op-write regression: an
// already-formatted file was rewritten anyway, bumping its mtime and waking
// every watcher and rebuild on the tree.
func TestFormatOneFileSkipsUnchangedFile(t *testing.T) {
	src := "component {\n        function a() {\n                return 1;\n        }\n}\n"
	path := writeTemp(t, "idempotent.cfc", src)

	if err := formatOneFile(path, cliOpts(true), true); err != nil {
		t.Fatalf("first format: %v", err)
	}

	stamp := time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := formatOneFile(path, cliOpts(true), true); err != nil {
		t.Fatalf("second format: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if !info.ModTime().Equal(stamp) {
		t.Errorf("already-formatted file was rewritten: mtime moved from %v to %v", stamp, info.ModTime())
	}
}

// TestFormatOneFileFollowsSymlink checks the atomic rename lands on the link's
// target. Renaming onto the link itself would silently replace the symlink
// with a regular file.
func TestFormatOneFileFollowsSymlink(t *testing.T) {
	src := "component {\n        function a() {\n                return 1;\n        }\n}\n"
	target := writeTemp(t, "real.cfc", src)
	link := filepath.Join(filepath.Dir(target), "link.cfc")

	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	if err := formatOneFile(link, cliOpts(true), true); err != nil {
		t.Fatalf("formatOneFile: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file")
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}

	if string(after) == src {
		t.Error("the symlink's target was not reformatted")
	}
}

// TestWriteFileInPlaceRefusesStaleWrite covers the race the atomic write opens
// up: reading, formatting, and writing are three steps, and an edit landing in
// between must not be overwritten by output computed from the old bytes.
func TestWriteFileInPlaceRefusesStaleWrite(t *testing.T) {
	onDisk := "edited by someone else\n"
	path := writeTemp(t, "raced.cfc", onDisk)

	err := writeFileInPlace(path, []byte("what we read earlier\n"), []byte("our formatted output\n"))
	if err == nil {
		t.Fatal("expected writeFileInPlace to refuse a stale write")
	}

	after, _ := os.ReadFile(path)
	if string(after) != onDisk {
		t.Errorf("the concurrent edit was overwritten: %q", after)
	}
}

// TestFormatOptionsForReadsConfig is the config regression: `format` built
// options from formatter.DefaultOptions() alone and so ignored every key under
// "formatting", formatting differently than the editor did for the same file.
func TestFormatOptionsForReadsConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"workspaceName":"t","formatting":{"enabled":true,"indentWidth":2,"lowercaseTags":false,"lineWidth":60}}`

	if err := os.WriteFile(filepath.Join(dir, ".cfmleditor.json"), []byte(cfg), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	opts := formatOptionsFor("", false)(filepath.Join(dir, "sub", "page.cfm"))

	if opts.IndentWidth != 2 {
		t.Errorf("indentWidth: want 2, got %d", opts.IndentWidth)
	}

	if opts.LowercaseTags {
		t.Error("lowercaseTags: config set false, options say true")
	}

	if opts.LineWidth != 60 {
		t.Errorf("lineWidth: want 60, got %d", opts.LineWidth)
	}

	if !opts.WhitespaceOnly {
		t.Error("whitespaceOnly should default on when the config omits it")
	}

	if opts.ParseScript == nil || opts.ParseQuery == nil || opts.ParseCFML == nil {
		t.Error("parse hooks were not installed, sub-grammar content would be emitted verbatim")
	}
}

// TestFormatOptionsForDefaultsWithoutConfig checks a file with no config above
// it still gets the documented defaults rather than a zero-valued option set.
func TestFormatOptionsForDefaultsWithoutConfig(t *testing.T) {
	opts := formatOptionsFor("", false)(filepath.Join(t.TempDir(), "page.cfm"))

	if opts.IndentWidth != 4 || opts.LineWidth != 100 || opts.AttrBreakThreshold != 4 {
		t.Errorf("defaults not applied: indent=%d width=%d attrs=%d", opts.IndentWidth, opts.LineWidth, opts.AttrBreakThreshold)
	}

	if !opts.LowercaseTags || !opts.WhitespaceOnly || !opts.SelfCloseTags {
		t.Error("boolean defaults not applied")
	}
}

// TestFormatOptionsForAllowNonWhitespace checks the flag still loosens the
// guard once the value comes from config rather than from the flag alone.
func TestFormatOptionsForAllowNonWhitespace(t *testing.T) {
	if formatOptionsFor("", true)(filepath.Join(t.TempDir(), "page.cfm")).WhitespaceOnly {
		t.Error("--allow-non-whitespace did not disable the guard")
	}
}
