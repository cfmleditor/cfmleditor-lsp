package server

import "github.com/cfmleditor/cfmleditor-lsp/internal/config"

// Settings is the per-session configuration a Server needs, gathered in one
// place so that every way of starting a session applies the same set.
//
// It exists because they did not. Daemon mode serves its first editor over
// stdio and every later editor over a socket, and the two were configured by
// separate code applying separate subsets of a twelve-argument parameter list.
// expressionMappings and servicePropertyResolvers reached the first editor and
// no other, so the same workspace resolved component paths differently
// depending on which window a file was opened in — and nothing reported it,
// because an unset mapping is indistinguishable from a genuinely dynamic
// expression.
//
// Adding a config key now means adding a field here and a line in Apply,
// rather than remembering three call sites.
type Settings struct {
	WorkspaceFolders         []string
	IndexGlobs               []string
	Mappings                 map[string]string
	ExpressionMappings       map[string]string
	ServicePropertyResolvers map[string]string
	ComponentResolvers       []config.Resolver
	PropertyResolvers        []config.PropResolver
	BeanPaths                map[string]string
	Formatting               config.ResolvedFormatting
	Linting                  bool
}

// Apply copies the settings onto a freshly created Server.
func (set Settings) Apply(s *Server) {
	s.WorkspaceFolders = set.WorkspaceFolders
	s.IndexGlobs = set.IndexGlobs
	s.Mappings = set.Mappings
	s.ExpressionMappings = set.ExpressionMappings
	s.ServicePropertyResolvers = set.ServicePropertyResolvers
	s.ComponentResolvers = append(s.ComponentResolvers, set.ComponentResolvers...)
	s.PropertyResolvers = append(s.PropertyResolvers, set.PropertyResolvers...)
	s.BeanPaths = set.BeanPaths
	s.Formatting = set.Formatting
	s.Linting = set.Linting
}
