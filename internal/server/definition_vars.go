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
func matchVarInFile(vars []parser.VarDef, word string, line int, scoped bool, scope parser.Scope, docURI uri.URI, content string) *protocol.Location {
	order := unscopedSearchOrder
	if scoped {
		order = []parser.Scope{scope}
	}

	for _, want := range order {
		if best := bestVarDecl(vars, word, want, line); best != nil {
			return varLocation(docURI, content, *best, word)
		}
	}

	return nil
}

// bestVarDecl picks the declaration of name in the given scope that is actually
// in scope at the cursor.
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
func varLocation(docURI uri.URI, content string, v parser.VarDef, name string) *protocol.Location {
	lines := strings.Split(content, "\n")

	rng, _ := entryRange(lines, v.Line, name)

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
