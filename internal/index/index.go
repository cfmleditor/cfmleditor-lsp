// Package index maintains a searchable index of CFML function definitions.
package index

import (
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// Index is a concurrency-safe store of function definitions keyed by name.
type Index struct {
	mu       sync.RWMutex
	funcs    map[string][]*parser.FunctionDef    // lowercase name -> definitions
	comprefs map[string][]*parser.ComponentRef   // lowercase variable -> refs
}

// New creates an empty Index.
func New() *Index {
	return &Index{
		funcs:    make(map[string][]*parser.FunctionDef),
		comprefs: make(map[string][]*parser.ComponentRef),
	}
}

// Lookup returns all function definitions matching the given name (case-insensitive).
func (idx *Index) Lookup(name string) []*parser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.funcs[strings.ToLower(name)]
}

// AllFunctions returns every indexed function definition.
func (idx *Index) AllFunctions() []*parser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var all []*parser.FunctionDef
	for _, defs := range idx.funcs {
		all = append(all, defs...)
	}
	return all
}

// FunctionsForFile returns all indexed function definitions for a specific file.
func (idx *Index) FunctionsForFile(fileURI uri.URI) []*parser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var out []*parser.FunctionDef
	for _, defs := range idx.funcs {
		for _, d := range defs {
			if d.URI == fileURI {
				out = append(out, d)
			}
		}
	}
	return out
}

// ShiftLines adjusts line numbers for all entries in a file where Line > afterLine.
func (idx *Index) ShiftLines(fileURI uri.URI, afterLine int, delta int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for _, defs := range idx.funcs {
		for _, d := range defs {
			if d.URI == fileURI && int(d.Line) > afterLine {
				d.Line = uint32(int(d.Line) + delta)
			}
		}
	}
	for _, refs := range idx.comprefs {
		for _, r := range refs {
			if r.URI == fileURI && int(r.Line) > afterLine {
				r.Line = uint32(int(r.Line) + delta)
			}
		}
	}
}

// IndexFile parses the given CFC content and updates the index for that file URI.
func (idx *Index) IndexFile(fileURI uri.URI, content string) {
	defs := parser.ParseFunctionDefs(fileURI, content)
	refs := parser.ParseComponentRefs(fileURI, content)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)

	for i := range defs {
		key := strings.ToLower(defs[i].Name)
		idx.funcs[key] = append(idx.funcs[key], &defs[i])
	}
	for i := range refs {
		key := strings.ToLower(refs[i].Variable)
		idx.comprefs[key] = append(idx.comprefs[key], &refs[i])
	}
}

// RemoveFilesUnder removes all indexed entries whose URI starts with prefix.
func (idx *Index) RemoveFilesUnder(prefix string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for key, entries := range idx.funcs {
		filtered := entries[:0]
		for _, e := range entries {
			if !strings.HasPrefix(string(e.URI), prefix) {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(idx.funcs, key)
		} else {
			idx.funcs[key] = filtered
		}
	}
	for key, entries := range idx.comprefs {
		filtered := entries[:0]
		for _, e := range entries {
			if !strings.HasPrefix(string(e.URI), prefix) {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(idx.comprefs, key)
		} else {
			idx.comprefs[key] = filtered
		}
	}
}

func (idx *Index) removeFileEntries(fileURI uri.URI) {
	for key, entries := range idx.funcs {
		filtered := entries[:0]
		for _, e := range entries {
			if e.URI != fileURI {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(idx.funcs, key)
		} else {
			idx.funcs[key] = filtered
		}
	}
	for key, entries := range idx.comprefs {
		filtered := entries[:0]
		for _, e := range entries {
			if e.URI != fileURI {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			delete(idx.comprefs, key)
		} else {
			idx.comprefs[key] = filtered
		}
	}
}

// LookupComponentRef returns component references for the given variable name.
func (idx *Index) LookupComponentRef(variable string) []*parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.comprefs[strings.ToLower(variable)]
}

// LookupComponentRefInFile returns the component ref for a variable in a specific file
// that is closest to (but not after) the given line.
func (idx *Index) LookupComponentRefInFile(variable string, fileURI uri.URI, line uint32) *parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var best *parser.ComponentRef
	for _, ref := range idx.comprefs[strings.ToLower(variable)] {
		if ref.URI == fileURI && ref.Line <= line {
			if best == nil || ref.Line > best.Line {
				best = ref
			}
		}
	}
	return best
}
