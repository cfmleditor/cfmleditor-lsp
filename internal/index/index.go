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
	mu        sync.RWMutex
	funcs     map[string][]*parser.FunctionDef             // lowercase name -> definitions
	fileFuncs map[string][]*parser.FunctionDef             // lowercase URI -> definitions in that file
	comprefs  map[string][]*parser.ComponentRef            // lowercase variable -> refs
	fileRefs  map[string][]*parser.ComponentRef            // lowercase URI -> refs in that file
	thisVars  map[string][]string                          // lowercase URI -> this-scoped var names
	scopeRefs map[string]map[string][]*parser.ComponentRef // lowercase URI -> function scope key -> refs
	beans     map[string]string                            // lowercase bean name -> dot-path
	entities  map[string]uri.URI                           // lowercase entity name -> file URI
}

// New creates an empty Index.
func New() *Index {
	return &Index{
		funcs:     make(map[string][]*parser.FunctionDef),
		fileFuncs: make(map[string][]*parser.FunctionDef),
		comprefs:  make(map[string][]*parser.ComponentRef),
		fileRefs:  make(map[string][]*parser.ComponentRef),
		thisVars:  make(map[string][]string),
		scopeRefs: make(map[string]map[string][]*parser.ComponentRef),
		beans:     make(map[string]string),
		entities:  make(map[string]uri.URI),
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

// ownFunc and ownRef copy an entry into storage the index alone owns.
//
// Indexing used to file `&funcs[i]` — a pointer into the caller's slice, which
// for the LSP server is the live ParseResult for an open document. That gave
// the entries two writers. ParseResult.ApplyEdit shifts line numbers on every
// in-function keystroke (internal/parser/incremental.go), holding the server's
// per-document lock and knowing nothing about this index; meanwhile another
// editor's connection could be reading the same struct through Lookup. In
// daemon mode one Index is shared by every connection, so those are genuinely
// concurrent, and neither lock covers the other's writer.
//
// Copying on the way in makes the index the only writer, which is what lets
// ShiftLines below fix the remaining half.
func ownFunc(d parser.FunctionDef) *parser.FunctionDef {
	return &d
}

func ownRef(r parser.ComponentRef) *parser.ComponentRef {
	return &r
}

// keepFuncs and keepRefs return a *new* slice of the entries satisfying keep.
//
// The obvious in-place form — `filtered := entries[:0]` followed by appends —
// writes over the backing array the map's slice already points at, and every
// accessor here (Lookup, FunctionsForFile, LookupComponentRef, RefsForFile)
// hands that same array straight out to the caller and then releases the read
// lock. A caller still walking the slice it was given is reading the array a
// concurrent IndexFile is compacting: `go test -race` reports it, and even with
// the timing on its side the caller silently sees another file's entries
// shifted into place. Allocating means the entries a caller was handed stay the
// entries it was handed.
//
// Appending to a map's slice is fine by contrast: it only ever writes at or
// past the length a caller can see.
func keepFuncs(entries []*parser.FunctionDef, keep func(*parser.FunctionDef) bool) []*parser.FunctionDef {
	var filtered []*parser.FunctionDef

	for _, e := range entries {
		if keep(e) {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

func keepRefs(entries []*parser.ComponentRef, keep func(*parser.ComponentRef) bool) []*parser.ComponentRef {
	var filtered []*parser.ComponentRef

	for _, e := range entries {
		if keep(e) {
			filtered = append(filtered, e)
		}
	}

	return filtered
}

// uriKey returns a lowercase URI for case-insensitive comparison on case-insensitive filesystems.
func uriKey(u uri.URI) string {
	return strings.ToLower(string(u))
}

// FunctionsForFile returns all indexed function definitions for a specific file.
func (idx *Index) FunctionsForFile(fileURI uri.URI) []*parser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.fileFuncs[uriKey(fileURI)]
}

// ShiftLines adjusts line numbers for all entries in a file where Line > afterLine.
//
// It replaces the affected entries rather than writing through the pointers it
// handed out. Every accessor here returns those pointers and then releases the
// read lock, so a caller reading def.Line is doing so unsynchronised; writing
// the field in place raced it, which `go test -race` reports as a write in
// ShiftLines against the caller's read. A caller cannot hold the index lock for
// as long as it holds the entry, so the write has to move instead.
//
// The replacement is threaded through all four maps at once: funcs and
// fileFuncs hold the same pointers, as do comprefs and fileRefs, and updating
// one without the other would leave a file's entries disagreeing about where
// its functions are.
//
// A caller that captured an entry before the shift keeps reading the old line
// number, which is what it would have read had it looked a moment earlier. It
// is a snapshot, not a torn value.
func (idx *Index) ShiftLines(fileURI uri.URI, afterLine int, delta int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	key := uriKey(fileURI)

	newFuncs := make(map[*parser.FunctionDef]*parser.FunctionDef)

	for _, defs := range idx.funcs {
		for _, d := range defs {
			if uriKey(d.URI) == key && int(d.Line) > afterLine && newFuncs[d] == nil {
				shifted := *d
				shifted.Line = uint32(int(shifted.Line) + delta)
				newFuncs[d] = &shifted
			}
		}
	}

	newRefs := make(map[*parser.ComponentRef]*parser.ComponentRef)

	for _, refs := range idx.comprefs {
		for _, r := range refs {
			if uriKey(r.URI) == key && int(r.Line) > afterLine && newRefs[r] == nil {
				shifted := *r
				shifted.Line = uint32(int(shifted.Line) + delta)
				newRefs[r] = &shifted
			}
		}
	}

	if len(newFuncs) == 0 && len(newRefs) == 0 {
		return
	}

	for k, defs := range idx.funcs {
		idx.funcs[k] = remapFuncs(defs, newFuncs)
	}

	for k, defs := range idx.fileFuncs {
		idx.fileFuncs[k] = remapFuncs(defs, newFuncs)
	}

	for k, refs := range idx.comprefs {
		idx.comprefs[k] = remapRefs(refs, newRefs)
	}

	for k, refs := range idx.fileRefs {
		idx.fileRefs[k] = remapRefs(refs, newRefs)
	}

	for fk, scopes := range idx.scopeRefs {
		for sk, refs := range scopes {
			idx.scopeRefs[fk][sk] = remapRefs(refs, newRefs)
		}
	}
}

// remapFuncs and remapRefs rebuild a slice with replaced entries substituted in.
// The slice is rebuilt rather than written through for the same reason
// ShiftLines replaces rather than mutates: a caller may still be walking it.
func remapFuncs(entries []*parser.FunctionDef, replaced map[*parser.FunctionDef]*parser.FunctionDef) []*parser.FunctionDef {
	out := make([]*parser.FunctionDef, len(entries))

	for i, e := range entries {
		if n := replaced[e]; n != nil {
			out[i] = n
		} else {
			out[i] = e
		}
	}

	return out
}

func remapRefs(entries []*parser.ComponentRef, replaced map[*parser.ComponentRef]*parser.ComponentRef) []*parser.ComponentRef {
	out := make([]*parser.ComponentRef, len(entries))

	for i, e := range entries {
		if n := replaced[e]; n != nil {
			out[i] = n
		} else {
			out[i] = e
		}
	}

	return out
}

// IndexFile parses the given CFC content and updates the index for that file URI.
func (idx *Index) IndexFile(fileURI uri.URI, content string) {
	pr := parser.Parse(fileURI, content)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)
	fk := uriKey(fileURI)
	idx.thisVars[fk] = pr.ThisVars()

	fileDefs := make([]*parser.FunctionDef, 0, len(pr.Funcs))

	for i := range pr.Funcs {
		d := ownFunc(pr.Funcs[i])
		key := strings.ToLower(d.Name)
		idx.funcs[key] = append(idx.funcs[key], d)
		fileDefs = append(fileDefs, d)
	}

	idx.fileFuncs[fk] = fileDefs

	fileRefsList := make([]*parser.ComponentRef, 0, len(pr.ComponentRefs))

	for i := range pr.ComponentRefs {
		r := ownRef(pr.ComponentRefs[i])
		key := strings.ToLower(r.Variable)
		idx.comprefs[key] = append(idx.comprefs[key], r)
		fileRefsList = append(fileRefsList, r)
	}

	idx.fileRefs[fk] = fileRefsList
}

// IndexFileFromResult updates the index using pre-parsed function defs and refs.
func (idx *Index) IndexFileFromResult(fileURI uri.URI, funcs []parser.FunctionDef, refs []parser.ComponentRef) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)
	fk := uriKey(fileURI)

	fileDefs := make([]*parser.FunctionDef, 0, len(funcs))

	for i := range funcs {
		d := ownFunc(funcs[i])
		key := strings.ToLower(d.Name)
		idx.funcs[key] = append(idx.funcs[key], d)
		fileDefs = append(fileDefs, d)
	}

	idx.fileFuncs[fk] = fileDefs

	fileRefsList := make([]*parser.ComponentRef, 0, len(refs))

	for i := range refs {
		r := ownRef(refs[i])
		key := strings.ToLower(r.Variable)
		idx.comprefs[key] = append(idx.comprefs[key], r)
		fileRefsList = append(fileRefsList, r)
	}

	idx.fileRefs[fk] = fileRefsList
}

// RemoveFilesUnder removes all indexed entries whose URI starts with prefix.
func (idx *Index) RemoveFilesUnder(prefix string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	for key, entries := range idx.funcs {
		filtered := keepFuncs(entries, func(e *parser.FunctionDef) bool {
			return !strings.HasPrefix(string(e.URI), prefix)
		})

		if len(filtered) == 0 {
			delete(idx.funcs, key)
		} else {
			idx.funcs[key] = filtered
		}
	}

	for key, entries := range idx.comprefs {
		filtered := keepRefs(entries, func(e *parser.ComponentRef) bool {
			return !strings.HasPrefix(string(e.URI), prefix)
		})

		if len(filtered) == 0 {
			delete(idx.comprefs, key)
		} else {
			idx.comprefs[key] = filtered
		}
	}
}

func (idx *Index) removeFileEntries(fileURI uri.URI) {
	key := uriKey(fileURI)
	delete(idx.thisVars, key)
	delete(idx.fileFuncs, key)
	delete(idx.fileRefs, key)
	delete(idx.scopeRefs, key)

	for k, entries := range idx.funcs {
		filtered := keepFuncs(entries, func(e *parser.FunctionDef) bool {
			return uriKey(e.URI) != key
		})

		if len(filtered) == 0 {
			delete(idx.funcs, k)
		} else {
			idx.funcs[k] = filtered
		}
	}

	for k, entries := range idx.comprefs {
		filtered := keepRefs(entries, func(e *parser.ComponentRef) bool {
			return uriKey(e.URI) != key
		})

		if len(filtered) == 0 {
			delete(idx.comprefs, k)
		} else {
			idx.comprefs[k] = filtered
		}
	}
}

// LookupComponentRef returns component references for the given variable name.
func (idx *Index) LookupComponentRef(variable string) []*parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.comprefs[strings.ToLower(variable)]
}

// RefsForFile returns all component references indexed for a specific file.
func (idx *Index) RefsForFile(fileURI uri.URI) []*parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.fileRefs[uriKey(fileURI)]
}

// ThisVarsForFile returns the this-scoped variable names for a file.
func (idx *Index) ThisVarsForFile(fileURI uri.URI) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.thisVars[uriKey(fileURI)]
}

// SetThisVars stores this-scoped variable names for a file.
func (idx *Index) SetThisVars(fileURI uri.URI, vars []string) {
	idx.mu.Lock()
	idx.thisVars[uriKey(fileURI)] = vars
	idx.mu.Unlock()
}

// SetFuncRefs records the component refs found inside one function scope,
// replacing whatever was recorded for that scope before.
//
// These arrive lazily: the server indexes a function's refs the first time a
// hover or a definition lookup lands inside it. The plain append this replaced
// had no way to tell a first indexing from a re-indexing, so every such lookup
// added another copy. Refs are memoised per function and invalidated when that
// function is edited, so an editing session alternating edits and hovers grew
// comprefs without bound — with duplicate refs at identical lines, which
// LookupComponentRefInFile then had to scan through on every subsequent call.
//
// scopeKey identifies the function within the file; the caller's line range is
// the natural choice.
func (idx *Index) SetFuncRefs(fileURI uri.URI, scopeKey string, refs []parser.ComponentRef) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	fk := uriKey(fileURI)

	if prev := idx.scopeRefs[fk][scopeKey]; len(prev) > 0 {
		drop := make(map[*parser.ComponentRef]bool, len(prev))
		for _, r := range prev {
			drop[r] = true
		}

		keep := func(e *parser.ComponentRef) bool { return !drop[e] }

		// Sweep each affected bucket once. Several refs in a scope routinely
		// share a variable name or a file, and filtering per ref would rebuild
		// the same bucket once per ref in it.
		varKeys := make(map[string]bool, len(prev))
		fileKeys := make(map[string]bool, len(prev))

		for _, r := range prev {
			varKeys[strings.ToLower(r.Variable)] = true
			fileKeys[uriKey(r.URI)] = true
		}

		for key := range varKeys {
			if kept := keepRefs(idx.comprefs[key], keep); len(kept) == 0 {
				delete(idx.comprefs, key)
			} else {
				idx.comprefs[key] = kept
			}
		}

		for rk := range fileKeys {
			if kept := keepRefs(idx.fileRefs[rk], keep); len(kept) == 0 {
				delete(idx.fileRefs, rk)
			} else {
				idx.fileRefs[rk] = kept
			}
		}
	}

	added := make([]*parser.ComponentRef, 0, len(refs))

	// Each ref is filed under its own URI, as the append this replaced did;
	// fileURI identifies only the scope whose refs are being replaced.
	for i := range refs {
		r := ownRef(refs[i])
		key := strings.ToLower(r.Variable)
		idx.comprefs[key] = append(idx.comprefs[key], r)
		rk := uriKey(r.URI)
		idx.fileRefs[rk] = append(idx.fileRefs[rk], r)
		added = append(added, r)
	}

	if idx.scopeRefs[fk] == nil {
		idx.scopeRefs[fk] = make(map[string][]*parser.ComponentRef)
	}

	idx.scopeRefs[fk][scopeKey] = added
}

// LookupComponentRefInFile returns the component ref for a variable in a specific file
// that is closest to (but not after) the given line.
func (idx *Index) LookupComponentRefInFile(variable string, fileURI uri.URI, line uint32) *parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	key := uriKey(fileURI)

	var best *parser.ComponentRef

	for _, ref := range idx.comprefs[strings.ToLower(variable)] {
		if uriKey(ref.URI) == key && ref.Line <= line {
			if best == nil || ref.Line > best.Line {
				best = ref
			}
		}
	}

	return best
}

// SetBeans replaces the bean map with the given name→dot-path mapping.
func (idx *Index) SetBeans(beans map[string]string) {
	idx.mu.Lock()
	idx.beans = beans
	idx.mu.Unlock()
}

// LookupBean returns the component dot-path for a bean name (case-insensitive).
func (idx *Index) LookupBean(name string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.beans[strings.ToLower(name)]
}

// SetEntity registers a persistent CFC as an ORM entity by name.
func (idx *Index) SetEntity(name string, fileURI uri.URI) {
	idx.mu.Lock()
	idx.entities[strings.ToLower(name)] = fileURI
	idx.mu.Unlock()
}

// LookupEntity returns the file URI for an ORM entity name (case-insensitive).
func (idx *Index) LookupEntity(name string) uri.URI {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return idx.entities[strings.ToLower(name)]
}

// FindFilesByBasename returns absolute file paths for all indexed CFC files whose
// filename (without extension) matches name case-insensitively.
func (idx *Index) FindFilesByBasename(name string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	suffix := "/" + strings.ToLower(name) + ".cfc"

	var paths []string

	for key, defs := range idx.fileFuncs {
		if !strings.HasSuffix(key, suffix) {
			continue
		}

		// Recover the real (correctly-cased) path from a stored definition's URI.
		if len(defs) > 0 {
			if p := strings.TrimPrefix(string(defs[0].URI), "file://"); p != "" {
				paths = append(paths, p)

				continue
			}
		}

		// File has no functions — fall back to the lowercased key (works on case-insensitive FSes).
		if p := strings.TrimPrefix(key, "file://"); p != "" {
			paths = append(paths, p)
		}
	}

	return paths
}
