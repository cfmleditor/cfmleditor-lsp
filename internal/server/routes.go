package server

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/route"
	"go.lsp.dev/protocol"
)

// routeResolver returns the resolver for the configured routing convention, or
// nil when no convention is configured.
//
// It is memoised behind the same invalidation as the component resolvers: the
// lookups close over the index and the workspace roots, so a resolver built
// before a reindex would answer from a workspace that no longer exists.
func (s *Server) routeResolver() *route.Resolver {
	s.routeMu.Lock()
	defer s.routeMu.Unlock()

	if s.cachedRoutes != nil {
		return s.cachedRoutes
	}

	if !s.Routes.Enabled() {
		return nil
	}

	roots := s.searchRoots()

	var (
		dirMu sync.Mutex
		dirs  = map[string]bool{}
	)

	s.cachedRoutes = &route.Resolver{
		Config: s.Routes,
		Lookups: route.Lookups{
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

// routeAtPosition returns the route reference the cursor is inside, if any.
//
// The scan is over the whole document rather than the cursor's line, because an
// attribute value can be split across lines and the cheap single-line version
// would silently stop working on exactly the long attribute lists where a link
// helps most.
func (s *Server) routeAtPosition(content string, line, char int) (route.Ref, bool) {
	r := s.routeResolver()
	if r == nil {
		return route.Ref{}, false
	}

	for _, ref := range route.Scan(content, r.Config) {
		if int(ref.Line) != line {
			continue
		}

		start := int(ref.Col)
		if char >= start && char <= start+len(ref.Value) {
			return ref, true
		}
	}

	return route.Ref{}, false
}

// routeDefinitions turns a route into go-to-definition locations.
//
// Several locations rather than one when a route resolves into several products:
// which one serves a shared page is a runtime fact, and an editor showing a
// picker is a truer answer than silently jumping to whichever happened to sort
// first.
func (s *Server) routeDefinitions(ref route.Ref) []protocol.Location {
	r := s.routeResolver()
	if r == nil || !route.Plausible(ref.Value) {
		return nil
	}

	var out []protocol.Location

	for _, t := range r.Resolve(ref.Value) {
		loc := protocol.Location{URI: cfpath.ToURI(t.Path)}

		if t.Kind == route.KindController && t.Method != "" {
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

	for _, ref := range route.Scan(docContent, r.Config) {
		if !route.Plausible(ref.Value) {
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
				Start: protocol.Position{Line: ref.Line, Character: ref.Col},
				End:   protocol.Position{Line: ref.Line, Character: ref.Col + uint32(len(ref.Value))},
			},
			Target:  targetRef,
			Tooltip: &tip,
		})
	}

	return links
}

// linkTarget picks the one destination a document link can have.
//
// A route resolving to both a controller method and a view is not ambiguous, it
// is complete — the method renders the template — so the controller wins, because
// that is where the code is. Genuine ambiguity is several *controllers*, which is
// a shared view reaching into more than one product; there a link would send the
// reader to the wrong one without saying so, and go-to-definition handles it
// instead by offering every location.
func linkTarget(targets []route.Target) (route.Target, bool) {
	var (
		controllers []route.Target
		views       []route.Target
	)

	for _, t := range targets {
		if t.Kind == route.KindController {
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

	return route.Target{}, false
}

func routeTooltip(t route.Target) string {
	if t.Kind == route.KindController && t.Method != "" {
		return t.Component + "." + t.Method + "()"
	}

	return filepath.Base(t.Path)
}
