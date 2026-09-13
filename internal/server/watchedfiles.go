package server

import (
	"context"
	"path/filepath"
	"strings"

	json "github.com/go-json-experiment/json"

	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// watchedFilesRegistrationID is the id the server registers its file watchers
// under. Fixed rather than generated: the daemon re-registers on nothing, and a
// stable id is what would let an unregisterCapability find them later.
const watchedFilesRegistrationID = "cfmleditor-watched-files"

// watchedGlobs are the patterns the client is asked to watch. Relative patterns
// (LSP 3.17) are deliberately not used — a plain glob is resolved against every
// workspace folder by every client, including the ones that never advertised
// relativePatternSupport.
//
// `.cfm` is watched as well as `.cfc` even though indexWorkspace only walks
// `.cfc`, because Application.cfm carries mappings, beans and ORM locations
// that cfpath caches; a change to one has to invalidate that cache the same way
// saving it in the editor does.
var watchedGlobs = []string{"**/*.cfc", "**/*.cfm"}

// registerFileWatchers asks the client to watch the workspace for CFML files
// changing outside the editor.
//
// Without this the index is a snapshot: indexWorkspace runs once at startup and
// nothing afterwards updates it except didOpen/didChange/didSave, which only
// ever cover files open in the editor. A git checkout, a branch switch, a
// codegen step or a second editor would leave the server answering completion,
// go-to-definition and hover from components that no longer exist — confidently
// wrong rather than merely stale. Daemon mode makes it worse: the daemon
// outlives every client, so one stale snapshot is shared by all of them and
// survives closing the editor.
//
// This is a server-initiated dynamic registration, so it needs nothing from the
// extension — any client advertising didChangeWatchedFiles.dynamicRegistration
// starts watching on its own.
func (s *Server) registerFileWatchers(ctx context.Context) {
	watchers := make([]protocol.FileSystemWatcher, 0, len(watchedGlobs))
	for _, g := range watchedGlobs {
		watchers = append(watchers, protocol.FileSystemWatcher{GlobPattern: protocol.Pattern(g)})
	}

	// RegisterOptions is an LSPAny — a raw JSON value — so the typed options
	// have to be marshalled rather than assigned.
	opts, err := json.Marshal(protocol.DidChangeWatchedFilesRegistrationOptions{Watchers: watchers})
	if err != nil {
		s.log.Error("failed to encode file watcher registration", cflog.Err(err))

		return
	}

	s.call(ctx, protocol.MethodClientRegisterCapability, &protocol.RegistrationParams{
		Registrations: []protocol.Registration{{
			ID:              watchedFilesRegistrationID,
			Method:          protocol.MethodWorkspaceDidChangeWatchedFiles,
			RegisterOptions: opts,
		}},
	}, nil)

	s.log.Info("registered file watchers", cflog.Strings("globs", watchedGlobs))
}

// handleDidChangeWatchedFiles applies on-disk changes to the index.
//
// Decoded with encoding/json/v2, as every other handler in this package is and
// as the jsonrpc2 codec on the wire already was — this file was the one left on
// the standard library. It matters more here than elsewhere because a checkout
// or a branch switch arrives as one batch of thousands of events: measured on
// 5,000, 4.6ms and 5,024 allocations against 1.3ms and 4.
func (s *Server) handleDidChangeWatchedFiles(_ context.Context, rawParams []byte) (any, error) { //nolint:unparam // notifications have no result; kept for uniform dispatch signature
	var params protocol.DidChangeWatchedFilesParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, err
	}

	if len(params.Changes) == 0 {
		return nil, nil
	}

	// A checkout can report thousands of files at once, and each one parses.
	// The read loop must not wait for that.
	changes := params.Changes

	s.safeGo("applyWatchedFileChanges", func() { s.applyWatchedFileChanges(changes) })

	return nil, nil
}

// applyWatchedFileChanges re-indexes or drops each changed file, then
// invalidates the caches whose contents the batch could have falsified.
func (s *Server) applyWatchedFileChanges(changes []protocol.FileEvent) {
	// The gate lives here rather than in the handler so that every route to
	// this work passes it — a client that kept a registration across a config
	// change, and any future caller. A gate in the handler alone is one a
	// direct call slips past, which is exactly what let the first version of
	// this test pass with the switch ignored.
	if !s.Features.WatchedFiles {
		return
	}

	var indexed, removed, skipped, appChanged int

	for _, ev := range changes {
		switch s.applyWatchedFileChange(ev) {
		case watchedIndexed:
			indexed++
		case watchedRemoved:
			removed++
		case watchedSkipped:
			skipped++
		}

		if isApplicationFile(cfpath.FromURI(string(ev.URI))) {
			appChanged++
		}
	}

	if indexed == 0 && removed == 0 {
		s.log.Debug("watched files: nothing to apply", cflog.Int("skipped", skipped))

		return
	}

	// Both caches are keyed on answers the index just stopped agreeing with.
	// This mirrors what didSave does for an edit made in the editor.
	s.invalidateResolveCache()

	if appChanged > 0 {
		cfpath.InvalidateAppMappingsCache()
	}

	s.log.Info("watched files applied",
		cflog.Int("indexed", indexed),
		cflog.Int("removed", removed),
		cflog.Int("skipped", skipped),
		cflog.Int("applicationFiles", appChanged))
}

// watchedOutcome is what applyWatchedFileChange did with one event.
type watchedOutcome int

const (
	watchedSkipped watchedOutcome = iota
	watchedIndexed
	watchedRemoved
)

func (s *Server) applyWatchedFileChange(ev protocol.FileEvent) watchedOutcome {
	path := cfpath.FromURI(string(ev.URI))
	if !cfpath.IsCFMLFile(path) {
		return watchedSkipped
	}

	fileURI := uri.File(path)

	// A deletion is honoured whether or not the file is open. An open buffer
	// keeps its own content, but an index entry pointing at a path that no
	// longer exists resolves go-to-definition to nothing and cannot recover on
	// its own; re-saving the buffer re-indexes it through didSave. Content, by
	// contrast, is the editor's to report — see below.
	if ev.Type == protocol.FileChangeTypeDeleted {
		s.index.RemoveFile(fileURI)
		s.dropFileCaches(fileURI)

		return watchedRemoved
	}

	// An open document's buffer outranks what is on disk: it may hold unsaved
	// edits, and when it does not the editor reloads it and sends didChange
	// itself. Reading the file here would file the editor's own save a second
	// time at best, and overwrite newer buffer content at worst.
	if _, open := s.getDocument(fileURI); open {
		return watchedSkipped
	}

	// The gate reindexIfCFC applies to editor writes, applied here for the same
	// reason: a workspace that has narrowed indexing to certain roots does not
	// want a file outside them indexed just because it changed.
	if cfpath.IsCFCFile(path) && len(s.WorkspaceFolders) > 0 && !s.isIncludedPath(string(fileURI)) {
		return watchedSkipped
	}

	data, err := s.FS.ReadFile(path)
	if err != nil {
		// Created-then-deleted inside one batch, or a file the watcher saw
		// mid-write. Treat it as gone rather than leaving the previous
		// contents indexed as though they were current.
		s.index.RemoveFile(fileURI)
		s.dropFileCaches(fileURI)

		return watchedRemoved
	}

	if cfpath.IsBinary(data) {
		return watchedSkipped
	}

	s.applyIndexResult(fileURI, path, s.parseContentForIndex(fileURI, string(data)))
	s.dropFileCaches(fileURI)

	return watchedIndexed
}

// dropFileCaches clears the per-file caches that survive independently of the
// index, so a closed file's stale parse or completion items cannot outlive the
// change that invalidated them.
func (s *Server) dropFileCaches(fileURI uri.URI) {
	s.mu.Lock()
	delete(s.parseResults, fileURI)
	delete(s.funcRanges, fileURI)
	s.mu.Unlock()

	s.compCache.Invalidate(fileURI)
}

// isApplicationFile reports whether path is an Application.cfc/.cfm, whose
// mappings, beans and ORM locations cfpath caches per directory.
func isApplicationFile(path string) bool {
	base := filepath.Base(path)

	return strings.EqualFold(base, "Application.cfc") || strings.EqualFold(base, "Application.cfm")
}
