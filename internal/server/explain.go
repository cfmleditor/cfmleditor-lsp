package server

import (
	"context"
	"fmt"
	"path/filepath"
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

	// It works on the text alone, in a parse of its own; let later messages in.
	releaseReadLoop(ctx)

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

// lineMayHoldCall reports whether a line of the document looks as if it holds
// a call, which is what decides whether the explain code action is offered.
//
// It is a look at the line's text, not a parse. The editor asks for code
// actions every time the cursor settles, on the read goroutine that every
// other message waits behind, and deciding this from a parse meant parsing the
// whole file after every edit: 11ms on a 12,000-line component and 87ms on a
// 65,000-line one, per cursor move. So this looks for a name directly before a
// "(", which is how a call is written in script and in a tag's expression, and
// leaves out the ones that are not calls: control keywords, a function's own
// declaration, and CFML tags whose attribute text starts with a paren.
//
// A line it wrongly accepts, such as a "(" inside a string or comment, costs an
// action that reports "No call sites" when run; the command itself parses, so
// its answer is exact either way.
func lineMayHoldCall(content string, line int) bool {
	start := parser.PositionToOffset(content, line, 0)

	text := content[start:]
	if end := strings.IndexByte(text, '\n'); end >= 0 {
		text = text[:end]
	}

	for i := strings.IndexByte(text, '('); i >= 0; {
		name, before := nameBefore(text[:i])
		if name != "" && !notACall[strings.ToLower(name)] && !strings.EqualFold(before, "function") {
			return true
		}

		next := strings.IndexByte(text[i+1:], '(')
		if next < 0 {
			break
		}

		i += next + 1
	}

	return false
}

// notACall is the words that a "(" can follow without making a call.
var notACall = map[string]bool{
	"if": true, "elseif": true, "for": true, "while": true, "switch": true, "catch": true,
	"function": true, "return": true, "and": true, "or": true, "not": true,
	"cfif": true, "cfelseif": true, "cfreturn": true, "cfset": true, "cfloop": true,
}

// nameBefore returns the identifier that s ends with, ignoring spaces before
// the paren, and the word before that identifier (for "function name(").
func nameBefore(s string) (name, before string) {
	s = strings.TrimRight(s, " \t")

	end := len(s)
	start := end

	for start > 0 && isNameByte(s[start-1]) {
		start--
	}

	if start == end || s[start] >= '0' && s[start] <= '9' {
		return "", ""
	}

	rest := strings.TrimRight(s[:start], " \t")

	wordStart := len(rest)
	for wordStart > 0 && isNameByte(rest[wordStart-1]) {
		wordStart--
	}

	return s[start:end], rest[wordStart:]
}

func isNameByte(b byte) bool {
	return b == '_' || b == '$' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}
