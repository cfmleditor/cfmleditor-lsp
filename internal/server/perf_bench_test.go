package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// These measure request handlers against an index the size of a real
// workspace, which is the axis the handlers themselves are silent about: a
// benchmark on an empty server reports the cost of the document and misses
// anything that walks the workspace once per request.

func benchDoc(funcs int) string {
	var b strings.Builder

	b.WriteString("component extends=\"base.AbstractService\" {\n")

	for i := range funcs {
		fmt.Fprintf(&b, `
	public struct function getThing%d(required numeric id) {
		var result = {};
		var dao = new model.UserDAO();
		var cache = createObject("component", "utils.CacheManager");
		result.id = arguments.id;
		return result;
	}
`, i)
	}

	b.WriteString("}\n")

	return b.String()
}

// benchLoadedServer fills an index with files declaring the same handful of
// method names, which is the shape that stresses the name buckets.
func benchLoadedServer(files, perFile int) *Server {
	s := newTestServer()

	for f := range files {
		u := uri.File(fmt.Sprintf("/ws/pkg%d/File%d.cfc", f%20, f))

		defs := make([]parser.FunctionDef, 0, perFile)
		refs := make([]parser.ComponentRef, 0, perFile)

		for i := range perFile {
			defs = append(defs, parser.FunctionDef{
				Name: fmt.Sprintf("method%d", i), URI: u, Line: uint32(10 + i*5),
			})
			refs = append(refs, parser.ComponentRef{
				Variable: fmt.Sprintf("svc%d", i), Component: "models.User", URI: u, Line: uint32(12 + i*5),
			})
		}

		s.index.IndexFileFromResult(u, defs, refs)
	}

	return s
}

func benchOpen(b *testing.B, s *Server, docURI uri.URI, content string) {
	b.Helper()

	open, err := json.Marshal(protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: docURI, LanguageID: "cfml", Version: 1, Text: content,
		},
	})
	if err != nil {
		b.Fatal(err)
	}

	if _, err := s.handleDidOpen(context.Background(), open); err != nil {
		b.Fatal(err)
	}
}

// workspace/symbol is issued on every keystroke in the symbol picker and
// discards all but a handful of matches, so what it must not do is materialise
// the whole index per request.
func BenchmarkWorkspaceSymbol(b *testing.B) {
	for _, q := range []string{"method3", "zzz"} {
		b.Run("query_"+q, func(b *testing.B) {
			s := benchLoadedServer(5000, 8)

			req, err := json.Marshal(protocol.WorkspaceSymbolParams{Query: q})
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if _, err := s.handleWorkspaceSymbol(context.Background(), req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// What a workspace/symbol response actually costs end to end, and the reason
// the handler-only benchmark above is not that number: on a 5,000-file index
// the handler is 6% of the request and marshalling its answer is the rest. The
// short queries are the point — the picker sends m, me, met on the way to
// anything, and a one-character query matches every definition in the
// workspace. See PERFORMANCE-GAPS.md.
func BenchmarkWorkspaceSymbolWithMarshal(b *testing.B) {
	s := benchLoadedServer(5000, 8)

	for _, q := range []string{"m", "method3", "zzz"} {
		b.Run("query_"+q, func(b *testing.B) {
			req, err := json.Marshal(protocol.WorkspaceSymbolParams{Query: q})
			if err != nil {
				b.Fatal(err)
			}

			res, err := s.handleWorkspaceSymbol(context.Background(), req)
			if err != nil {
				b.Fatal(err)
			}

			out, err := json.Marshal(res)
			if err != nil {
				b.Fatal(err)
			}

			b.Logf("response: %d symbols, %d bytes of JSON", len(res.([]protocol.SymbolInformation)), len(out))

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				r, err := s.handleWorkspaceSymbol(context.Background(), req)
				if err != nil {
					b.Fatal(err)
				}

				if _, err := json.Marshal(r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompletion(b *testing.B) {
	s := benchLoadedServer(5000, 8)
	docURI := uri.File("/ws/open/Doc.cfc")

	benchOpen(b, s, docURI, benchDoc(60))

	req, err := json.Marshal(protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Position:     protocol.Position{Line: 4, Character: 6},
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := s.handleCompletion(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

// A keystroke inside a function body: the path that applies the edit, shifts
// the index and arms the completion-cache rebuild.
func BenchmarkKeystroke(b *testing.B) {
	for _, funcs := range []int{20, 200} {
		b.Run(fmt.Sprintf("funcs%d", funcs), func(b *testing.B) {
			s := benchLoadedServer(5000, 8)
			docURI := uri.File("/ws/open/Doc.cfc")

			benchOpen(b, s, docURI, benchDoc(funcs))

			const editLine = 3

			chg, err := json.Marshal(protocol.DidChangeTextDocumentParams{
				TextDocument: protocol.VersionedTextDocumentIdentifier{
					TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
					Version:                2,
				},
				ContentChanges: []protocol.TextDocumentContentChangeEvent{
					&protocol.TextDocumentContentChangePartial{
						Range: protocol.Range{
							Start: protocol.Position{Line: editLine, Character: 0},
							End:   protocol.Position{Line: editLine, Character: 0},
						},
						Text: "\t\tvar zz = 1;\n",
					},
				},
			})
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if _, err := s.handleDidChange(context.Background(), chg); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// scopesToFuncRanges runs holding s.mu on didOpen and on every
// signature-changing edit, so its cost is paid by every other request in
// flight.
func BenchmarkScopesToFuncRanges(b *testing.B) {
	for _, funcs := range []int{20, 200} {
		b.Run(fmt.Sprintf("funcs%d", funcs), func(b *testing.B) {
			pr := parser.Parse("file:///bench.cfc", benchDoc(funcs))

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				scopesToFuncRanges(pr)
			}
		})
	}
}

// What a completion response actually costs end to end. The handler's result is
// JSON-marshalled onto the wire, so the question for any saving inside the
// handler is how it compares with that.
func BenchmarkCompletionWithMarshal(b *testing.B) {
	s := benchLoadedServer(5000, 8)
	docURI := uri.File("/ws/open/Doc.cfc")

	benchOpen(b, s, docURI, benchDoc(60))

	req, err := json.Marshal(protocol.CompletionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Position:     protocol.Position{Line: 4, Character: 6},
	})
	if err != nil {
		b.Fatal(err)
	}

	res, err := s.handleCompletion(context.Background(), req)
	if err != nil {
		b.Fatal(err)
	}

	out, err := json.Marshal(res)
	if err != nil {
		b.Fatal(err)
	}

	b.Logf("response: %d items, %d bytes of JSON", len(res.(*protocol.CompletionList).Items), len(out))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		r, _ := s.handleCompletion(context.Background(), req) //nolint:errcheck

		if _, err := json.Marshal(r); err != nil {
			b.Fatal(err)
		}
	}
}

// foldingRange walks every named node in the document and every accessor on a
// tree-sitter node is a cgo call, so this is the handler most sensitive to how
// the walk is written rather than to what it computes.
func BenchmarkFoldingRange(b *testing.B) {
	for _, funcs := range []int{20, 60} {
		b.Run(fmt.Sprintf("funcs%d", funcs), func(b *testing.B) {
			s := newTestServer()
			docURI := uri.File("/ws/open/Doc.cfc")

			benchOpen(b, s, docURI, benchDoc(funcs))

			req, err := json.Marshal(protocol.FoldingRangeParams{
				TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			})
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if _, err := s.handleFoldingRange(context.Background(), req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// The handlers that answer from the index rather than from a parse. Grouped so
// a change that moves one of them shows up next to the others.
func BenchmarkIndexBackedHandlers(b *testing.B) {
	cases := []struct {
		name string
		req  func(uri.URI) any
		call func(*Server, []byte) (any, error)
	}{
		{"hover", func(u uri.URI) any {
			return protocol.HoverParams{TextDocumentPositionParams: benchPos(u, 2, 30)}
		}, func(s *Server, r []byte) (any, error) { return s.handleHover(context.Background(), r) }},
		{"definition", func(u uri.URI) any {
			return protocol.DefinitionParams{TextDocumentPositionParams: benchPos(u, 2, 30)}
		}, func(s *Server, r []byte) (any, error) { return s.handleDefinition(context.Background(), r) }},
		{"signatureHelp", func(u uri.URI) any {
			return protocol.SignatureHelpParams{TextDocumentPositionParams: benchPos(u, 4, 20)}
		}, func(s *Server, r []byte) (any, error) { return s.handleSignatureHelp(context.Background(), r) }},
		{"documentHighlight", func(u uri.URI) any {
			return protocol.DocumentHighlightParams{TextDocumentPositionParams: benchPos(u, 4, 8)}
		}, func(s *Server, r []byte) (any, error) { return s.handleDocumentHighlight(context.Background(), r) }},
		{"documentLink", func(u uri.URI) any {
			return protocol.DocumentLinkParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}}
		}, func(s *Server, r []byte) (any, error) { return s.handleDocumentLink(context.Background(), r) }},
		{"documentSymbol", func(u uri.URI) any {
			return protocol.DocumentSymbolParams{TextDocument: protocol.TextDocumentIdentifier{URI: u}}
		}, func(s *Server, r []byte) (any, error) { return s.handleDocumentSymbol(context.Background(), r) }},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			s := benchLoadedServer(5000, 8)
			docURI := uri.File("/ws/open/Doc.cfc")

			benchOpen(b, s, docURI, benchDoc(60))

			req, err := json.Marshal(tc.req(docURI))
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if _, err := tc.call(s, req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchPos(docURI uri.URI, line, char uint32) protocol.TextDocumentPositionParams {
	return protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
		Position:     protocol.Position{Line: line, Character: char},
	}
}
