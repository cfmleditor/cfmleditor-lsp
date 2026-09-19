package server

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	cflog "github.com/cfmleditor/cfmleditor-lsp/internal/log"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"go.lsp.dev/protocol"
)

// codeMapRequest is the optional argument to cfmleditor.generateCodeMap.
//
// Every field has a working default, so the command is useful with no arguments
// at all: an editor can bind it to a menu item and a person who has never read
// this struct gets a report of their workspace.
type codeMapRequest struct {
	// Level is "function" (the default), "call", "file" or "package".
	Level string `json:"level"`

	// Format is "html" (the default), "json", "dot", "mermaid" or "text".
	Format string `json:"format"`

	// Out is where to write. Relative paths resolve against the workspace root;
	// the default is .cfmleditor/codemap.<ext> beside the config.
	Out string `json:"out"`

	// Under scopes the map to a path prefix, keeping the nodes just outside it
	// that an edge crosses into.
	Under string `json:"under"`

	// From scopes the map to what these node ids reach.
	From []string `json:"from"`

	// Live keeps only what an entry point reaches; Detached only what none does.
	Live     bool `json:"live"`
	Detached bool `json:"detached"`

	// Open asks the editor to show the report once it is written.
	Open bool `json:"open"`
}

// handleGenerateCodeMap builds a whole-project map from the running server.
//
// It runs on its own goroutine and reports through window/showMessage, because a
// map of a large workspace takes seconds to build and an executeCommand that
// blocked for that long would freeze the editor — the LSP loop is single-threaded
// from the client's point of view.
//
// The build reuses the server's own index rather than making its own. That index
// is already current, maintained by didChange and the watched-file handler, and
// re-reading every .cfc to rebuild it is the more expensive half of a cold CLI
// run. What is left is the scan, which has to happen either way.
func (s *Server) handleGenerateCodeMap(params []protocol.LSPAny) (any, error) {
	req := parseCodeMapRequest(params)

	roots := s.searchRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("no workspace root to map; open a folder first")
	}

	out, err := s.codeMapOutputPath(req, roots[0])
	if err != nil {
		return nil, err
	}

	s.safeGo("generateCodeMap", func() {
		// context.Background for the same reason scanWorkspace uses it: this
		// goroutine outlives the handler, whose ctx is pooled and reset on return.
		ctx := context.Background()

		started := time.Now()

		s.notifyInfo(ctx, "Building code map…")

		m, err := s.buildCodeMap(req, roots)
		if err != nil {
			s.notifyError(ctx, "Code map failed: "+err.Error())

			return
		}

		if err := s.writeCodeMap(m, req, out); err != nil {
			s.notifyError(ctx, "Could not write "+out+": "+err.Error())

			return
		}

		s.log.Info("code map written",
			cflog.String("path", out),
			cflog.Int("nodes", len(m.Nodes)),
			cflog.Int("edges", len(m.Edges)))

		s.notifyInfo(ctx, fmt.Sprintf("Code map: %d nodes, %d edges in %s → %s",
			len(m.Nodes), len(m.Edges), time.Since(started).Round(time.Millisecond), out))

		if req.Open {
			external := true
			s.notify(ctx, protocol.MethodWindowShowDocument, &protocol.ShowDocumentParams{
				URI:      cfpath.ToURI(out),
				External: &external,
			})
		}
	})

	// The path is returned immediately so a client can watch for the file rather
	// than wait on the command.
	return map[string]any{"path": out, "started": true}, nil
}

func (s *Server) buildCodeMap(req codeMapRequest, roots []string) (*codemap.Map, error) {
	files := collectWorkspaceCFMLFiles(s, roots)
	if len(files) == 0 {
		return nil, fmt.Errorf("no CFML files under %s", strings.Join(roots, ", "))
	}

	// One resolution environment for the whole scan: unlike the CLI, a server
	// session is already governed by one config — the one it was started in — so
	// there is nothing to vary per file.
	cfg := codemap.FileConfig{
		Resolver:                 s.getResolver(),
		Resolvers:                s.cfResolvers(),
		ExpressionMappings:       s.ExpressionMappings,
		ServicePropertyResolvers: s.ServicePropertyResolvers,
		Routes:                   s.routeResolver(),
	}

	m := codemap.Build(codemap.Options{
		Root:                     roots[0],
		Files:                    files,
		FS:                       s.FS,
		Resolver:                 cfg.Resolver,
		Resolvers:                cfg.Resolvers,
		ExpressionMappings:       cfg.ExpressionMappings,
		ServicePropertyResolvers: cfg.ServicePropertyResolvers,
		EntryGlobs:               s.CodeMap.Entry,
		UtilityGlobs:             s.CodeMap.Utility,
		ConfigFor:                func(string) codemap.FileConfig { return cfg },
	})

	return applyCodeMapViews(m, req), nil
}

func applyCodeMapViews(m *codemap.Map, req codeMapRequest) *codemap.Map {
	if req.Under != "" {
		m = m.FilterUnder(req.Under)
	}

	switch {
	case len(req.From) > 0:
		m = m.Reachable(req.From)
	case req.Live:
		m = m.Reachable(nil)
	case req.Detached:
		m = m.Detached()
	}

	switch req.Level {
	case "call":
		m = m.CallGraph()
	case "file":
		m = m.Collapse(codemap.LevelFile)
	case "package":
		m = m.Collapse(codemap.LevelPackage)
	}

	return m
}

func (s *Server) writeCodeMap(m *codemap.Map, req codeMapRequest, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(out), err)
	}

	file, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("creating %s: %w", out, err)
	}

	defer func() { _ = file.Close() }()

	switch req.Format {
	case "json":
		err = m.WriteJSON(file)
	case "dot":
		err = m.WriteDOT(file)
	case "mermaid":
		err = m.WriteMermaid(file, 2000)
	case "text":
		err = m.WriteText(file, 20)
	default:
		err = m.WriteHTML(file, codemap.HTMLOptions{
			Title:       "Code map — " + filepath.Base(m.Root),
			HideUtility: s.CodeMap.HideUtility,
		})
	}

	if err != nil {
		return err
	}

	return file.Close()
}

// codeMapOutputPath resolves where to write, refusing anything outside the
// workspace.
//
// A command an editor can invoke with arbitrary arguments is a command that can
// be asked to write anywhere on the filesystem. Keeping it inside the workspace
// means the worst a bad argument does is leave a file the user can see and delete.
func (s *Server) codeMapOutputPath(req codeMapRequest, root string) (string, error) {
	out := req.Out
	if out == "" {
		return filepath.Join(root, ".cfmleditor", "codemap"+codeMapExt(req.Format)), nil
	}

	if !filepath.IsAbs(out) {
		out = filepath.Join(root, out)
	}

	abs, err := filepath.Abs(out)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", req.Out, err)
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolving the workspace root: %w", err)
	}

	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to write outside the workspace: %s", req.Out)
	}

	return abs, nil
}

func codeMapExt(format string) string {
	switch format {
	case "json":
		return ".json"
	case "dot":
		return ".dot"
	case "mermaid":
		return ".md"
	case "text":
		return ".txt"
	default:
		return ".html"
	}
}

// parseCodeMapRequest decodes the optional options object.
//
// An LSPAny is raw JSON, not a decoded map, so this unmarshals rather than type
// asserts — and every field keeps its default when the argument is absent,
// malformed, or holds only some of the keys. A command an editor binds to a menu
// item has to work when invoked with nothing at all.
func parseCodeMapRequest(params []protocol.LSPAny) codeMapRequest {
	req := codeMapRequest{Level: "function", Format: "html"}

	if len(params) == 0 {
		return req
	}

	var opts struct {
		Level    *string  `json:"level"`
		Format   *string  `json:"format"`
		Out      *string  `json:"out"`
		Under    *string  `json:"under"`
		From     []string `json:"from"`
		Live     *bool    `json:"live"`
		Detached *bool    `json:"detached"`
		Open     *bool    `json:"open"`
	}

	if err := json.Unmarshal(params[0], &opts); err != nil {
		return req
	}

	setString := func(from *string, into *string) {
		if from != nil && *from != "" {
			*into = *from
		}
	}

	setString(opts.Level, &req.Level)
	setString(opts.Format, &req.Format)
	setString(opts.Out, &req.Out)
	setString(opts.Under, &req.Under)

	if opts.Live != nil {
		req.Live = *opts.Live
	}

	if opts.Detached != nil {
		req.Detached = *opts.Detached
	}

	if opts.Open != nil {
		req.Open = *opts.Open
	}

	for _, id := range opts.From {
		if id != "" {
			req.From = append(req.From, id)
		}
	}

	return req
}

// collectWorkspaceCFMLFiles walks the roots for CFML files, skipping the
// directories a scan has no business in — the same rule collectCFMLFiles uses in
// the CLI, kept here rather than shared because the server walks through vfs.FS.
func collectWorkspaceCFMLFiles(s *Server, roots []string) []string {
	var files []string

	for _, root := range roots {
		//nolint:nilerr // an unreadable entry is skipped, not fatal: aborting the
		// walk on one permission error would silently map a fraction of the
		// workspace and report it as the whole thing.
		_ = s.FS.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				if path != root && skipScanDir(info.Name()) {
					return filepath.SkipDir
				}

				return nil
			}

			if cfpath.IsCFMLFile(path) {
				files = append(files, path)
			}

			return nil
		})
	}

	return files
}

// skipScanDir mirrors the CLI's rule: every dot-directory, plus the usual
// dependency folders. A git worktree inside the project is a complete second copy
// of the codebase and doubles every count in the map.
func skipScanDir(name string) bool {
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}

	switch name {
	case "node_modules", "target", "vendor", "bower_components":
		return true
	default:
		return false
	}
}

func (s *Server) notifyInfo(ctx context.Context, msg string) {
	s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
		Type: protocol.MessageTypeInfo, Message: msg,
	})
}

func (s *Server) notifyError(ctx context.Context, msg string) {
	s.notify(ctx, protocol.MethodWindowShowMessage, &protocol.ShowMessageParams{
		Type: protocol.MessageTypeError, Message: msg,
	})
}

// handleCodeMapStats answers the cheap question — how much of this workspace does
// the map actually resolve — without writing anything.
//
// It exists because the expensive command's output is only worth as much as its
// resolution rate, and a reader has no way to judge a picture without that number.
// Same build, no file.
func (s *Server) handleCodeMapStats(ctx context.Context, params []protocol.LSPAny) (any, error) {
	roots := s.searchRoots()
	if len(roots) == 0 {
		return nil, fmt.Errorf("no workspace root to map; open a folder first")
	}

	m, err := s.buildCodeMap(parseCodeMapRequest(params), roots)
	if err != nil {
		return nil, err
	}

	resolved := 0.0
	if m.Stats.CallSites > 0 {
		resolved = float64(m.Stats.Resolved) * 100 / float64(m.Stats.CallSites)
	}

	routes := 0.0
	if m.Stats.Routes > 0 {
		routes = float64(m.Stats.Routes-m.Stats.RoutesUnresolved) * 100 / float64(m.Stats.Routes)
	}

	s.notifyInfo(ctx, fmt.Sprintf(
		"%d nodes, %d edges · %.1f%% of %d calls resolved · %.1f%% of %d routes · %d detached",
		len(m.Nodes), len(m.Edges), resolved, m.Stats.CallSites, routes, m.Stats.Routes, m.Stats.Detached))

	return map[string]any{
		"nodes": len(m.Nodes), "edges": len(m.Edges),
		"stats": m.Stats, "resolvedPercent": resolved, "routesPercent": routes,
	}, nil
}
