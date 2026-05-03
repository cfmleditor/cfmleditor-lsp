package index

import (
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/cfml"
	"go.lsp.dev/uri"
)

type Index struct {
	mu    sync.RWMutex
	funcs map[string][]cfml.FunctionDef // lowercase name -> definitions
}

func New() *Index {
	return &Index{funcs: make(map[string][]cfml.FunctionDef)}
}

func (idx *Index) Lookup(name string) []cfml.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.funcs[strings.ToLower(name)]
}

func (idx *Index) AllFunctions() []cfml.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var all []cfml.FunctionDef
	for _, defs := range idx.funcs {
		all = append(all, defs...)
	}
	return all
}

func (idx *Index) IndexFile(fileURI uri.URI, content string) {
	defs := cfml.ParseFunctionDefs(fileURI, content)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)

	for _, d := range defs {
		key := strings.ToLower(d.Name)
		idx.funcs[key] = append(idx.funcs[key], d)
	}
}

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
}
