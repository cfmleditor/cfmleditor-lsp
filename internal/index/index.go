// Package index maintains a searchable index of CFML function definitions.
package index

import (
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
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

	return snapshot(idx.funcs[strings.ToLower(name)])
}

// LookupPreferred answers the question hover, signature help and argument
// completion actually ask of a bare function name: the definition to show,
// preferring the one declared in the requesting file, plus how many candidates
// there were so a caller can tell "the only one" from "the first of several".
//
// It exists so those three do not go through Lookup, which copies the bucket it
// returns (see snapshot). That copy is the right default — a caller walking
// every match pays for what it walks — but these three want one entry, and the
// bucket for a name every component declares holds one entry per file in the
// workspace: 40KB per call, on paths that run while the user is typing.
//
// inFile distinguishes a real preference from a fallback, which hover needs:
// it shows a global match only when it is unambiguous.
func (idx *Index) LookupPreferred(name string, preferURI uri.URI) (def *parser.FunctionDef, inFile bool, total int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	defs := idx.funcs[strings.ToLower(name)]
	if len(defs) == 0 {
		return nil, false, 0
	}

	// The preference is answered from the file's own definitions rather than by
	// scanning the name's bucket for the file, so the cost is bounded by the
	// open document instead of by the workspace. The two views hold the same
	// entries, so the candidate set is the same either way.
	//
	// One deliberate difference from the callers this replaces, which each
	// compared d.URI == docURI raw: the file is found by uriKey, the same
	// normalisation FunctionsForFile and every other per-file accessor here
	// uses. That makes this agree with the rest of the index on what "the same
	// file" means, where the raw comparison could miss a definition over a
	// percent-escape or a case difference in the URI. It is free — uriKey runs
	// once on the argument, not once per entry.
	for _, d := range idx.fileFuncs[uriKey(preferURI)] {
		if strings.EqualFold(d.Name, name) {
			return d, true, len(defs)
		}
	}

	return nearestTo(defs, preferURI), false, len(defs)
}

// nearestTo picks the definition in the directory closest to preferURI, and the
// lowest URI among equals.
//
// This used to be defs[0] — the order the bucket happened to hold, which is the
// order eight parallel indexing goroutines finished in, so it differed between
// restarts and hovering the same call could name a different component than it
// did yesterday. Nearest is both stable and the better guess: a workspace where
// several components declare the same method name is usually one where the
// relevant one is the near one.
//
// The lowest-URI fallback is not cosmetic. Without it two files the same
// distance away leave the answer decided by bucket order again, which is the
// thing being fixed.
func nearestTo(defs []*parser.FunctionDef, preferURI uri.URI) *parser.FunctionDef {
	ref := string(preferURI)
	best := defs[0]
	bestDist := cfpath.URIDistance(ref, string(best.URI))

	for _, d := range defs[1:] {
		dist := cfpath.URIDistance(ref, string(d.URI))
		if dist < bestDist || (dist == bestDist && d.URI < best.URI) {
			best, bestDist = d, dist
		}
	}

	return best
}

// CountFunctions reports how many definitions are indexed under a name, without
// building the slice Lookup would have to copy to answer the same question.
func (idx *Index) CountFunctions(name string) int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.funcs[strings.ToLower(name)])
}

// AllFunctions returns every indexed function definition.
func (idx *Index) AllFunctions() []*parser.FunctionDef {
	return idx.FunctionsMatching(nil)
}

// FunctionsMatching returns the indexed definitions whose lowercased name
// satisfies match, or every definition when match is nil.
//
// The predicate is given the bucket key, so it runs once per distinct name
// rather than once per definition: a workspace of 5,000 components declaring
// the same eight methods asks it eight times, not forty thousand. It sees only
// a name, which is what keeps it safe to run under the read lock — it has
// nothing to re-enter the index with.
//
// workspace/symbol is the reason this exists. It is issued on every keystroke
// in the symbol picker and it discards all but a handful of matches, but it
// went through AllFunctions, which materialised every definition in the
// workspace into a slice grown from nil: 3.1MB allocated and copied per
// keystroke on a 40,000-definition index, to return a few dozen symbols.
func (idx *Index) FunctionsMatching(match func(loweredName string) bool) []*parser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	total := 0

	for name, defs := range idx.funcs {
		if match == nil || match(name) {
			total += len(defs)
		}
	}

	if total == 0 {
		return nil
	}

	all := make([]*parser.FunctionDef, 0, total)

	for name, defs := range idx.funcs {
		if match == nil || match(name) {
			all = append(all, defs...)
		}
	}

	return all
}

// Every entry below is filed as new(funcs[i]) — a copy the index alone owns —
// and never as &funcs[i].
//
// A pointer into the caller's slice is, for the LSP server, a pointer into the
// live ParseResult for an open document. That gives the entry two writers.
// ParseResult.ApplyEdit shifts line numbers on every in-function keystroke
// (internal/parser/incremental.go), holding the server's per-document lock and
// knowing nothing about this index; meanwhile another editor's connection could
// be reading the same struct through Lookup. In daemon mode one Index is shared
// by every connection, so those are genuinely concurrent, and neither lock
// covers the other's writer.
//
// Copying on the way in makes the index the only writer, which is what lets
// ShiftLines below fix the remaining half. new(x) allocates a fresh variable
// initialised to x's value rather than aliasing it, so it carries the same
// guarantee the ownFunc/ownRef helpers used to.

// snapshot copies a stored slice for a caller.
//
// Every accessor here hands its result out and then releases the read lock, so
// what it returns has to be storage no writer will touch. That used to be
// arranged the other way round — the maps held slices no writer ever mutated in
// place, so a read could hand out the live one for free — and it made writes
// pay for reads. funcs is keyed by lowercased name, so a workspace where every
// component declares init has one bucket holding an entry per file, and
// changing this file's single entry in it rebuilt all 5,000: ~1ms of
// write-locked work and 660KB of garbage per line-changing keystroke, to alter
// one pointer.
//
// Copying on the way out inverts that. A read now pays for the entries it is
// about to iterate anyway, which is a constant factor on work it was already
// doing, while a write pays only for the entries it actually changes. Typing
// is far more frequent than hovering, and a bucket is large in exactly the
// case where the reader wants all of it.
//
// nil for an empty result keeps the accessors' old "nothing indexed" answer
// distinguishable from an empty non-nil slice, which some callers compare
// against nil.
func snapshot[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}

	return append(make([]T, 0, len(s)), s...)
}

// keepFuncs and keepRefs compact entries in place, returning the prefix that
// satisfied keep.
//
// Writing over the backing array is safe precisely because snapshot exists: no
// caller is holding it. Before that it was not, and this allocated a fresh
// slice for every filtered bucket — the comment here recorded a race that
// `go test -race` had actually reported, where a caller walking the slice it
// had been given read another file's entries shifted into place by a
// concurrent IndexFile.
//
// The tail is cleared so the entries dropped from the bucket can be collected
// rather than pinned by an array the map still points at.
func keepFuncs(entries []*parser.FunctionDef, keep func(*parser.FunctionDef) bool) []*parser.FunctionDef {
	n := 0

	for _, e := range entries {
		if keep(e) {
			entries[n] = e
			n++
		}
	}

	clear(entries[n:])

	return entries[:n]
}

func keepRefs(entries []*parser.ComponentRef, keep func(*parser.ComponentRef) bool) []*parser.ComponentRef {
	n := 0

	for _, e := range entries {
		if keep(e) {
			entries[n] = e
			n++
		}
	}

	clear(entries[n:])

	return entries[:n]
}

// uriKey returns a lowercase URI for case-insensitive comparison on case-insensitive filesystems.
func uriKey(u uri.URI) string {
	s := string(u)

	// Percent-escapes are decoded so that the two ways this codebase builds a
	// file URI land on one key. Indexing goes through uri.File, which escapes:
	// a workspace under "/My Documents" is stored as "file:///My%20Documents/…".
	// Several lookups build "file://" + path by hand instead, so they ask for
	// "file:///My Documents/…" and miss every entry — this-scope completions
	// simply stopped appearing for anyone whose path contained a space, with no
	// error anywhere. Normalising here fixes both directions at once, which the
	// alternative — rewriting all 37 URI conversions in one go — does not.
	if dec, err := url.PathUnescape(s); err == nil {
		s = dec
	}

	return strings.ToLower(s)
}

// FunctionsForFile returns all indexed function definitions for a specific file.
func (idx *Index) FunctionsForFile(fileURI uri.URI) []*parser.FunctionDef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return snapshot(idx.fileFuncs[uriKey(fileURI)])
}

// HasFile reports whether the index holds an entry for this file, which is not
// the same question as whether the file declares any functions: a component
// with only properties or `this` assignments indexes to an empty — but present
// — entry, and callers that use "no functions" as a stand-in for "never
// indexed" re-read such a file from disk on every lookup.
func (idx *Index) HasFile(fileURI uri.URI) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	_, ok := idx.fileFuncs[uriKey(fileURI)]

	return ok
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

	// Only the edited file's entries can move, and fileFuncs/fileRefs already
	// index exactly those. This used to walk every bucket of funcs and comprefs
	// to find them and then rebuild all five maps wholesale — on a 5,000-file
	// workspace, 18ms of write-locked work per line-changing keystroke,
	// blocking every other session's lookups for the duration. Measured by
	// BenchmarkShiftLines, that becomes 6µs where method names differ between
	// files.
	//
	// The replacement is then written into the affected buckets in place. What
	// used to remain proportional to workspace size was exactly that step:
	// funcs is keyed by lowercased name, so a workspace where every component
	// declares `init` has one bucket holding an entry per file, and swapping
	// this file's single pointer in it meant rebuilding all 5,000 — because
	// callers held the bucket slice after the read lock was released. Accessors
	// now hand out a copy (see snapshot), so the bucket is the index's alone to
	// write and the worst case is flat rather than ~0.9ms.
	//
	// The entry itself is still replaced rather than written through: a caller
	// holding a *FunctionDef is reading def.Line unsynchronised, so the struct
	// has to be new even though the slot holding it need not be.
	// Replacements are collected per bucket, for the reason removeFileEntries
	// gives: a bucket sweep should only consider the entries that could be in
	// that bucket. Pooling them into one map keyed by pointer made every entry
	// of a 5,000-entry `init` bucket pay a hash and a probe to be told it was
	// not one of this file's few.
	funcReps := make(map[string][]funcReplacement)
	fileFuncReps := make([]funcReplacement, 0, len(idx.fileFuncs[key]))

	for _, d := range idx.fileFuncs[key] {
		if int(d.Line) <= afterLine {
			continue
		}

		shifted := *d
		shifted.Line = uint32(int(shifted.Line) + delta)

		rep := funcReplacement{old: d, new: &shifted}
		name := strings.ToLower(d.Name)
		funcReps[name] = append(funcReps[name], rep)
		fileFuncReps = append(fileFuncReps, rep)
	}

	refReps := make(map[string][]refReplacement)
	fileRefReps := make([]refReplacement, 0, len(idx.fileRefs[key]))

	for _, r := range idx.fileRefs[key] {
		if int(r.Line) <= afterLine {
			continue
		}

		shifted := *r
		shifted.Line = uint32(int(shifted.Line) + delta)

		rep := refReplacement{old: r, new: &shifted}
		name := strings.ToLower(r.Variable)
		refReps[name] = append(refReps[name], rep)
		fileRefReps = append(fileRefReps, rep)
	}

	if len(fileFuncReps) == 0 && len(fileRefReps) == 0 {
		return
	}

	// Only the buckets holding a replaced entry need touching. A definition
	// lives under its lowercased name and a ref under its lowercased variable,
	// so the affected keys are known without searching for them.
	for name, reps := range funcReps {
		remapFuncs(idx.funcs[name], reps)
	}

	remapFuncs(idx.fileFuncs[key], fileFuncReps)

	for name, reps := range refReps {
		remapRefs(idx.comprefs[name], reps)
	}

	remapRefs(idx.fileRefs[key], fileRefReps)

	for _, refs := range idx.scopeRefs[key] {
		remapRefs(refs, fileRefReps)
	}
}

// funcReplacement and refReplacement pair an entry with the shifted copy that
// supersedes it.
type funcReplacement struct {
	old, new *parser.FunctionDef
}

type refReplacement struct {
	old, new *parser.ComponentRef
}

// remapFuncs and remapRefs substitute replaced entries into a stored slice in
// place. Safe because accessors hand out a copy (see snapshot), so the only
// slices these are called on are the index's own.
//
// reps is scanned rather than hashed: for a name bucket it holds the entries of
// one file carrying one name, which is one of them unless that file declares
// the name twice.
func remapFuncs(entries []*parser.FunctionDef, reps []funcReplacement) {
	for i, e := range entries {
		for _, r := range reps {
			if r.old == e {
				entries[i] = r.new

				break
			}
		}
	}
}

func remapRefs(entries []*parser.ComponentRef, reps []refReplacement) {
	for i, e := range entries {
		for _, r := range reps {
			if r.old == e {
				entries[i] = r.new

				break
			}
		}
	}
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
		d := new(pr.Funcs[i])
		key := strings.ToLower(d.Name)
		idx.funcs[key] = append(idx.funcs[key], d)
		fileDefs = append(fileDefs, d)
	}

	idx.fileFuncs[fk] = fileDefs

	fileRefsList := make([]*parser.ComponentRef, 0, len(pr.ComponentRefs))

	for i := range pr.ComponentRefs {
		r := new(pr.ComponentRefs[i])
		key := strings.ToLower(r.Variable)
		idx.comprefs[key] = append(idx.comprefs[key], r)
		fileRefsList = append(fileRefsList, r)
	}

	idx.fileRefs[fk] = fileRefsList
}

// IndexFileFromResult updates the index using pre-parsed function defs and refs.
func (idx *Index) IndexFileFromResult(fileURI uri.URI, funcs []parser.FunctionDef, refs []parser.ComponentRef) {
	// The strings a parse produces are slices of the file's source, and a Go
	// substring keeps the whole backing array alive. That is right for a parse
	// result, which dies with the source it came from, and wrong here: the index
	// outlives every one of them, so a single retained function name holds its
	// entire file in memory and an index of a workspace holds the workspace.
	//
	// On a real one — 18,245 files, 322MB of source — the index retained 285MB,
	// 0.9x the source it was built from, for data whose own size is a few
	// megabytes. Copying the strings as they are stored takes it to 55.6MB.
	//
	// Done here rather than at each caller because this is the one door into the
	// index, and a caller that forgot would put the retention back with nothing
	// to show it had.
	funcs = parser.CompactDefs(funcs)
	refs = parser.CompactRefs(refs)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)
	fk := uriKey(fileURI)

	fileDefs := make([]*parser.FunctionDef, 0, len(funcs))

	for i := range funcs {
		d := new(funcs[i])
		key := strings.ToLower(d.Name)
		idx.funcs[key] = append(idx.funcs[key], d)
		fileDefs = append(fileDefs, d)
	}

	idx.fileFuncs[fk] = fileDefs

	fileRefsList := make([]*parser.ComponentRef, 0, len(refs))

	for i := range refs {
		r := new(refs[i])
		key := strings.ToLower(r.Variable)
		idx.comprefs[key] = append(idx.comprefs[key], r)
		fileRefsList = append(fileRefsList, r)
	}

	idx.fileRefs[fk] = fileRefsList
}

// RemoveFile drops every entry the index holds for one file — its functions,
// component refs, this-scoped vars, per-function scope refs, and its ORM entity
// registration if it had one. Used when a watched file is deleted on disk.
//
// The entity map is keyed by name rather than by URI, so it is swept by value
// rather than by a computed key: an entity name the deleted file registered may
// since have been claimed by a different file, and deleting the key blind would
// unregister that one instead.
func (idx *Index) RemoveFile(fileURI uri.URI) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.removeFileEntries(fileURI)

	key := uriKey(fileURI)
	for name, u := range idx.entities {
		if uriKey(u) == key {
			delete(idx.entities, name)
		}
	}
}

// RemoveFilesUnder removes all indexed entries whose URI starts with prefix.
// Called when a workspace folder is removed, so it is rare and may walk every
// bucket; what it must not do is leave the two views of an entry disagreeing.
//
// It used to clear funcs and comprefs alone. The per-file maps then still
// listed the removed folder's files, which is wrong twice over: FunctionsForFile
// kept answering with definitions no bucket held any more, and HasFile kept
// reporting the file as indexed, so Resolver.EnsureIndexed never re-read it if
// the folder came back. It also leaves removeFileEntries — which reaches the
// name buckets through exactly these maps — reconciling a view no writer
// maintains.
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

	// The per-file maps are keyed by uriKey, which lowercases and decodes the
	// URI, so the prefix has to be put through the same normalisation before it
	// can be compared against a key.
	fileKey := uriKey(uri.URI(prefix))

	for key := range idx.fileFuncs {
		if strings.HasPrefix(key, fileKey) {
			delete(idx.fileFuncs, key)
			delete(idx.fileRefs, key)
			delete(idx.thisVars, key)
			delete(idx.scopeRefs, key)
		}
	}

	// fileRefs can hold a file that declared no functions, so it cannot be
	// swept only alongside fileFuncs.
	for key := range idx.fileRefs {
		if strings.HasPrefix(key, fileKey) {
			delete(idx.fileRefs, key)
			delete(idx.thisVars, key)
			delete(idx.scopeRefs, key)
		}
	}

	for key := range idx.thisVars {
		if strings.HasPrefix(key, fileKey) {
			delete(idx.thisVars, key)
			delete(idx.scopeRefs, key)
		}
	}
}

// removeFileEntries drops one file's functions and component refs from every
// map that holds them.
//
// It reaches the name buckets through fileFuncs/fileRefs, which already hold
// exactly this file's entries, rather than walking every bucket in the index
// and asking each entry which file it came from. That walk is the same shape
// ShiftLines was fixed for and it was left here, where it costs more: every
// index write goes through this function, so re-indexing one file was
// proportional to the whole workspace and the startup scan was quadratic in
// the file count. Measured by BenchmarkIndexFileFromResult on a 5,000-file
// index, one file cost 43ms and 160,020 allocations.
//
// The per-entry uriKey was most of it. It lowercases and percent-decodes, both
// of which allocate on a real workspace path, and it ran once per entry in the
// index per file indexed: 5.9 GB of the 6.6 GB a 5,624-file corpus scan
// allocated, and 86% of its CPU, was this one comparison.
//
// Entries are matched by pointer identity, not by URI, because that is what
// makes the bucket set knowable in advance: a definition lives under its
// lowercased name and a ref under its lowercased variable, and fileFuncs holds
// the very pointers the name buckets do (ownFunc/ownRef mint one pointer per
// entry, so identity is exact). The two views cannot disagree — every writer
// here fills both together, and each removal runs before the write that
// replaces them.
func (idx *Index) removeFileEntries(fileURI uri.URI) {
	key := uriKey(fileURI)
	delete(idx.thisVars, key)
	delete(idx.scopeRefs, key)

	// Grouped by bucket, not pooled into one set of everything this file
	// declares. Both spellings visit the same bucket entries; the difference is
	// what each visit costs. A pooled set has to be a map, so every entry in a
	// 5,000-entry `init` bucket pays a hash and a probe to be told it is not
	// one of this file's eight — 57% of the time this function spent, measured,
	// was runtime.mapaccess1. Grouped, the candidates for a bucket are the
	// entries of this file that carry that one name, which is one of them
	// except where a file declares the same name twice, so the test is a
	// pointer comparison or two.
	for name, group := range groupFuncsByName(idx.fileFuncs[key]) {
		if filtered := keepFuncs(idx.funcs[name], notInFuncs(group)); len(filtered) == 0 {
			delete(idx.funcs, name)
		} else {
			idx.funcs[name] = filtered
		}
	}

	delete(idx.fileFuncs, key)

	for name, group := range groupRefsByVariable(idx.fileRefs[key]) {
		if filtered := keepRefs(idx.comprefs[name], notInRefs(group)); len(filtered) == 0 {
			delete(idx.comprefs, name)
		} else {
			idx.comprefs[name] = filtered
		}
	}

	delete(idx.fileRefs, key)
}

// groupFuncsByName and groupRefsByVariable bucket one file's entries by the key
// they are filed under, so a bucket sweep only has to consider the entries that
// could actually be in it.
func groupFuncsByName(defs []*parser.FunctionDef) map[string][]*parser.FunctionDef {
	if len(defs) == 0 {
		return nil
	}

	byName := make(map[string][]*parser.FunctionDef, len(defs))
	for _, d := range defs {
		k := strings.ToLower(d.Name)
		byName[k] = append(byName[k], d)
	}

	return byName
}

func groupRefsByVariable(refs []*parser.ComponentRef) map[string][]*parser.ComponentRef {
	if len(refs) == 0 {
		return nil
	}

	byVar := make(map[string][]*parser.ComponentRef, len(refs))
	for _, r := range refs {
		k := strings.ToLower(r.Variable)
		byVar[k] = append(byVar[k], r)
	}

	return byVar
}

// notInFuncs and notInRefs are keep predicates over a group small enough that a
// linear scan beats hashing. The group holds one file's entries under a single
// name, so its length is the number of times that file declares that name.
func notInFuncs(group []*parser.FunctionDef) func(*parser.FunctionDef) bool {
	return func(e *parser.FunctionDef) bool {
		return !slices.Contains(group, e)
	}
}

func notInRefs(group []*parser.ComponentRef) func(*parser.ComponentRef) bool {
	return func(e *parser.ComponentRef) bool {
		return !slices.Contains(group, e)
	}
}

// LookupComponentRef returns component references for the given variable name.
func (idx *Index) LookupComponentRef(variable string) []*parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return snapshot(idx.comprefs[strings.ToLower(variable)])
}

// RefsForFile returns all component references indexed for a specific file.
func (idx *Index) RefsForFile(fileURI uri.URI) []*parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return snapshot(idx.fileRefs[uriKey(fileURI)])
}

// ThisVarsForFile returns the this-scoped variable names for a file.
func (idx *Index) ThisVarsForFile(fileURI uri.URI) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return snapshot(idx.thisVars[uriKey(fileURI)])
}

// SetThisVars stores this-scoped variable names for a file.
func (idx *Index) SetThisVars(fileURI uri.URI, vars []string) {
	idx.mu.Lock()
	// Owned, for the reason ownFunc gives: the caller's slice is a live
	// ParseResult's, and the index must be the only writer of what it stores.
	idx.thisVars[uriKey(fileURI)] = snapshot(vars)
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
		r := new(refs[i])
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
//
// It searches the file's own refs for the name, not the name's bucket for the
// file. Both views hold the same entries, so the answer is the same, but the
// sizes are not: comprefs is keyed by variable name, and the names this is
// asked about are the ordinary ones — svc, dao, qry — so that bucket holds one
// entry per file in the workspace. Searching it meant a uriKey per entry, which
// lowercases and percent-decodes and so allocates, on a lookup that hover,
// definition and completion each perform on the keystroke (hover twice).
//
// Measured on a 5,000-file workspace: 0.98ms and 160KB per lookup, against
// 0.29µs and 32 bytes now, flat as the workspace grows. The trade is real but
// small and bounded by the open file: where the old form found the name's
// bucket holding one entry, a file declaring 200 refs now costs 1.6µs against
// the 0.34µs it used to.
func (idx *Index) LookupComponentRefInFile(variable string, fileURI uri.URI, line uint32) *parser.ComponentRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var best *parser.ComponentRef

	for _, ref := range idx.fileRefs[uriKey(fileURI)] {
		if ref.Line > line || !strings.EqualFold(ref.Variable, variable) {
			continue
		}

		if best == nil || ref.Line > best.Line {
			best = ref
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
