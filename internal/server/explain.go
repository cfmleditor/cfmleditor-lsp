package server

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// handleExplainCall is cfmleditor.explainCall, the server's form of
// `cfmleditor-lsp explain`: for every call site on a line, the steps
// CanResolveCall walked through and its verdict. Arguments are the document
// URI, the 0-based line, and an optional substring of the call's name or
// receiver.
//
// It answers with the server's own resolver and index, so it explains the
// verdict this session reaches, where the CLI builds a resolver of its own
// from the file's nearest config. The report text is the CLI's
// (resolve.WriteExplanation), and it goes out as window/showMessage as well as
// the return value, because Zed does nothing with a command's result.
func (s *Server) handleExplainCall(ctx context.Context, args []protocol.LSPAny) (any, error) {
	docURI, _ := argString(args, 0)
	if docURI == "" {
		return nil, fmt.Errorf("cfmleditor.explainCall requires a document URI argument")
	}

	lineArg, ok := argFloat(args, 1)
	if !ok || lineArg < 0 {
		return nil, fmt.Errorf("cfmleditor.explainCall requires a 0-based line number argument")
	}

	line := int(lineArg)
	filter, _ := argString(args, 2)

	fileURI := uri.URI(docURI)
	file := cfpath.FromURI(docURI)

	content, ok := s.getDocument(fileURI)
	if !ok {
		// The command picker can name a file that is not open.
		data, err := s.FS.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("cfmleditor.explainCall: %w", err)
		}

		content = string(data)
	}

	pr := s.parseForCalls(fileURI, content)
	calls := resolve.CallsOnLine(pr, line, filter)

	var msg string

	if len(calls) == 0 {
		msg = fmt.Sprintf("No call sites on %s:%d", file, line+1)
		if filter != "" {
			msg += fmt.Sprintf(" matching %q", filter)
		}
	} else {
		var b strings.Builder

		s.getResolver().WriteExplanation(&b, file, line, calls, pr, filepath.Dir(file))
		msg = strings.TrimSuffix(b.String(), "\n")
	}

	s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
		Type:    protocol.MessageTypeInfo,
		Message: msg,
	})

	return msg, nil
}

// parseForCalls parses content afresh for its call sites.
//
// The document's cached ParseResult cannot answer this. An edit outside a
// function reparses it shallowly, which drops every call site recorded inside
// one, and an edit inside a function moves lines without moving the calls
// below it. So one keystroke leaves the cache reporting no calls, or calls on
// the wrong line. A fresh parse of the text the editor holds is also what the
// CLI does.
//
// The result is private to the caller and never enters s.parseResults, so it
// needs no document lock (see lockDoc), which is also why the resolver can
// memoise into it freely.
func (s *Server) parseForCalls(fileURI uri.URI, content string) *parser.ParseResult {
	return s.parseContent(fileURI, content)
}

// lineHasCall reports whether a 0-based line of the document holds a call
// site, which is what decides whether the explain code action is offered.
//
// The editor asks for code actions each time the cursor settles, and answering
// needs a full parse, so the lines with calls are memoised against the text
// they came from. The same arrangement as scanDocumentRoutes: keyed on a hash
// of the content, one entry, because the editor asks about the document in
// front of it. The URI and the interpolation switch are in the key as well,
// since the file's extension and the switch both change which calls it has.
func (s *Server) lineHasCall(fileURI uri.URI, content string, line uint32) bool {
	key := string(fileURI) + "\x00" + contentKey(content)
	if s.Features.OutputContextInterpolation {
		key += "i"
	}

	s.callLinesMu.Lock()
	lines, ok := s.callLines, s.callLinesKey == key
	s.callLinesMu.Unlock()

	if !ok {
		lines = nil

		for _, call := range s.parseForCalls(fileURI, content).AllCalls() {
			// AllCalls is sorted by line, so a repeat is always the last one.
			if n := len(lines); n == 0 || lines[n-1] != call.Line {
				lines = append(lines, call.Line)
			}
		}

		s.callLinesMu.Lock()
		s.callLinesKey, s.callLines = key, lines
		s.callLinesMu.Unlock()
	}

	_, found := slices.BinarySearch(lines, line)

	return found
}
