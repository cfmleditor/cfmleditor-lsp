package server

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// unscopedSearchOrder is where CFML looks for a name written without a scope,
// and so where this looks too.
//
// Abridged from the engine's real order, which also takes in thread, cffile and
// a few others this parser does not record. What matters is that local and
// arguments come before variables, and variables before the request scopes: a
// function with `var total` and a page with `url.total` must resolve the bare
// `total` inside that function to its own, or go-to-definition sends the reader
// somewhere the value never came from.
var unscopedSearchOrder = []parser.Scope{
	parser.ScopeLocal,
	parser.ScopeArguments,
	parser.ScopeVariables,
	parser.ScopeThis,
	parser.ScopeURL,
	parser.ScopeForm,
	parser.ScopeCookie,
	parser.ScopeCGI,
	parser.ScopeClient,
}

// crossFileScopes name a scope whose declarations usually live in another file.
// `application.x` is written in Application.cfc's onApplicationStart and read
// everywhere; `server.x` is written in Server.cfc. A definition search that
// stopped at the current file would answer nothing for exactly the scopes whose
// whole point is that they are shared.
var crossFileScopes = map[parser.Scope][]string{
	parser.ScopeApplication: {"Application.cfc", "Application.cfm"},
	parser.ScopeRequest:     {"Application.cfc", "Application.cfm"},
	parser.ScopeSession:     {"Application.cfc", "Application.cfm"},
	parser.ScopeServer:      {"Server.cfc"},
}

// resolveVariableDef answers a cursor sitting on a variable rather than a
// function or a component.
//
// It runs before the function-name lookup, and declines rather than guesses:
// everything it cannot place falls through to the paths that were there before,
// so a name it does not recognise costs one scan of the document's declarations
// and nothing else.
func (s *Server) resolveVariableDef(content string, line, char int, word string, docURI uri.URI) []protocol.Location {
	if !s.Features.VariableDefinitions || word == "" || isCallAt(content, line, char) {
		return nil
	}

	qualifier := parser.QualifierBeforeWord(content, line, char)

	scope, scoped := parser.Scope(0), false
	if qualifier != "" {
		scope, scoped = parser.ScopeForPrefix(qualifier)
		if !scoped {
			// A qualifier that is not a scope is a receiver — `widget.size` is a
			// property of something, not a variable, and the component paths
			// handle it.
			return nil
		}
	}

	vars := s.documentVars(docURI, content)

	if loc := matchVarInFile(vars, word, line, scoped, scope, docURI, content); loc != nil {
		return []protocol.Location{*loc}
	}

	// Only the shared scopes are worth another file: they are the ones declared
	// somewhere other than where they are read.
	if !scoped {
		return nil
	}

	// This is the one path here that reads files it was not given, and it does
	// so once per workspace root. A workspace with dozens of roots — an editor
	// sends every folder, not the handful the config lists — turns one
	// go-to-definition into dozens of reads and parses, so it is worth being
	// able to see on its own.
	start := time.Now()
	locs := s.crossFileVarDef(scope, word, docURI)

	if d := time.Since(start); d > 50*time.Millisecond {
		s.log.Warn("slow cross-file variable lookup",
			cflog.String("word", word),
			cflog.String("scope", parser.ScopeName(scope)),
			cflog.Duration("dur", d),
			cflog.Int("roots", len(s.searchRoots())))
	}

	return locs
}

// matchVarInFile finds the declaration in this document, if it is here.
//
// One pass over the declarations, not one per scope. An unscoped name has nine
// scopes to try, and asking each in turn walked every declaration in the file
// nine times — on a 64,000-line component that is 10,497 of them, and the answer
// is usually in the first scope or nowhere. The pass collects the best candidate
// for every scope at once; the search order then only decides which of them to
// return.
func matchVarInFile(vars []parser.VarDef, word string, line int, scoped bool, scope parser.Scope, docURI uri.URI, content string) *protocol.Location {
	// A scope-qualified name has exactly one scope to look in, so it takes the
	// single-scope walk: the per-scope version would allocate two slices and
	// prepare a lookup table to answer a question about one of them.
	if scoped {
		if best := bestVarDecl(vars, word, scope, line); best != nil {
			return varLocation(docURI, content, *best, word)
		}

		return nil
	}

	best := bestVarDeclPerScope(vars, word, unscopedSearchOrder, line)
	for i := range unscopedSearchOrder {
		if best[i] != nil {
			return varLocation(docURI, content, *best[i], word)
		}
	}

	return nil
}

// lowerASCIIByte folds one ASCII byte, for the guard below.
func lowerASCIIByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}

	return b
}

// bestVarDeclPerScope returns the declaration in force at line for each scope in
// order, indexed the same way as order.
func bestVarDeclPerScope(vars []parser.VarDef, name string, order []parser.Scope, line int) []*parser.VarDef {
	if name == "" {
		return make([]*parser.VarDef, len(order))
	}

	inFunc := make([]*parser.VarDef, len(order))
	atFile := make([]*parser.VarDef, len(order))

	// Scope values are small and dense, so a lookup from scope to its position in
	// the order beats searching the order per declaration.
	slot := [64]int{}
	for i := range slot {
		slot[i] = -1
	}

	for i, sc := range order {
		if int(sc) < len(slot) {
			slot[sc] = i
		}
	}

	// The name is checked before the scope, and cheaply before that.
	//
	// A single-scope walk rejects on the scope first, which is very selective and
	// leaves EqualFold running on a handful of declarations. This walk admits
	// every scope, so without a cheap guard EqualFold runs on all of them — and a
	// function call per declaration is what made the one pass cost more than the
	// nine it replaced. Length and a folded first byte reject almost everything
	// inline.
	want := lowerASCIIByte(name[0])

	for i := range vars {
		v := &vars[i]

		if len(v.Name) != len(name) || lowerASCIIByte(v.Name[0]) != want {
			continue
		}

		if int(v.Scope) >= len(slot) {
			continue
		}

		at := slot[v.Scope]
		if at < 0 || !strings.EqualFold(v.Name, name) {
			continue
		}

		enclosing := v.FuncStart >= 0 && line >= v.FuncStart && line <= v.FuncEnd

		switch {
		case enclosing:
			inFunc[at] = nearerTo(inFunc[at], v, line)
		case v.FuncStart < 0:
			atFile[at] = nearerTo(atFile[at], v, line)
		}
	}

	// A declaration inside the enclosing function outranks one outside it, per
	// scope, which is what bestVarDecl did one scope at a time.
	for i := range inFunc {
		if inFunc[i] == nil {
			inFunc[i] = atFile[i]
		}
	}

	return inFunc
}

// bestVarDecl picks the declaration of name in the given scope that is actually
// in scope at the cursor.
//
// Used for a scope-qualified name, where there is only one scope to consider.
// An unscoped name goes through bestVarDeclPerScope, which answers for all nine
// in one pass; this states the rule plainly for one, and is what that version is
// checked against.
//
// A declaration inside the enclosing function wins over one outside it, because
// that is what the name means there. Among several, the nearest at or before the
// cursor wins, so a variable reassigned down a page resolves to the assignment
// in force rather than to whichever the parser recorded first — the same rule
// CanResolveCall uses for component refs, and for the same reason.
func bestVarDecl(vars []parser.VarDef, name string, scope parser.Scope, line int) *parser.VarDef {
	var (
		inFunc *parser.VarDef
		atFile *parser.VarDef
	)

	for i := range vars {
		v := &vars[i]
		if v.Scope != scope || !strings.EqualFold(v.Name, name) {
			continue
		}

		enclosing := v.FuncStart >= 0 && line >= v.FuncStart && line <= v.FuncEnd

		switch {
		case enclosing:
			inFunc = nearerTo(inFunc, v, line)
		case v.FuncStart < 0:
			atFile = nearerTo(atFile, v, line)
		}
	}

	if inFunc != nil {
		return inFunc
	}

	return atFile
}

// nearerTo keeps whichever declaration is the one in force at line: the latest
// at or before it, or failing that the earliest after it, so a forward reference
// still lands somewhere rather than nowhere.
func nearerTo(current, candidate *parser.VarDef, line int) *parser.VarDef {
	if current == nil {
		return candidate
	}

	curBefore := int(current.Line) <= line
	candBefore := int(candidate.Line) <= line

	switch {
	case curBefore && candBefore:
		if candidate.Line > current.Line {
			return candidate
		}
	case !curBefore && !candBefore:
		if candidate.Line < current.Line {
			return candidate
		}
	case candBefore:
		return candidate
	}

	return current
}

// crossFileVarDef looks for a shared-scope declaration in the files that
// conventionally hold them.
//
// Only reached when the current file does not declare the name, and only for the
// four shared scopes, so an ordinary variable lookup never reads another file.
// Application.cfc is found by walking up from the document, which is how the
// engine finds it; Server.cfc has no such rule, so the workspace roots are the
// only place to look.
func (s *Server) crossFileVarDef(scope parser.Scope, word string, docURI uri.URI) []protocol.Location {
	names, ok := crossFileScopes[scope]
	if !ok {
		return nil
	}

	for _, dir := range s.varSearchDirs(docURI, scope) {
		for _, name := range names {
			path := filepath.Join(dir, name)

			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			content := string(data)
			targetURI := cfpath.ToURI(path)

			// Line -1: nothing in the target file encloses the cursor, so every
			// declaration there is file-level as far as this search is concerned.
			if best := bestVarDeclAnywhere(parser.ParseVars(content), word, scope); best != nil {
				if loc := varLocation(targetURI, content, *best, word); loc != nil {
					return []protocol.Location{*loc}
				}
			}
		}
	}

	return nil
}

// bestVarDeclAnywhere is bestVarDecl for another file, where "the enclosing
// function" means nothing — an application variable is set inside
// onApplicationStart, and the reader is not in it.
func bestVarDeclAnywhere(vars []parser.VarDef, name string, scope parser.Scope) *parser.VarDef {
	for i := range vars {
		if vars[i].Scope == scope && strings.EqualFold(vars[i].Name, name) {
			return &vars[i]
		}
	}

	return nil
}

// varSearchDirs is where to look for the file a shared scope is declared in,
// nearest first.
func (s *Server) varSearchDirs(docURI uri.URI, scope parser.Scope) []string {
	baseDir := filepath.Dir(docURI.Path())

	var dirs []string

	// Application.cfc governs from the directory it sits in downwards, so the
	// walk up from the document is the right search and the resolver already
	// caches it.
	if scope != parser.ScopeServer {
		if appDir := s.getResolver().FindApplicationRoot(baseDir); appDir != "" {
			dirs = append(dirs, appDir)
		}
	}

	return append(dirs, s.searchRoots()...)
}

// varLocation points at the identifier on its declaring line rather than at the
// line, so the editor reveals the name.
//
// One line, not all of them. This used to split the whole document into a
// []string to read a single row of it: a megabyte allocated per lookup on a
// 64,000-line component, and the only thing wanted from it was one line's text.
func varLocation(docURI uri.URI, content string, v parser.VarDef, name string) *protocol.Location {
	rng, _ := rangeInLine(parser.LineTextAt(content, int(v.Line)), v.Line, name)

	return &protocol.Location{URI: docURI, Range: rng}
}

// documentVars returns the declarations in an open document, through the cached
// parse result when there is one. A definition request on a file the server has
// not parsed is rare but not impossible, and re-parsing beats declining.
func (s *Server) documentVars(docURI uri.URI, content string) []parser.VarDef {
	s.mu.RLock()
	pr := s.parseResults[docURI]
	s.mu.RUnlock()

	if pr != nil {
		return pr.AllVars()
	}

	return parser.ParseVars(content)
}

// isCallAt reports whether the word at the cursor is immediately followed by an
// opening paren, which makes it a call rather than a variable. Without this the
// variable branch would claim every function name that happens to share a name
// with a local.
func isCallAt(content string, line, char int) bool {
	text := parser.LineTextAt(content, line)
	if text == "" {
		return false
	}

	end := min(char, len(text))
	for end < len(text) && parser.IsWordChar(text[end]) {
		end++
	}

	for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
		end++
	}

	return end < len(text) && text[end] == '('
}
