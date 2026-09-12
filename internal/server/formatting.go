package server

import (
	"context"
	json "github.com/go-json-experiment/json"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"go.lsp.dev/protocol"
)

func (s *Server) handleFormatting(ctx context.Context, rawParams []byte) (any, error) {
	if !s.Formatting.Enabled {
		return nil, nil
	}

	var params protocol.DocumentFormattingParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	s.log.Info("formatting document", cflog.String("uri", string(params.TextDocument.URI)))

	content, ok := s.getDocument(params.TextDocument.URI)
	if !ok {
		return nil, nil
	}

	start := time.Now()
	formatted, err := formatDocument(content, params.Options, s.Formatting)
	elapsed := time.Since(start)

	if err != nil {
		s.log.Warn("formatting failed", cflog.String("uri", string(params.TextDocument.URI)), cflog.Duration("elapsed", elapsed), cflog.Err(err))
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeWarning,
			Message: "Formatting failed: " + err.Error(),
		})

		return []protocol.TextEdit{}, nil
	}

	if formatted == content {
		s.log.Debug("formatting complete (no changes)", cflog.String("uri", string(params.TextDocument.URI)), cflog.Duration("elapsed", elapsed))

		return []protocol.TextEdit{}, nil
	}

	s.log.Debug("formatting complete", cflog.String("uri", string(params.TextDocument.URI)), cflog.Duration("elapsed", elapsed))

	// Idempotency check: format again and verify the result is stable.
	if s.Formatting.Debug {
		formatted2, err2 := formatDocument(formatted, params.Options, s.Formatting)
		if err2 != nil {
			s.log.Warn("formatting idempotency check failed", cflog.String("uri", string(params.TextDocument.URI)), cflog.Err(err2))
			s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeWarning,
				Message: "Formatting is not idempotent: second pass failed: " + err2.Error(),
			})
		} else if formatted2 != formatted {
			s.log.Warn("formatting is not idempotent", cflog.String("uri", string(params.TextDocument.URI)))
			s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeWarning,
				Message: "Formatting is not idempotent: second pass produced different output",
			})
		}
	}

	lines := parser.CountNewlines(content)
	edits := []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: uint32(lines + 1), Character: 0},
		},
		NewText: formatted,
	}}

	return edits, nil
}

func formatDocument(content string, opts protocol.FormattingOptions, cfg config.ResolvedFormatting) (string, error) {
	src := []byte(content)
	tree := language.Parse(language.CFML, src, nil)

	// The tree owns C memory that the Go GC does not account for, so it has to
	// be released explicitly. Without this every format request — including the
	// two Format calls the idempotency check makes — leaks one whole CST, which
	// on a long editing session with format-on-save is the server's largest
	// source of unexplained memory growth.
	defer tree.Close()

	if err := formatter.ParseError(tree, src); err != nil {
		return content, err
	}

	fmtOpts := cfg.FormatterOptions()
	fmtOpts.UseTabs = !opts.InsertSpaces

	// A workspace indentWidth is authoritative; the editor's tabSize only fills
	// in when the workspace has not set one.
	if cfg.IndentWidth == 0 && opts.TabSize > 0 {
		fmtOpts.IndentWidth = int(opts.TabSize)
	}

	fmtOpts.ParseScript = func(s []byte) *sitter.Tree {
		return language.Parse(language.CFScript, s, nil)
	}
	fmtOpts.ParseQuery = func(s []byte) *sitter.Tree {
		return language.Parse(language.CFQuery, s, nil)
	}
	fmtOpts.ParseCFML = func(s []byte) *sitter.Tree {
		return language.Parse(language.CFML, s, nil)
	}

	out, err := formatter.Format(src, tree, fmtOpts)
	if err != nil {
		return content, err
	}

	return applyFinalNewlineOptions(content, string(out), opts), nil
}

// applyFinalNewlineOptions honours the two end-of-file options the editor sends
// with every formatting request (LSP 3.15) and the formatter has no way to know
// about, because it rebuilds the document rather than editing it: it always
// ends its output with exactly one newline, whatever the source did.
//
// That is not a safe default to keep unconditionally. VS Code's own defaults
// for both `files.insertFinalNewline` and `files.trimFinalNewlines` are false,
// so on a stock editor the formatter was adding a final newline the user's
// settings say not to add and removing trailing blank lines they say to keep.
//
// Both fields are optional pointers. A client that sends neither — the
// `cfmleditor.format` command does, and so does any client older than 3.15 —
// gets what the formatter produced, so nothing changes for them.
//
// The two options answer different questions and neither implies the other:
// insertFinalNewline is only about a source that ended without one, and
// trimFinalNewlines is only about the blank lines after it. Setting
// insertFinalNewline to false on a file that already ends in a newline asks for
// nothing, and does nothing here.
//
// Only trailing newlines move, so the whitespaceOnly guard inside Format has
// already passed on content this cannot change. Re-formatting is stable: on the
// second pass the restored ending is what the source now has, and each rule
// asks for it again.
func applyFinalNewlineOptions(content, out string, opts protocol.FormattingOptions) string {
	srcNL := trailingNewlines(content)
	want := trailingNewlines(out)

	if opts.TrimFinalNewlines != nil && !*opts.TrimFinalNewlines && srcNL > want {
		want = srcNL
	}

	if opts.InsertFinalNewline != nil && !*opts.InsertFinalNewline && srcNL == 0 {
		want = 0
	}

	body := strings.TrimRight(out, "\n")

	if want == len(out)-len(body) {
		return out
	}

	return body + strings.Repeat("\n", want)
}

// trailingNewlines counts the newlines a document ends with. The formatter
// emits "\n" line endings, so a "\r\n" source has its "\r" folded into the
// preceding line by the time this sees it and there is nothing to count twice.
func trailingNewlines(s string) int {
	return len(s) - len(strings.TrimRight(s, "\n"))
}
