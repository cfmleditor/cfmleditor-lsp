package server

import (
	"cmp"
	"container/heap"
	"context"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

	"github.com/cfmleditor/clif/internal/frameworkapi"

	"github.com/cfmleditor/clif/internal/parser"
	"go.lsp.dev/protocol"
)

// The symbol picker sends each prefix while the user types. Keep those broad,
// transient one- and two-character responses bounded; once the query is
// specific enough, return the complete match set again.
const workspaceSymbolShortQueryLimit = 1000

func (s *Server) handleDocumentSymbol(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.DocumentSymbolParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	defer s.lockDoc(docURI)()

	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	var defs []parser.FunctionDef
	if pr != nil {
		defs = pr.Funcs
	} else {
		content, ok := s.getDocument(docURI)
		if !ok {
			return nil, nil
		}

		defs = parser.ParseFunctionDefs(docURI, content)
	}

	symbols := make([]protocol.DocumentSymbol, 0, len(defs))

	for i := range defs {
		d := &defs[i]

		r := protocol.Range{
			Start: protocol.Position{Line: d.Line, Character: 0},
			End:   protocol.Position{Line: d.Line, Character: 0},
		}
		symbols = append(symbols, protocol.DocumentSymbol{
			Name:           d.Name,
			Kind:           protocol.SymbolKindFunction,
			Range:          r,
			SelectionRange: r,
		})
	}

	return symbols, nil
}

func (s *Server) handleWorkspaceSymbol(_ context.Context, rawParams []byte) (any, error) {
	var params protocol.WorkspaceSymbolParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	query := strings.ToLower(params.Query)

	// Filtered inside the index, which tests the query once per distinct name
	// rather than once per definition, and never builds the discarded
	// remainder. AllFunctions here cost 3.1MB per keystroke on a
	// 40,000-definition workspace to return a few dozen symbols.
	defs := s.index.FunctionsMatching(func(name string) bool {
		return query == "" || containsFoldStr(name, query)
	})
	defs = browsableWorkspaceSymbols(defs)

	if len(query) < 3 && len(defs) > workspaceSymbolShortQueryLimit {
		defs = bestWorkspaceSymbols(defs, query, workspaceSymbolShortQueryLimit)
	}

	symbols := make([]protocol.SymbolInformation, 0, len(defs))

	for _, d := range defs {
		symbols = append(symbols, protocol.SymbolInformation{
			Name: d.Name,
			Kind: protocol.SymbolKindFunction,
			Location: protocol.Location{
				URI: d.URI,
				Range: protocol.Range{
					Start: protocol.Position{Line: d.Line, Character: 0},
					End:   protocol.Position{Line: d.Line, Character: 0},
				},
			},
		})
	}

	return symbols, nil
}

func compareWorkspaceSymbols(a, b *parser.FunctionDef, query string) int {
	if d := cmp.Compare(symbolMatchRank(a.Name, query), symbolMatchRank(b.Name, query)); d != 0 {
		return d
	}

	if d := strings.Compare(a.Name, b.Name); d != 0 {
		return d
	}

	if d := cmp.Compare(a.URI, b.URI); d != 0 {
		return d
	}

	return cmp.Compare(a.Line, b.Line)
}

type workspaceSymbolHeap struct {
	defs  []*parser.FunctionDef
	query string
}

func (h *workspaceSymbolHeap) Len() int { return len(h.defs) }
func (h *workspaceSymbolHeap) Less(i, j int) bool {
	return compareWorkspaceSymbols(h.defs[i], h.defs[j], h.query) > 0
}
func (h *workspaceSymbolHeap) Swap(i, j int) { h.defs[i], h.defs[j] = h.defs[j], h.defs[i] }
func (h *workspaceSymbolHeap) Push(value any) {
	def, ok := value.(*parser.FunctionDef)
	if !ok {
		panic(fmt.Sprintf("workspace symbol heap received %T", value))
	}

	h.defs = append(h.defs, def)
}

func (h *workspaceSymbolHeap) Pop() any {
	last := len(h.defs) - 1
	value := h.defs[last]
	h.defs = h.defs[:last]

	return value
}

func bestWorkspaceSymbols(defs []*parser.FunctionDef, query string, limit int) []*parser.FunctionDef {
	h := &workspaceSymbolHeap{defs: append([]*parser.FunctionDef(nil), defs[:limit]...), query: query}
	heap.Init(h)

	for _, d := range defs[limit:] {
		if compareWorkspaceSymbols(d, h.defs[0], query) < 0 {
			h.defs[0] = d
			heap.Fix(h, 0)
		}
	}

	slices.SortFunc(h.defs, func(a, b *parser.FunctionDef) int {
		return compareWorkspaceSymbols(a, b, query)
	})

	return h.defs
}

func browsableWorkspaceSymbols(defs []*parser.FunctionDef) []*parser.FunctionDef {
	out := defs[:0]

	for _, d := range defs {
		// A framework's bundled API is looked up, not browsed: it is in the
		// index only because a call reached it, and it has nowhere to open.
		if !frameworkapi.IsStubURI(string(d.URI)) {
			out = append(out, d)
		}
	}

	return out
}

func symbolMatchRank(name, query string) int {
	switch {
	case strings.EqualFold(name, query):
		return 0
	case len(name) >= len(query) && strings.EqualFold(name[:len(query)], query):
		return 1
	default:
		return 2
	}
}

// containsFoldStr reports whether s contains substr (case-insensitive, ASCII).
// substr must already be lowercase.
// lowerASCII lowercases an ASCII letter and leaves every other byte alone.
func lowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}

	return c
}

func containsFoldStr(s, substr string) bool {
	n := len(substr)
	if n == 0 {
		return true
	}

	end := len(s) - n
	for i := 0; i <= end; i++ {
		match := true

		for j := range n {
			// Both sides go through the same fold, and the fold only touches
			// A-Z. The old code folded the haystack with |0x20 and compared
			// against a raw needle byte, so anything outside a-z never matched
			// its own fold: '_' is 0x5F and 0x5F|0x20 is 0x7F, which meant any
			// workspace-symbol query containing an underscore silently returned
			// nothing. Folding both sides with |0x20 would fix that but make
			// '@' match '`' and '[' match '{', so the fold is explicit.
			if lowerASCII(s[i+j]) != lowerASCII(substr[j]) {
				match = false

				break
			}
		}

		if match {
			return true
		}
	}

	return false
}
