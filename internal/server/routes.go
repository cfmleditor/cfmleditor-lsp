package server

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	routepkg "github.com/cfmleditor/cfmleditor-lsp/internal/route"
	"go.lsp.dev/protocol"
)

// routeResolver returns the resolver for the configured routing convention, or
// nil when no convention is configured.
//
// It is memoised behind the same invalidation as the component resolvers: the
// lookups close over the index and the workspace roots, so a resolver built
// before a reindex would answer from a workspace that no longer exists.
func (s *Server) routeResolver() *routepkg.Resolver {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()

	if s.cachedRoutes != nil {
		return s.cachedRoutes
	}

	// The switch is here rather than at each caller for the reason the
	// watched-files one documents: a gate in the caller is one a direct call
	// slips past, and four handlers reach routes through this single function.
	if !s.Features.Routes || !s.Routes.Enabled() {
		return nil
	}

	roots := s.searchRoots()

	var (
		dirMu sync.Mutex
		dirs  = map[string]bool{}
	)

	s.cachedRoutes = &routepkg.Resolver{
		Config: s.Routes,
		Lookups: routepkg.Lookups{
			ComponentPath: func(component string) string {
				// One base directory for the whole workspace, not the directory of
				// the file the route was written in. A route names a component
				// globally — the same route means the same thing wherever it is
				// written — so resolving it relative to each file would make a
				// shared view resolve differently depending on which page included
				// it.
				//
				// The consequence is that a controller template should be written
				// through a mapping ("tassweb.packages.tass.${1}") rather than as a
				// path relative to the workspace root ("packages.tass.${1}"). Both
				// work when the root is the application; only the mapped form keeps
				// working for a route in a sibling application's file.
				base := ""
				if len(roots) > 0 {
					base = roots[0]
				}

				return s.getResolver().ComponentPath(component, base)
			},
			HasMethod: func(cfcPath, method string) bool {
				for _, fn := range s.getResolver().EnsureIndexed(cfcPath) {
					if strings.EqualFold(fn.Name, method) {
						return true
					}
				}

				return false
			},
			DirExists: func(rel string) bool {
				dirMu.Lock()
				defer dirMu.Unlock()

				if v, ok := dirs[rel]; ok {
					return v
				}

				found := false

				for _, r := range roots {
					if info, err := os.Stat(filepath.Join(r, filepath.FromSlash(rel))); err == nil && info.IsDir() {
						found = true

						break
					}
				}

				dirs[rel] = found

				return found
			},
			FindFile: func(rel string) string {
				for _, r := range roots {
					p := filepath.Join(r, filepath.FromSlash(rel))
					if info, err := os.Stat(p); err == nil && !info.IsDir() {
						return p
					}
				}

				return ""
			},
		},
	}

	return s.cachedRoutes
}

// invalidateRoutes drops the memoised route resolver. Called wherever the
// component resolvers are invalidated, for the same reason.
func (s *Server) invalidateRoutes() {
	s.routeMu.Lock()
	s.cachedRoutes = nil
	s.routeMu.Unlock()
}

// routeWindow is how many lines either side of the cursor are scanned for a
// route reference.
//
// The scan used to cover the whole document, on the reasoning that an attribute
// value can be split across lines and a single-line scan would miss it. The
// first half is true and the conclusion does not follow: only a ref whose
// recorded line *is* the cursor's line can match below, so every other line
// scanned was work thrown away. On a 23,687-line component that was 380ms of a
// 389ms go-to-definition — 98% of the request — repeated for every one.
//
// A window keeps what the whole-document scan was protecting: an attribute
// opening this many lines above the cursor is still seen, so a value split
// across a long attribute list still resolves. Fifty lines is far more than any
// real tag spans and is 0.2% of that file.
const routeWindow = 50

// routeAtPosition returns the route reference the cursor is inside, if any.
func (s *Server) routeAtPosition(content string, line, char int) (routepkg.Ref, bool) {
	r := s.routeResolver()
	if r == nil {
		return routepkg.Ref{}, false
	}

	window, baseLine := linesAround(content, line, routeWindow)
	if window == "" {
		return routepkg.Ref{}, false
	}

	want := line - baseLine

	for _, ref := range routepkg.Scan(window, r.Config) {
		if int(ref.Line) != want {
			continue
		}

		start := int(ref.Col)
		if char >= start && char <= start+len(ref.Value) {
			// Reported against the document, not the window.
			ref.Line = uint32(line)

			return ref, true
		}
	}

	return routepkg.Ref{}, false
}

// linesAround returns the text of the lines within n of line, and the number of
// the first line it starts at, so a caller can map a position back.
func linesAround(content string, line, n int) (window string, baseLine int) {
	first := max(line-n, 0)

	start, cur := 0, 0

	for cur < first {
		i := strings.IndexByte(content[start:], '\n')
		if i < 0 {
			return "", 0
		}

		start += i + 1
		cur++
	}

	end := start

	// Up to the cursor's line plus n, counted from where the window actually
	// starts. Ranging over 2n+1 lines instead overshoots whenever the start was
	// clamped to the top of the file, which is harmless but means the window is
	// not the range its own name describes.
	for range line + n - first + 1 {
		i := strings.IndexByte(content[end:], '\n')
		if i < 0 {
			end = len(content)

			break
		}

		end += i + 1
	}

	return content[start:end], first
}

// routeDefinitions turns a route into go-to-definition locations.
//
// Several locations rather than one when a route resolves into several products:
// which one serves a shared page is a runtime fact, and an editor showing a
// picker is a truer answer than silently jumping to whichever happened to sort
// first.
func (s *Server) routeDefinitions(ref routepkg.Ref) []protocol.Location {
	r := s.routeResolver()
	if r == nil || !routepkg.Plausible(ref.Value) {
		return nil
	}

	var out []protocol.Location

	for _, t := range r.Resolve(ref.Value) {
		loc := protocol.Location{URI: cfpath.ToURI(t.Path)}

		if t.Kind == routepkg.KindController && t.Method != "" {
			if fn := s.findMethodPosition(t.Path, t.Method); fn != nil {
				loc.Range = *fn
			}
		}

		out = append(out, loc)
	}

	return out
}

// findMethodPosition locates a method inside a component so the jump lands on the
// declaration rather than on line one of a ten-thousand-line controller.
func (s *Server) findMethodPosition(cfcPath, method string) *protocol.Range {
	for _, fn := range s.getResolver().EnsureIndexed(cfcPath) {
		if !strings.EqualFold(fn.Name, method) {
			continue
		}

		return &protocol.Range{
			Start: protocol.Position{Line: fn.Line},
			End:   protocol.Position{Line: fn.Line},
		}
	}

	return nil
}

// routeLinks returns a document link for every resolvable route in the document,
// so the whole file's routes are ctrl-clickable rather than only the one under
// the cursor.
//
// A route that resolves to more than one target gets no link: a link has exactly
// one destination, and picking one of several would send the reader to the wrong
// product without saying so. Go-to-definition handles the ambiguous ones, where
// the editor can offer the choice.
func (s *Server) routeLinks(docContent string) []protocol.DocumentLink {
	r := s.routeResolver()
	if r == nil {
		return nil
	}

	var links []protocol.DocumentLink

	cols := newColMapper(docContent)

	for _, ref := range s.scanDocumentRoutes(docContent, r) {
		if !routepkg.Plausible(ref.Value) {
			continue
		}

		best, ok := linkTarget(r.Resolve(ref.Value))
		if !ok {
			continue
		}

		target := cfpath.ToURI(best.Path)
		tip := routeTooltip(best)
		targetRef := &target

		links = append(links, protocol.DocumentLink{
			Range: protocol.Range{
				Start: protocol.Position{Line: ref.Line, Character: cols.col(ref.Line, ref.Col)},
				End:   protocol.Position{Line: ref.Line, Character: cols.col(ref.Line, ref.Col+uint32(len(ref.Value)))}, //nolint:gosec // a route is one attribute value
			},
			Target:  targetRef,
			Tooltip: &tip,
		})
	}

	return links
}

// scanDocumentRoutes is the whole-document route scan, memoised against the
// content it was taken from.
//
// Document links are the one route caller that genuinely needs every reference
// in the file, so the cursor window that made go-to-definition cheap does not
// apply. What does apply is that the editor asks for links repeatedly on a
// document that has not changed — on open, on focus, after any request that
// refreshes them — and the scan is proportional to the file. On a 64,000-line
// component it is three seconds, and it was three seconds every time.
//
// Keyed on a hash of the content rather than a version number, because the
// version belongs to the document and this cache is keyed by nothing else: an
// edit that changes the text changes the key, and one that does not, does not.
// One entry, because the editor asks about the document in front of it.
func (s *Server) scanDocumentRoutes(content string, r *routepkg.Resolver) []routepkg.Ref {
	key := contentKey(content)

	s.routeMu.Lock()
	if s.routeScanKey == key {
		refs := s.routeScanRefs
		s.routeMu.Unlock()

		return refs
	}
	s.routeMu.Unlock()

	refs := routepkg.Scan(content, r.Config)

	s.routeMu.Lock()
	s.routeScanKey = key
	s.routeScanRefs = refs
	s.routeMu.Unlock()

	return refs
}

// contentKey identifies a document's text for the scan cache.
func contentKey(content string) string {
	sum := sha256.Sum256([]byte(content))

	return string(sum[:])
}

// linkTarget picks the one destination a document link can have.
//
// A route resolving to both a controller method and a view is not ambiguous, it
// is complete — the method renders the template — so the controller wins, because
// that is where the code is. Genuine ambiguity is several *controllers*, which is
// a shared view reaching into more than one product; there a link would send the
// reader to the wrong one without saying so, and go-to-definition handles it
// instead by offering every location.
func linkTarget(targets []routepkg.Target) (routepkg.Target, bool) {
	var (
		controllers []routepkg.Target
		views       []routepkg.Target
	)

	for _, t := range targets {
		if t.Kind == routepkg.KindController {
			controllers = append(controllers, t)
		} else {
			views = append(views, t)
		}
	}

	if len(controllers) == 1 {
		return controllers[0], true
	}

	if len(controllers) == 0 && len(views) == 1 {
		return views[0], true
	}

	return routepkg.Target{}, false
}

func routeTooltip(t routepkg.Target) string {
	if t.Kind == routepkg.KindController && t.Method != "" {
		return t.Component + "." + t.Method + "()"
	}

	return filepath.Base(t.Path)
}

// handleResolveRoute answers "where does this route go" for a route the caller
// supplies, rather than one found in a document.
//
// It exists so an editor command that takes a route by hand and go-to-definition
// on a route in source resolve through the same code. The alternative — an editor
// keeping its own copy of the convention — was the state of things before this,
// and two resolvers reading two different configs disagree quietly: the one that
// is wrong still opens a file, just not the right one.
func (s *Server) handleResolveRoute(params []protocol.LSPAny) (any, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("cfmleditor.resolveRoute requires a route")
	}

	route, _ := argString(params, 0)
	if route == "" {
		return nil, nil
	}

	r := s.routeResolver()
	if r == nil {
		return map[string]any{
			"route":   route,
			"targets": []any{},
			"reason":  "no routes convention is configured for this workspace",
		}, nil
	}

	if !routepkg.Plausible(route) {
		return map[string]any{
			"route":   route,
			"targets": []any{},
			"reason":  "not shaped like a route",
		}, nil
	}

	targets := r.Resolve(route)
	out := make([]any, 0, len(targets))

	for _, t := range targets {
		entry := map[string]any{
			"kind": string(t.Kind),
			"uri":  string(cfpath.ToURI(t.Path)),
			"path": t.Path,
		}

		if t.Component != "" {
			entry["component"] = t.Component
		}

		if t.Method != "" {
			entry["method"] = t.Method

			// The declaration line, so the caller reveals the method rather than
			// the top of a ten-thousand-line controller.
			if rng := s.findMethodPosition(t.Path, t.Method); rng != nil {
				entry["range"] = *rng
			}
		}

		if t.Alias != "" {
			entry["via"] = t.Alias
		}

		out = append(out, entry)
	}

	return map[string]any{"route": route, "targets": out}, nil
}
