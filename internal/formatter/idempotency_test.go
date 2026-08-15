package formatter

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

// formatOnce formats src with the standard option set used by the CLI.
func formatOnce(t *testing.T, src []byte) []byte {
	t.Helper()

	opts := DefaultOptions()
	opts.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
	opts.ParseQuery = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
	opts.ParseCFML = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }

	tree := language.Parse(language.CFML, src, nil)
	defer tree.Close()

	out, err := Format(src, tree, opts)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	return out
}

// assertIdempotent checks that formatting already-formatted output is a no-op.
func assertIdempotent(t *testing.T, label string, src []byte) {
	t.Helper()

	first := formatOnce(t, src)
	second := formatOnce(t, first)

	if string(first) != string(second) {
		t.Errorf("%s: formatting is not idempotent\n--- first pass ---\n%s\n--- second pass ---\n%s",
			label, first, second)
	}
}

// TestFormatIsIdempotentOnCorpus formats every CFML file in testdata twice and
// requires the second pass to be a no-op. Formatting has to be a fixed point:
// an editor that formats on save would otherwise keep producing new diffs for
// an unchanged file.
func TestFormatIsIdempotentOnCorpus(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata")

	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil //nolint:nilerr // unreadable entries are simply skipped
		}

		switch strings.ToLower(filepath.Ext(path)) {
		case ".cfc", ".cfm", ".cfml":
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking testdata: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no CFML files found in testdata")
	}

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}

		rel, _ := filepath.Rel(root, f)

		t.Run(rel, func(t *testing.T) {
			assertIdempotent(t, rel, src)
		})
	}
}

// TestFormatIdempotentSingleLineFunctions covers the cfscript half of the bug:
// `function f() {}` is expanded into a braced body, and the blank line between
// members used to be decided from the source span, so it only appeared once the
// file had already been formatted.
func TestFormatIdempotentSingleLineFunctions(t *testing.T) {
	src := []byte("component {\n\n    function open() {}\n    function readLine() {}\n    function close() {}\n\n}\n")
	assertIdempotent(t, "single-line functions", src)
}

// TestFormatIdempotentUnclosedCFTag covers the tag half: an unclosed cf tag
// produces a synthetic implicit_cf_end_tag whose text was emitted as content,
// adding a blank line that vanished on the next pass once the closing tag was
// present.
func TestFormatIdempotentUnclosedCFTag(t *testing.T) {
	assertIdempotent(t, "unclosed cfmodule", []byte("<cfmodule template=\"x\">\n"))
}

// TestSingleLineFunctionsSeparatedByBlankLine pins the resulting layout, so the
// fix cannot regress into simply never emitting the separator.
func TestSingleLineFunctionsSeparatedByBlankLine(t *testing.T) {
	src := []byte("component {\n\n    function open() {}\n    function close() {}\n\n}\n")

	got := string(formatOnce(t, src))
	if !strings.Contains(got, "}\n\n\tfunction close()") {
		t.Errorf("expected a blank line between the two expanded functions, got:\n%s", got)
	}
}

// TestConsecutiveSimpleStatementsStayTogether guards the opposite failure: the
// separator is for statements that actually span multiple lines, so ordinary
// one-line statements must not be spread apart.
func TestConsecutiveSimpleStatementsStayTogether(t *testing.T) {
	src := []byte("component {\n\n    property name=\"a\" type=\"string\";\n    property name=\"b\" type=\"string\";\n\n}\n")

	got := string(formatOnce(t, src))
	if strings.Contains(got, "\";\n\n\tproperty name=\"b\"") {
		t.Errorf("consecutive single-line properties should not be separated by a blank line, got:\n%s", got)
	}
}

// TestThrowArgumentsFormatLikeAnyCall pins throw's named arguments to the same
// layout every other call gets. The grammar models `throw(...)` with the same
// `arguments` node as a call expression, but the formatter used to emit that
// list verbatim, leaving throw the only call whose named arguments were spaced
// differently from the rest of the language.
func TestThrowArgumentsFormatLikeAnyCall(t *testing.T) {
	src := []byte("component {\n\n    function f() {\n" +
		"        throw(type=\"A\", message=\"B\");\n" +
		"        writeLog(text=\"C\", type=\"D\");\n" +
		"    }\n\n}\n")

	got := string(formatOnce(t, src))

	for _, want := range []string{
		`throw(type = "A", message = "B");`,
		`writeLog(text = "C", type = "D");`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
}

// TestThrowExpressionFormUnchanged guards the other throw shape: `throw <expr>`
// is not a call and must keep the space after the keyword.
func TestThrowExpressionFormUnchanged(t *testing.T) {
	src := []byte("component {\n\n    function f() {\n" +
		"        throw new Exception(\"oops\");\n" +
		"    }\n\n}\n")

	got := string(formatOnce(t, src))
	if !strings.Contains(got, `throw new Exception("oops");`) {
		t.Errorf("expression-form throw should be unchanged, got:\n%s", got)
	}
}
