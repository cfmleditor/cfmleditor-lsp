package server

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/knownissues"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Diagnostic sources. A publish for a file replaces everything the server has
// published for it, so each source's diagnostics are kept apart and a file is
// always published with all of them.
const (
	sourceCFLint = "cflint"
	sourceParse  = "parse"
	// knownIssuesSource prefixes a known-issues file's own source key, one per
	// configured file, so reloading one leaves the others alone.
	knownIssuesSource = "known:"
)

// diagnosticStore is every diagnostic the session has published, per file and
// per source.
//
// Before it, CFLint on save, the workspace parse scan and didClose each
// published straight to the client, and each publish replaced whatever the
// others had sent for that file: saving a file wiped its parse errors, and a
// scan wiped its lint results. Known issues made that untenable, since they are
// published for files nobody has opened and have to survive every other source
// coming and going.
type diagnosticStore struct {
	mu    sync.Mutex
	files map[string]*fileDiagnostics // by diagKey
	// known holds, per known-issues source, the files it last published to, so
	// a reload clears the ones an edit removed.
	known map[string][]string
	// openOnly marks a source published only for open files, and open the
	// files the client has open, by diagKey: tracked here rather than read from
	// the document map, which is keyed by the client's URI spelling.
	openOnly map[string]bool
	open     map[string]bool
	// supersededBy names, for a generated known-issues source, the live source
	// that replaces it file by file: a cflint report's entries give way to
	// CFLint's own run on save, so an issue is never listed from both.
	supersededBy map[string]string
}

type fileDiagnostics struct {
	uri      uri.URI // the spelling to publish under
	bySource map[string][]protocol.Diagnostic
	// ran holds the live sources that have run on the file since the reports
	// they supersede were loaded. A clean run leaves no diagnostics, and still
	// hides the report's, which it has just shown to be stale.
	ran map[string]bool
}

// diagKey is a file's identity across the URI spellings the client and the
// server produce for it.
func diagKey(u uri.URI) string {
	return filepath.Clean(cfpath.FromURI(string(u)))
}

// setDiagnostics replaces one source's diagnostics for a file and publishes the
// file's merged set. Empty d removes the source.
func (s *Server) setDiagnostics(ctx context.Context, u uri.URI, source string, d []protocol.Diagnostic) {
	s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, s.storeDiagnostics(u, source, d))
}

// storeDiagnostics is setDiagnostics without the publish: it records d and
// returns what the file's publish must carry.
func (s *Server) storeDiagnostics(u uri.URI, source string, d []protocol.Diagnostic) *protocol.PublishDiagnosticsParams {
	st := &s.diag
	key := diagKey(u)

	st.mu.Lock()
	defer st.mu.Unlock()

	if st.files == nil {
		st.files = make(map[string]*fileDiagnostics)
	}

	f := st.files[key]
	if f == nil {
		f = &fileDiagnostics{uri: u, bySource: make(map[string][]protocol.Diagnostic)}
		st.files[key] = f
	}

	// A source reporting on a document the client opened names it the
	// client's way; a known issue for an unopened file has only ours.
	if !strings.HasPrefix(source, knownIssuesSource) {
		f.uri = u
	}

	if len(d) == 0 {
		delete(f.bySource, source)
	} else {
		f.bySource[source] = d
	}

	merged := st.mergedLocked(key, f)
	if len(f.bySource) == 0 && len(f.ran) == 0 {
		delete(st.files, key)
	}

	return &protocol.PublishDiagnosticsParams{URI: f.uri, Diagnostics: merged}
}

// mergedLocked is what the client should hold for f: every source's
// diagnostics in a stable source order, less an open-only source's while the
// file is closed.
func (st *diagnosticStore) mergedLocked(key string, f *fileDiagnostics) []protocol.Diagnostic {
	sources := make([]string, 0, len(f.bySource))
	for src := range f.bySource {
		if st.openOnly[src] && !st.open[key] {
			continue
		}

		if live := st.supersededBy[src]; live != "" && f.ran[live] {
			continue
		}

		sources = append(sources, src)
	}

	slices.Sort(sources)

	merged := []protocol.Diagnostic{}
	for _, src := range sources {
		merged = append(merged, f.bySource[src]...)
	}

	return merged
}

// diagnosticsRan records that the live source has run on u, so a generated
// report it supersedes stops publishing there. Call it before the run's own
// setDiagnostics, whose publish then leaves the report's entries out.
func (s *Server) diagnosticsRan(u uri.URI, source string) {
	st := &s.diag
	key := diagKey(u)

	st.mu.Lock()
	defer st.mu.Unlock()

	if st.files == nil {
		st.files = make(map[string]*fileDiagnostics)
	}

	f := st.files[key]
	if f == nil {
		f = &fileDiagnostics{uri: u, bySource: make(map[string][]protocol.Diagnostic)}
		st.files[key] = f
	}

	if f.ran == nil {
		f.ran = make(map[string]bool)
	}

	f.ran[source] = true
}

// diagnosticsFor is the merged set the server last published for u.
func (s *Server) diagnosticsFor(u uri.URI) []protocol.Diagnostic {
	st := &s.diag
	key := diagKey(u)

	st.mu.Lock()
	defer st.mu.Unlock()

	f := st.files[key]
	if f == nil {
		return nil
	}

	return st.mergedLocked(key, f)
}

// diagnosticsOpened records that the client opened u and, when an open-only
// source has diagnostics for it, publishes them.
func (s *Server) diagnosticsOpened(ctx context.Context, u uri.URI) {
	st := &s.diag
	key := diagKey(u)

	st.mu.Lock()

	if st.open == nil {
		st.open = make(map[string]bool)
	}

	st.open[key] = true

	var params *protocol.PublishDiagnosticsParams

	if f := st.files[key]; f != nil {
		for src := range f.bySource {
			if st.openOnly[src] {
				f.uri = u
				params = &protocol.PublishDiagnosticsParams{URI: u, Diagnostics: st.mergedLocked(key, f)}

				break
			}
		}
	}

	st.mu.Unlock()

	if params != nil {
		s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, params)
	}
}

// diagnosticsClosed records that the client closed u. The close handler's own
// publish, after it clears the document's sources, then leaves open-only ones
// out.
func (s *Server) diagnosticsClosed(u uri.URI) {
	st := &s.diag

	st.mu.Lock()
	defer st.mu.Unlock()

	delete(st.open, diagKey(u))
}

// isDiagnosticsOpen reports whether the client has u open.
func (s *Server) isDiagnosticsOpen(u uri.URI) bool {
	st := &s.diag

	st.mu.Lock()
	defer st.mu.Unlock()

	return st.open[diagKey(u)]
}

// hasDiagnostics reports whether source has diagnostics recorded for u.
func (s *Server) hasDiagnostics(u uri.URI, source string) bool {
	st := &s.diag

	st.mu.Lock()
	defer st.mu.Unlock()

	f := st.files[diagKey(u)]

	return f != nil && len(f.bySource[source]) > 0
}

// hasKnownIssues reports whether any known-issues file publishes to u.
func (s *Server) hasKnownIssues(u uri.URI) bool {
	st := &s.diag

	st.mu.Lock()
	defer st.mu.Unlock()

	f := st.files[diagKey(u)]
	if f == nil {
		return false
	}

	for src := range f.bySource {
		if strings.HasPrefix(src, knownIssuesSource) {
			return true
		}
	}

	return false
}

// isKnownIssuesFile reports whether path is one of the configured files.
func (s *Server) isKnownIssuesFile(path string) bool {
	clean := filepath.Clean(path)

	for _, k := range s.KnownIssues {
		if k.File == clean {
			return true
		}
	}

	return false
}

// loadKnownIssues reads every configured known-issues file and publishes its
// entries, clearing any file an earlier load published to that this one does
// not. A file that cannot be read publishes nothing and is logged.
func (s *Server) loadKnownIssues(ctx context.Context) {
	for _, k := range s.KnownIssues {
		s.loadKnownIssuesFile(ctx, k.File)
	}
}

func (s *Server) loadKnownIssuesFile(ctx context.Context, file string) {
	i := slices.IndexFunc(s.KnownIssues, func(k config.KnownIssues) bool { return k.File == file })
	if i < 0 {
		return
	}

	cfg := s.KnownIssues[i]
	lint := cfg.IsGenerated(config.GenerateCFLint)

	label := cfg.Source
	if label == "" {
		label = "known issue"

		// Labelled as CFLint labels its own, so an entry reads the same
		// whether the report or a run on save produced it.
		if lint {
			label = sourceCFLint
		}
	}

	source := knownIssuesSource + file
	openOnly := !strings.EqualFold(strings.TrimSpace(cfg.Scope), "workspace")

	s.diag.mu.Lock()
	if s.diag.openOnly == nil {
		s.diag.openOnly = make(map[string]bool)
	}

	if s.diag.supersededBy == nil {
		s.diag.supersededBy = make(map[string]string)
	}

	s.diag.openOnly[source] = openOnly
	delete(s.diag.supersededBy, source)

	if lint {
		s.diag.supersededBy[source] = sourceCFLint

		// A fresh report is as new as any run on a closed file, whose results
		// are gone. An open file keeps its run's, which the report would only
		// repeat.
		for key, f := range s.diag.files {
			if !s.diag.open[key] {
				delete(f.ran, sourceCFLint)
			}
		}
	}

	s.diag.mu.Unlock()

	byFile := map[string][]protocol.Diagnostic{}
	uris := map[string]uri.URI{}

	if data, err := s.FS.ReadFile(file); errors.Is(err, fs.ErrNotExist) {
		// An implicit report that has not been generated yet, or one deleted:
		// nothing to publish, and whatever it published before is cleared below.
		s.log.Debug("known issues file absent", cflog.String("file", file))
	} else if err != nil {
		s.log.Warn("known issues file not read", cflog.String("file", file), cflog.Err(err))
	} else {
		severity := knownissues.Severity(cfg.Severity)
		lines := map[string][]string{}

		for _, e := range knownissues.Parse(string(data), filepath.Dir(file)) {
			src, ok := lines[e.Path]
			if !ok {
				if b, err := s.FS.ReadFile(e.Path); err == nil {
					src = strings.Split(string(b), "\n")
				}

				lines[e.Path] = src
			}

			u := uri.File(e.Path)
			key := diagKey(u)
			uris[key] = u
			byFile[key] = append(byFile[key], knownissues.Diagnostic(e, src, severity, label))
		}
	}

	s.diag.mu.Lock()
	previous := s.diag.known[source]
	s.diag.mu.Unlock()

	// An open-only source reaches the client only for an open file, so for a
	// closed one the store is updated and nothing is sent.
	publish := func(u uri.URI, d []protocol.Diagnostic) {
		params := s.storeDiagnostics(u, source, d)
		if !openOnly || s.isDiagnosticsOpen(u) {
			s.notify(ctx, protocol.MethodTextDocumentPublishDiagnostics, params)
		}
	}

	for _, key := range previous {
		if _, still := byFile[key]; !still {
			publish(uri.File(key), nil)
		}
	}

	keys := make([]string, 0, len(byFile))
	for key, d := range byFile {
		publish(uris[key], d)
		keys = append(keys, key)
	}

	s.diag.mu.Lock()
	if s.diag.known == nil {
		s.diag.known = make(map[string][]string)
	}

	s.diag.known[source] = keys
	s.diag.mu.Unlock()

	s.log.Info("known issues published", cflog.String("file", file), cflog.Int("files", len(keys)))
}
