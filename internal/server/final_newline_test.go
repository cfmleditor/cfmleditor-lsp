package server

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"go.lsp.dev/protocol"
)

func boolp(b bool) *bool { return &b }

func fmtOpts(insertFinal, trimFinal *bool) protocol.FormattingOptions {
	return protocol.FormattingOptions{
		InsertSpaces:       false,
		TabSize:            4,
		InsertFinalNewline: insertFinal,
		TrimFinalNewlines:  trimFinal,
	}
}

func resolvedFormatting() config.ResolvedFormatting {
	return config.Resolve(&config.JSON{Formatting: &config.Formatting{}}, ".").Formatting
}

const eofSrc = "component {\n\tfunction f() {\n\t\tx = 1;\n\t}\n}"

func formatEOF(t *testing.T, content string, opts protocol.FormattingOptions) string {
	t.Helper()

	out, err := formatDocument(content, opts, resolvedFormatting())
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return out
}

// TestFormatterAlwaysEndsWithOneNewlineByDefault is the baseline the two
// options are measured against, and the reason they are needed: the formatter
// rebuilds the document rather than editing it, so it ends with exactly one
// newline whatever the source did. A client that sends neither option — the
// `cfmleditor.format` command, or anything older than LSP 3.15 — has to keep
// getting exactly that.
func TestFormatterAlwaysEndsWithOneNewlineByDefault(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"no final newline":     eofSrc,
		"one final newline":    eofSrc + "\n",
		"four trailing blanks": eofSrc + "\n\n\n\n",
	} {
		got := formatEOF(t, content, fmtOpts(nil, nil))
		if n := len(got) - len(strings.TrimRight(got, "\n")); n != 1 {
			t.Errorf("%s: output ends with %d newlines, want 1", name, n)
		}
	}
}

// TestInsertFinalNewlineFalseKeepsASourceWithoutOne covers a source that ended
// without a newline and an editor that says not to add one. VS Code's own
// default for files.insertFinalNewline is false, so this is the stock setting,
// not an exotic one.
func TestInsertFinalNewlineFalseKeepsASourceWithoutOne(t *testing.T) {
	t.Parallel()

	got := formatEOF(t, eofSrc, fmtOpts(boolp(false), nil))
	if strings.HasSuffix(got, "\n") {
		t.Errorf("insertFinalNewline=false still added a final newline:\n%q", got[len(got)-10:])
	}
}

// TestInsertFinalNewlineFalseLeavesAnExistingOneAlone is the boundary. The
// option is about inserting, not removing: a source that already ends in a
// newline is asking for nothing, and must keep it.
func TestInsertFinalNewlineFalseLeavesAnExistingOneAlone(t *testing.T) {
	t.Parallel()

	got := formatEOF(t, eofSrc+"\n", fmtOpts(boolp(false), nil))
	if !strings.HasSuffix(got, "}\n") {
		t.Errorf("insertFinalNewline=false stripped a newline the source had:\n%q", got[max(0, len(got)-10):])
	}
}

// TestTrimFinalNewlinesFalseKeepsTrailingBlankLines covers the other stock VS
// Code default: files.trimFinalNewlines is false, and the formatter was
// collapsing every trailing blank line regardless.
func TestTrimFinalNewlinesFalseKeepsTrailingBlankLines(t *testing.T) {
	t.Parallel()

	got := formatEOF(t, eofSrc+"\n\n\n\n", fmtOpts(nil, boolp(false)))
	if n := len(got) - len(strings.TrimRight(got, "\n")); n != 4 {
		t.Errorf("trimFinalNewlines=false kept %d trailing newlines, want the source's 4", n)
	}
}

// TestTrimFinalNewlinesTrueTrims is the other half — an editor asking for the
// trim explicitly gets it, the same as the default.
func TestTrimFinalNewlinesTrueTrims(t *testing.T) {
	t.Parallel()

	got := formatEOF(t, eofSrc+"\n\n\n\n", fmtOpts(nil, boolp(true)))
	if n := len(got) - len(strings.TrimRight(got, "\n")); n != 1 {
		t.Errorf("trimFinalNewlines=true left %d trailing newlines, want 1", n)
	}
}

// TestFinalNewlineOptionsAreIdempotent is the failure mode a post-pass on the
// document's ending invites: the second format sees the restored ending as the
// source's own and applies the rule to it again.
func TestFinalNewlineOptionsAreIdempotent(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		content string
		opts    protocol.FormattingOptions
	}{
		"insert=false, no newline":  {eofSrc, fmtOpts(boolp(false), nil)},
		"trim=false, four blanks":   {eofSrc + "\n\n\n\n", fmtOpts(nil, boolp(false))},
		"both false, no newline":    {eofSrc, fmtOpts(boolp(false), boolp(false))},
		"both false, four blanks":   {eofSrc + "\n\n\n\n", fmtOpts(boolp(false), boolp(false))},
		"neither sent, four blanks": {eofSrc + "\n\n\n\n", fmtOpts(nil, nil)},
	}

	for name, tc := range cases {
		once := formatEOF(t, tc.content, tc.opts)
		if twice := formatEOF(t, once, tc.opts); twice != once {
			t.Errorf("%s: not idempotent\n--- pass 1\n%q\n--- pass 2\n%q", name, once, twice)
		}
	}
}
