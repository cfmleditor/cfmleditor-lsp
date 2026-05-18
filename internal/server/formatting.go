package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
	"go.uber.org/zap"
)

func (s *Server) handleFormatting(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
	if !s.Formatting.Enabled {
		return reply(ctx, nil, nil)
	}

	var params protocol.DocumentFormattingParams
	if err := json.Unmarshal(req.Params(), &params); err != nil {
		return reply(ctx, nil, err)
	}

	content, ok := s.getDocument(uri.URI(params.TextDocument.URI))
	if !ok {
		return reply(ctx, nil, nil)
	}

	start := time.Now()
	formatted, err := formatDocument(content, params.Options, s.Formatting)
	elapsed := time.Since(start)
	if err != nil {
		s.logger.Warn("formatting failed", zap.String("uri", string(params.TextDocument.URI)), zap.Duration("elapsed", elapsed), zap.Error(err))
		_ = s.conn.Notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeWarning,
			Message: "Formatting failed: " + err.Error(),
		})
		return reply(ctx, []protocol.TextEdit{}, nil)
	}
	if formatted == content {
		s.logger.Debug("formatting complete (no changes)", zap.String("uri", string(params.TextDocument.URI)), zap.Duration("elapsed", elapsed))
		return reply(ctx, []protocol.TextEdit{}, nil)
	}

	s.logger.Debug("formatting complete", zap.String("uri", string(params.TextDocument.URI)), zap.Duration("elapsed", elapsed))

	// Idempotency check: format again and verify the result is stable.
	if s.Formatting.Debug {
		formatted2, err2 := formatDocument(formatted, params.Options, s.Formatting)
		if err2 != nil {
			s.logger.Warn("formatting idempotency check failed", zap.String("uri", string(params.TextDocument.URI)), zap.Error(err2))
			_ = s.conn.Notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeWarning,
				Message: "Formatting is not idempotent: second pass failed: " + err2.Error(),
			})
		} else if formatted2 != formatted {
			s.logger.Warn("formatting is not idempotent", zap.String("uri", string(params.TextDocument.URI)))
			_ = s.conn.Notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
				Type:    protocol.MessageTypeWarning,
				Message: "Formatting is not idempotent: second pass produced different output",
			})
		}
	}

	lines := countNewlines(content)
	edits := []protocol.TextEdit{{
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: uint32(lines + 1), Character: 0},
		},
		NewText: formatted,
	}}
	return reply(ctx, edits, nil)
}

func formatDocument(content string, opts protocol.FormattingOptions, cfg FormattingConfig) (string, error) {
	src := []byte(content)
	tree := language.Parse(language.CFML, src, nil)

	if tree.RootNode().HasError() {
		errNode := findErrorNode(tree.RootNode())
		if errNode != nil {
			pos := errNode.StartPosition()
			snippet := string(src[errNode.StartByte():errNode.EndByte()])
			if len(snippet) > 50 {
				snippet = snippet[:50] + "..."
			}
			return content, fmt.Errorf("parse error at line %d, col %d near %q", pos.Row+1, pos.Column+1, snippet)
		}
		return content, fmt.Errorf("parse error in document, cannot format")
	}

	fmtOpts := formatter.DefaultOptions()
	fmtOpts.UseTabs = !opts.InsertSpaces
	if opts.TabSize > 0 {
		fmtOpts.IndentWidth = int(opts.TabSize)
	}
	fmtOpts.SelfCloseTags = cfg.SelfCloseTags
	fmtOpts.WhitespaceOnly = cfg.WhitespaceOnly
	fmtOpts.LowercaseTags = cfg.LowercaseTags
	fmtOpts.LowercaseAttributes = cfg.LowercaseAttributes
	fmtOpts.DoubleQuoteAttributes = cfg.DoubleQuoteAttributes
	fmtOpts.UppercaseSQLKeywords = cfg.UppercaseSQLKeywords
	fmtOpts.ScopeCase = cfg.ScopeCase
	fmtOpts.CommaPosition = cfg.CommaPosition
	fmtOpts.CommaPositionSQL = cfg.CommaPositionSQL
	if cfg.LineWidth > 0 {
		fmtOpts.LineWidth = cfg.LineWidth
	}
	if cfg.AttrBreakThreshold > 0 {
		fmtOpts.AttrBreakThreshold = cfg.AttrBreakThreshold
	}
	if cfg.IndentWidth > 0 {
		fmtOpts.IndentWidth = cfg.IndentWidth
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
	result := string(out)
	return result, nil
}

func countNewlines(s string) int {
	n := 0
	for _, c := range s {
		if c == '\n' {
			n++
		}
	}
	return n
}

func findErrorNode(n *sitter.Node) *sitter.Node {
	if n.IsError() || n.IsMissing() {
		return n
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if found := findErrorNode(n.Child(i)); found != nil {
			return found
		}
	}
	return nil
}

