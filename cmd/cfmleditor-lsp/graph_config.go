package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/daemon"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

// configSet resolves each file against the .cfmleditor.json that actually governs
// it, rather than against one config chosen at the start of the scan.
//
// A TASS-shaped workspace is a dozen applications side by side, each with its own
// config: kiosk declares 52 componentResolvers, tassreporting six, and tassweb its
// own set again. Every one of them lists the others in workspacePaths, so a scan
// rooted anywhere reads all of them — under whichever single config was found
// first. The other applications' resolvers then never fire, their calls resolve to
// nothing, and their functions come out of the map with no edges: 11,387 of 11,682
// functions in the sibling applications had no caller, which reads as a dead
// codebase rather than as a scan configured for somebody else's code.
//
// Nothing about this is TASS-specific. Any monorepo with per-package settings has
// the same shape.
//
// The index is deliberately shared across every config. Function signatures are a
// property of the workspace and do not change with whose resolvers you read them
// under; duplicating it per config would multiply the memory and let the same file
// resolve differently depending on which application asked.
type configSet struct {
	fsys   vfs.FS
	shared *index.Index

	mu       sync.RWMutex
	byDir    map[string]codemap.FileConfig // directory → governing config
	byPath   map[string]codemap.FileConfig // config file path → built environment
	fallback codemap.FileConfig
	seen     map[string]string // config path → content hash, for the fingerprint
}

func newConfigSet(fsys vfs.FS, shared *index.Index, fallback codemap.FileConfig) *configSet {
	return &configSet{
		fsys:     fsys,
		shared:   shared,
		byDir:    make(map[string]codemap.FileConfig),
		byPath:   make(map[string]codemap.FileConfig),
		fallback: fallback,
		seen:     make(map[string]string),
	}
}

// For returns the resolution environment for one file, memoised per directory.
// It is called once per file from the scan's worker goroutines.
func (cs *configSet) For(file string) codemap.FileConfig {
	dir := filepath.Dir(file)

	cs.mu.RLock()
	cfg, ok := cs.byDir[dir]
	cs.mu.RUnlock()

	if ok {
		return cfg
	}

	cfg = cs.build(dir)

	cs.mu.Lock()
	cs.byDir[dir] = cfg
	cs.mu.Unlock()

	return cfg
}

func (cs *configSet) build(dir string) codemap.FileConfig {
	found, _ := daemon.FindConfig(dir)
	if found == nil {
		return cs.fallback
	}

	cs.mu.RLock()
	cfg, ok := cs.byPath[found.Path]
	cs.mu.RUnlock()

	if ok {
		return cfg
	}

	var resolvers []parser.Resolver
	for _, r := range found.ComponentResolvers() {
		resolvers = append(resolvers, parser.Resolver{
			Match: r.Match, Resolve: r.Resolve, Prefix: r.Prefix,
			NoFollow: r.NoFollow, Anchored: r.Anchored,
		})
	}

	cfg = codemap.FileConfig{
		Resolver: &resolve.Resolver{
			FS:                 cs.fsys,
			Index:              cs.shared,
			Resolvers:          resolvers,
			Mappings:           found.Mappings(),
			ExpressionMappings: found.ExpressionMappings(),
			WorkspaceFolders:   found.WorkspaceFolders(),
		},
		Resolvers:                resolvers,
		ExpressionMappings:       found.ExpressionMappings(),
		ServicePropertyResolvers: found.ServicePropertyResolvers(),
	}

	hash := ""

	if data, err := os.ReadFile(found.Path); err == nil {
		sum := sha256.Sum256(data)
		hash = hex.EncodeToString(sum[:])
	}

	cs.mu.Lock()
	cs.byPath[found.Path] = cfg
	cs.seen[found.Path] = hash
	cs.mu.Unlock()

	return cfg
}

// Fingerprint identifies every config used so far, so a cached edge set is not
// served after the resolvers that produced it change. Call it after the scan.
func (cs *configSet) Fingerprint() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	paths := make([]string, 0, len(cs.seen))
	for p := range cs.seen {
		paths = append(paths, p)
	}

	sort.Strings(paths)

	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "%s=%s\n", p, cs.seen[p])
	}

	return b.String()
}

// Configs names the config files in play, for the scan's own report.
func (cs *configSet) Configs() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	paths := make([]string, 0, len(cs.seen))
	for p := range cs.seen {
		paths = append(paths, p)
	}

	sort.Strings(paths)

	return paths
}

// preload builds the environment for every directory that holds a .cfmleditor.json
// under the scan roots, before the scan starts.
//
// Two reasons it is not left to happen lazily. The fingerprint has to cover every
// config the build *could* use, and one discovered late would not be in the
// fingerprint that keyed the cache writes already made. And discovering them up
// front is what lets the command tell you which configs it is about to use, which
// is the difference between a surprising result and an explained one.
func (cs *configSet) preload(roots []string) {
	for _, root := range roots {
		//nolint:nilerr // an unreadable entry is skipped, not fatal: a preload that
		// aborted on one permission error would silently fall back to a single
		// config for the whole scan, which is the bug this exists to prevent.
		_ = cs.fsys.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			if filepath.Base(path) == ".cfmleditor.json" {
				cs.For(filepath.Join(filepath.Dir(path), "x.cfc"))
			}

			return nil
		})
	}
}
