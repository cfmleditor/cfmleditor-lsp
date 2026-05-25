package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
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
		s.log.Warn("formatting failed", cflog.String("uri", string(params.TextDocument.URI)), cflog.Duration("elapsed", elapsed), cflog.Err(err))
		s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
			Type:    protocol.MessageTypeWarning,
			Message: "Formatting failed: " + err.Error(),
		})
		return reply(ctx, []protocol.TextEdit{}, nil)
	}
	if formatted == content {
		s.log.Debug("formatting complete (no changes)", cflog.String("uri", string(params.TextDocument.URI)), cflog.Duration("elapsed", elapsed))
		return reply(ctx, []protocol.TextEdit{}, nil)
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
	return reply(ctx, edits, nil)
}

func formatDocument(content string, opts protocol.FormattingOptions, cfg config.ResolvedFormatting) (string, error) {
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
	fmtOpts.QueryFormat = cfg.QueryFormat
	fmtOpts.LowercaseTags = cfg.LowercaseTags
	fmtOpts.LowercaseAttributes = cfg.LowercaseAttributes
	fmtOpts.DoubleQuoteAttributes = cfg.DoubleQuoteAttributes
	fmtOpts.QueryUppercaseKeywords = cfg.QueryUppercaseKeywords
	fmtOpts.ScopeCase = cfg.ScopeCase
	fmtOpts.CommaPosition = cfg.CommaPosition
	fmtOpts.QueryCommaPosition = cfg.QueryCommaPosition
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
