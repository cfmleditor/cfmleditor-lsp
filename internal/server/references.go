package server

import (
	"context"
	"strings"
	"time"

	json "github.com/go-json-experiment/json"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/refs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// handleReferences answers textDocument/references.
//
// Opt-in: it is off unless `"references": {"enabled": true}` is set, and the
// capability is advertised only when it is on, so a client that has not opted
// in never offers the command and never sends the request. What is being tried
// out behind the flag is the cost — answering one request walks and parses
// every CFML file under the search roots, the same scan `cfmleditor.findRefs`
// and the `refs` CLI do, and how that feels on a large workspace is the open
// question.
//
// Two kinds of thing can be under the cursor and they are searched differently:
// a component dot-path (`new models.UserDAO()`, an `extends`, a `<cfinvoke
// component>`) is matched against parsed component refs, and anything else is
// treated as a function name and matched against parsed call sites.
func (s *Server) handleReferences(_ context.Context, rawParams []byte) (any, error) {
	// Defensive: a client that sends the request anyway gets an empty answer
	// rather than a workspace scan it was never offered.
	if !s.References {
		return nil, nil
	}

	var params protocol.ReferenceParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	docURI := params.TextDocument.URI

	defer s.lockDoc(docURI)()

	content, ok := s.getDocument(docURI)
	if !ok {
		return nil, nil
	}

	var (
		line  = int(params.Position.Line)
		char  = int(params.Position.Character)
		start = time.Now()
	)

	if comp := parser.ComponentPathAtCursor(content, line, char); comp != "" {
		locs := s.componentReferences(comp)
		s.log.Debug("references: component", cflog.String("component", comp), cflog.Int("results", len(locs)), cflog.Any("dur", time.Since(start)))

		return locs, nil
	}

	word := parser.WordAtPosition(content, line, char)
	if word == "" {
		return nil, nil
	}

	locs := s.functionReferences(word, content, docURI, line, char, params.Context.IncludeDeclaration)
	s.log.Debug("references: function", cflog.String("word", word), cflog.Int("results", len(locs)), cflog.Any("dur", time.Since(start)))

	return locs, nil
}

// openDocFS serves the editor's copy of any file it has open, falling through
// to the real filesystem for everything else.
//
// refs.Find opens files itself, from disk, and the line numbers it reports come
// from what it read. Resolving columns from the editor's buffer while the lines
// came from the saved text mixes two different versions of the file — and the
// file being edited is the likeliest one to run a reference search on.
type openDocFS struct {
	vfs.FS

	srv *Server
}

func (o openDocFS) ReadFile(path string) ([]byte, error) {
	if content, ok := o.srv.getDocument(cfpath.ToURI(path)); ok {
		return []byte(content), nil
	}

	return o.FS.ReadFile(path)
}

func (s *Server) refsFS() vfs.FS {
	return openDocFS{FS: s.FS, srv: s}
}

// functionReferences finds the call sites of the function under the cursor.
func (s *Server) functionReferences(word, content string, docURI uri.URI, line, char int, includeDecl bool) []protocol.Location {
	decl := s.declarationOf(word, content, docURI, line, char)

	// Which file declares the function decides two things inside refs.Find: a
	// qualified call matches only when its receiver resolves to that file, and
	// a bare call matches only when it sits in that file. Taking it from the
	// declaration rather than from the document the request arrived on is what
	// lets this be asked from a call site, which is where someone wondering
	// "who else calls this" usually has their cursor. With no declaration
	// found, the requesting document is the best guess left.
	sourceFile := cfpath.FromURI(string(docURI))
	if decl != nil {
		sourceFile = cfpath.FromURI(string(decl.URI))
	}

	r := s.getResolver()
	entries := refs.Find(s.refsFS(), s.searchRoots(), refs.Options{
		FuncName:          word,
		Resolvers:         s.cfResolvers(),
		PropertyResolvers: s.cfPropertyResolvers(),
		SourceFile:        sourceFile,
		VerifyCall: func(component, fn, fileDir string) bool {
			return r.HasFunction(component, fn, fileDir)
		},
		VerifyTarget: func(component, fileDir, source string) bool {
			return cfpath.SamePath(r.ComponentPath(component, fileDir), source)
		},
	})

	// A call site always writes the name it calls, so a line the name cannot be
	// found on is a wrapped call rather than a wrong entry: keep it, pointing
	// at the line.
	locs := s.entryLocations(entries, word, false)

	if includeDecl && decl != nil {
		d := *decl
		d.Range = nameRange(s.fileLines(cfpath.FromURI(string(d.URI))), d.Range.Start.Line, word)

		locs = dedupeLocations(append([]protocol.Location{d}, locs...))
	}

	return locs
}

// componentReferences finds the places a component dot-path is referred to.
func (s *Server) componentReferences(component string) []protocol.Location {
	entries := refs.Find(s.refsFS(), s.searchRoots(), refs.Options{
		Component:         component,
		Resolvers:         s.cfResolvers(),
		PropertyResolvers: s.cfPropertyResolvers(),
	})

	// The line holds the whole dot-path; its last segment is the part worth
	// highlighting, and the part a mapping cannot have rewritten.
	name := component
	if at := strings.LastIndex(name, "."); at >= 0 {
		name = name[at+1:]
	}

	// Unlike a call site, a component ref is not always written where it is
	// recorded. The parser records the variables a reference flows into, and a
	// componentResolver can establish one from an expression that never spells
	// the resolved path — `svc = getPageTools()` is a ref to
	// packages.tass.pagetools, and "pagetools" appears nowhere on the line. So
	// the receiving variable is the fallback anchor: it is on the line in every
	// one of these shapes, which keeps resolver-established references (the
	// majority, in a resolver-configured project) out of the bin while still
	// never highlighting a whole line the reader has to search by eye.
	return s.entryLocations(entries, name, true)
}

// declarationOf locates the function the cursor is on, in the same order of
// preference go-to-definition uses, so the two agree about which of several
// same-named functions is meant.
func (s *Server) declarationOf(word, content string, docURI uri.URI, line, char int) *protocol.Location {
	if qualifier := parser.QualifierBeforeWord(content, line, char); qualifier != "" {
		if def := s.resolveUserFunc(qualifier, word, docURI, uint32(line)); def != nil {
			return defLocation(def)
		}
	}

	defs := s.index.Lookup(word)
	for _, d := range defs {
		if d.URI == docURI {
			return defLocation(d)
		}
	}

	if loc := s.resolveSuper(word, docURI); loc != nil {
		return loc
	}

	// Only when there is exactly one candidate left. Several same-named
	// functions across the workspace give no basis for choosing, and choosing
	// wrong would scope the whole search to the wrong component.
	if len(defs) == 1 {
		return defLocation(defs[0])
	}

	return nil
}

func defLocation(def *parser.FunctionDef) *protocol.Location {
	return &protocol.Location{
		URI:   def.URI,
		Range: protocol.Range{Start: protocol.Position{Line: def.Line}, End: protocol.Position{Line: def.Line}},
	}
}

// searchRoots is where a workspace-wide search looks: the folders from config
// when there are any, and otherwise the roots the editor opened. Without the
// fallback a session with no .cfmleditor.json searches nowhere and reports no
// references, which is indistinguishable from there being none.
func (s *Server) searchRoots() []string {
	if len(s.WorkspaceFolders) > 0 {
		return s.WorkspaceFolders
	}

	return s.workspaceRoots
}

// entryLocations turns reference entries into LSP locations.
//
// refs.Entry carries a line number and no column — nothing that consumed it
// before needed one — so the column is recovered by finding name on the line,
// reading each file once for however many entries it holds.
//
// useVariable says what to do when the name is not on the line: try the entry's
// variable, and drop the entry if that is not there either. Off, the line
// itself is the range, which is right for a call site (it always writes the
// name it calls, so a miss means a call wrapped across lines) and wrong for a
// component ref (which may be recorded on a line that names neither).
func (s *Server) entryLocations(entries []refs.Entry, name string, useVariable bool) []protocol.Location {
	if len(entries) == 0 {
		return nil
	}

	lines := make(map[string][]string, 8)
	locs := make([]protocol.Location, 0, len(entries))

	for _, e := range entries {
		text, ok := lines[e.File]
		if !ok {
			text = s.fileLines(e.File)
			lines[e.File] = text
		}

		rng, exact := entryRange(text, e.Line, name)

		if !exact && useVariable {
			if rng, exact = entryRange(text, e.Line, e.Variable); !exact {
				continue
			}
		}

		locs = append(locs, protocol.Location{URI: cfpath.ToURI(e.File), Range: rng})
	}

	return dedupeLocations(locs)
}

// fileLines reads a file for column lookup, through the same filesystem the
// scan used, so the column and the line number it belongs to always come from
// one version of the file.
func (s *Server) fileLines(path string) []string {
	data, err := s.refsFS().ReadFile(path)
	if err != nil {
		return nil
	}

	return strings.Split(string(data), "\n")
}

// nameRange spans name where it appears on the given line, so the client
// highlights the identifier rather than the whole line. A line the name cannot
// be found on — the reference is on a continuation line, or the file has
// changed since the scan — falls back to the whole line, which is still a
// usable place to jump to.
func nameRange(lines []string, line uint32, name string) protocol.Range {
	rng, _ := entryRange(lines, line, name)

	return rng
}

// entryRange is nameRange plus whether the name was actually found, which is
// what lets a caller drop an entry rather than point at a whole line.
func entryRange(lines []string, line uint32, name string) (protocol.Range, bool) {
	text := ""
	if int(line) < len(lines) {
		text = strings.TrimSuffix(lines[line], "\r")
	}

	if start, end, ok := identSpan(text, name); ok {
		return protocol.Range{
			Start: protocol.Position{Line: line, Character: utf16Len(text[:start])},
			End:   protocol.Position{Line: line, Character: utf16Len(text[:end])},
		}, true
	}

	return protocol.Range{
		Start: protocol.Position{Line: line},
		End:   protocol.Position{Line: line, Character: utf16Len(text)},
	}, false
}

// identSpan finds name in text as a whole identifier, case-insensitively
// because CFML is. The neighbour check is what keeps "GetData" from matching
// inside "GetDataSet" and reporting a column in the wrong call.
func identSpan(text, name string) (start, end int, ok bool) {
	if name == "" {
		return 0, 0, false
	}

	for i := 0; i+len(name) <= len(text); i++ {
		// EqualFold on equal-length byte slices, rather than lowercasing the
		// line first: folding can change a string's length (U+0130 lowercases
		// to two runes), which would desynchronise every offset after it.
		if !strings.EqualFold(text[i:i+len(name)], name) {
			continue
		}

		if isIdentByte(byteAt(text, i-1)) || isIdentByte(byteAt(text, i+len(name))) {
			continue
		}

		return i, i + len(name), true
	}

	return 0, 0, false
}

func byteAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}

	return s[i]
}

// isIdentByte reports whether b can be part of a CFML identifier. "_" and "$"
// are identifier characters to the parser's scanner, and any byte above ASCII
// is part of some wider word rather than a boundary.
func isIdentByte(b byte) bool {
	return b >= 0x80 ||
		b >= 'a' && b <= 'z' ||
		b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' ||
		b == '_' || b == '$'
}

// utf16Len is the length of s in UTF-16 code units, which is what an LSP
// character offset counts unless the client negotiated otherwise. Counting
// bytes instead shifts the highlight on any line with a non-ASCII character
// before the match — a comment in prose, a name with an accent.
func utf16Len(s string) uint32 {
	var n uint32

	for _, r := range s {
		n++

		if r > 0xFFFF {
			n++
		}
	}

	return n
}

// dedupeLocations drops repeats, keeping first-seen order. Two call sites on
// one line collapse to the same location once the column is the identifier's
// rather than the call's, and the declaration can itself be among the entries
// when a function calls itself.
func dedupeLocations(locs []protocol.Location) []protocol.Location {
	seen := make(map[protocol.Location]struct{}, len(locs))
	out := locs[:0]

	for _, l := range locs {
		if _, dup := seen[l]; dup {
			continue
		}

		seen[l] = struct{}{}

		out = append(out, l)
	}

	return out
}
