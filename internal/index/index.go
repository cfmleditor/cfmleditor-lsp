// Package index maintains a searchable index of CFML function definitions.
package index

import (
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfparser"
	"go.lsp.dev/uri"
)

// Index is a concurrency-safe store of function definitions keyed by name.
type Index struct {
	mu       sync.RWMutex
	funcs    map[string][]*cfparser.FunctionDef    // lowercase name -> definitions
	comprefs map[string][]*cfparser.ComponentRef   // lowercase variable -> refs
	thisVars map[uri.URI][]string                  // file URI -> this-scoped var names
}

// New creates an empty Index.
func New() *Index {
	return &Index{
		funcs:    make(map[string][]*cfparser.FunctionDef),
		comprefs: make(map[string][]*cfparser.ComponentRef),
		thisVars: make(map[uri.URI][]string),
	}
}

// Lookup returns all function definitions matching the given name (case-insensitive).
func (idx *Index) Lookup(name string) []*cfparser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.funcs[strings.ToLower(name)]
}

// AllFunctions returns every indexed function definition.
func (idx *Index) AllFunctions() []*cfparser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var all []*cfparser.FunctionDef
	for _, defs := range idx.funcs {
		all = append(all, defs...)
	}
	return all
}

// FunctionsForFile returns all indexed function definitions for a specific file.
func (idx *Index) FunctionsForFile(fileURI uri.URI) []*cfparser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var out []*cfparser.FunctionDef
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
	pr := cfparser.Parse(fileURI, content)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)
	idx.thisVars[fileURI] = pr.ThisVars()

	for i := range pr.Funcs {
		key := strings.ToLower(pr.Funcs[i].Name)
		idx.funcs[key] = append(idx.funcs[key], &pr.Funcs[i])
	}
	for i := range pr.Refs {
		key := strings.ToLower(pr.Refs[i].Variable)
		idx.comprefs[key] = append(idx.comprefs[key], &pr.Refs[i])
	}
}

// IndexFileFromResult updates the index using pre-parsed function defs and refs.
func (idx *Index) IndexFileFromResult(fileURI uri.URI, funcs []cfparser.FunctionDef, refs []cfparser.ComponentRef) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)

	for i := range funcs {
		key := strings.ToLower(funcs[i].Name)
		idx.funcs[key] = append(idx.funcs[key], &funcs[i])
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
	delete(idx.thisVars, fileURI)
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
func (idx *Index) LookupComponentRef(variable string) []*cfparser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.comprefs[strings.ToLower(variable)]
}

// ThisVarsForFile returns the this-scoped variable names for a file.
func (idx *Index) ThisVarsForFile(fileURI uri.URI) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.thisVars[fileURI]
}

// SetThisVars stores this-scoped variable names for a file.
func (idx *Index) SetThisVars(fileURI uri.URI, vars []string) {
	idx.mu.Lock()
	idx.thisVars[fileURI] = vars
	idx.mu.Unlock()
}

// AddRefs appends additional component refs to the index.
func (idx *Index) AddRefs(refs []cfparser.ComponentRef) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for i := range refs {
		key := strings.ToLower(refs[i].Variable)
		idx.comprefs[key] = append(idx.comprefs[key], &refs[i])
	}
}

// LookupComponentRefInFile returns the component ref for a variable in a specific file
// that is closest to (but not after) the given line.
func (idx *Index) LookupComponentRefInFile(variable string, fileURI uri.URI, line uint32) *cfparser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var best *cfparser.ComponentRef
	for _, ref := range idx.comprefs[strings.ToLower(variable)] {
		if ref.URI == fileURI && ref.Line <= line {
			if best == nil || ref.Line > best.Line {
				best = ref
			}
		}
	}
	return best
}
