// Package codemap builds a whole-project map of a CFML codebase: every function,
// every file, and every call, instantiation, inheritance and include between them.
//
// It is the inverse of [github.com/cfmleditor/cfmleditor-lsp/internal/deps], and
// deliberately so. deps.Build is a seeded breadth-first walk: give it one file or
// function and it expands outward, re-parsing each target on demand. Its node
// identity carries the call's line number ("Base.cfc (line 42)") so that repeated
// references stay distinct within one tree, which is right for one tree and wrong
// for a project — run it per file and the same method becomes a different node in
// every seed's graph, so tens of thousands of definitions become hundreds of
// thousands of nodes that nothing can join back up.
//
// So a whole-project map is not deps run N times. It is a single streaming pass
// over every file, emitting canonically-identified nodes and deduplicated edges —
// structurally the pass the `unresolved` command already makes, carrying edges
// instead of error records.
//
// Node identity is the file path, never the component dot-path. Dot-paths are
// many-to-one: mappings, per-directory resolution and expression substitution mean
// two spellings routinely name one file, and one spelling can name different files
// from different base directories. The file is the only thing that is actually the
// same thing. Dot-paths ride along as display labels.
package codemap

import (
	"path"
	"strings"
)

// NodeKind classifies what a node stands for.
type NodeKind string

// The kinds of node a map holds.
const (
	// KindFunction is one declared function or method.
	KindFunction NodeKind = "function"

	// KindFile is a whole file. It is both a container for its functions and the
	// caller of record for top-level code, which is most of what lives in a .cfm.
	KindFile NodeKind = "file"

	// KindExternal is a call that left the workspace: unresolved, or resolved only
	// to a component with no file behind it. Keeping these as nodes rather than
	// dropping them is deliberate — a map that silently omits what it could not
	// follow reads as a map of a smaller, tidier codebase than the real one.
	KindExternal NodeKind = "external"

	// KindPackage is a directory, produced only by collapsing.
	KindPackage NodeKind = "package"
)

// EdgeKind classifies a relationship.
type EdgeKind string

// The kinds of edge a map holds.
const (
	// EdgeCalls is a call site: one function (or a file's top-level code) invoking
	// another function.
	EdgeCalls EdgeKind = "calls"

	// EdgeInstantiates is a component reference — a new/createObject/cfinvoke that
	// names a component, whether or not a method was then called on it. A component
	// constructed and passed on is a dependency that no call edge records.
	EdgeInstantiates EdgeKind = "instantiates"

	// EdgeExtends is an inheritance edge, child to parent.
	EdgeExtends EdgeKind = "extends"

	// EdgeIncludes is a cfinclude or equivalent file reference.
	EdgeIncludes EdgeKind = "includes"

	// EdgeRoute is a framework route: a page naming a controller method or a view
	// that a dispatcher will reach at runtime. Nothing in the source calls either,
	// so without this edge every routed controller method looks uncalled and every
	// view unreferenced. Kept distinct from EdgeCalls because it *is* less certain
	// — it is a convention resolving, not a call site parsing.
	EdgeRoute EdgeKind = "route"

	// EdgeContains joins a file to the functions it declares. It is emitted only at
	// function level, where it is what gives a renderer the hierarchy to group by.
	EdgeContains EdgeKind = "contains"
)

// Node is one vertex of the map.
type Node struct {
	ID        string   `json:"id"`
	Kind      NodeKind `json:"kind"`
	Name      string   `json:"name"`
	File      string   `json:"file,omitempty"`      // slash-separated, relative to Map.Root
	Line      uint32   `json:"line,omitempty"`      // 0-based, as the parser reports it
	Access    string   `json:"access,omitempty"`    // public, private, remote, package
	Component string   `json:"component,omitempty"` // a dot-path this node is known by
	Entry     bool     `json:"entry,omitempty"`     // reachable from outside the codebase
	Abstract  bool     `json:"abstract,omitempty"`  // synthesised by a collapse, not parsed

	// Island is the connected component this node belongs to, over the undirected
	// dependency graph. Every declared function is in the map whether or not
	// anything calls it, so a function nothing reaches is not missing — it is an
	// island of its own, and this is the field that says so. A renderer lays each
	// island out as its own tree instead of piling the unreachable ones on the
	// origin; a reader sees a detached subsystem rather than a gap.
	//
	// Islands group a file together with the functions it declares, so a detached
	// island is a separate subsystem rather than a single uncalled helper. Whether
	// anything *calls* a given function is a different question, answered by
	// Reachable.
	Island int `json:"island"`

	// Reachable reports whether an entry point reaches this node: a .cfm page, a
	// remote method, or an Application.cfc lifecycle hook. False is a candidate for
	// deletion, not a verdict — an unresolved call is an edge the map does not
	// have, so read it against Stats.Unresolved.
	Reachable bool `json:"reachable,omitempty"`

	// Root marks a node nothing in its island depends on, which is where a tree
	// view of that island starts. An island made entirely of a cycle has no such
	// node, so its lowest id is named root to keep it drawable.
	Root bool `json:"root,omitempty"`

	// Utility marks a node as infrastructure rather than application code: the
	// logging, the PDF writer, the context accessor that every request touches.
	//
	// It is a label, never a filter. These are genuinely the most-depended-on code
	// in a workspace — one context accessor had 1,439 callers — so they dominate
	// every ranking and tie every part of the graph to every other part, and a map
	// that hid them would be lying about what the code does. A map that cannot set
	// them aside on request is unreadable for a different reason. Marking, and
	// letting the reader toggle, is the only honest version.
	Utility bool `json:"utility,omitempty"`

	// Boundary marks a node that is outside a scoped view but kept because an edge
	// crosses into it. See [Map.FilterUnder]: scoping to a package and dropping
	// everything beyond it hides exactly the callers you scoped in order to find.
	Boundary bool `json:"boundary,omitempty"`
}

// Edge is one directed relationship, deduplicated across every call site that
// produced it. Count says how many there were.
type Edge struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Kind    EdgeKind `json:"kind"`
	Count   int      `json:"count"`
	Dynamic bool     `json:"dynamic,omitempty"` // target is a guess: $any, noFollow, onMissingMethod
}

// Stats records what the build saw. The resolved/dynamic/unresolved split is the
// map's own confidence report: a project whose calls are mostly unresolved has a
// mostly-fictional call graph, and the numbers say so rather than the picture
// implying otherwise.
type Stats struct {
	Files      int `json:"files"`
	Functions  int `json:"functions"`
	CallSites  int `json:"callSites"`
	Resolved   int `json:"resolved"`
	Dynamic    int `json:"dynamic"`
	Unresolved int `json:"unresolved"`
	Builtin    int `json:"builtin"`
	Unreadable int `json:"unreadable"`

	// Routes counts the framework route strings found, and RoutesUnresolved how
	// many named nothing the convention could reach. A high unresolved share means
	// the routes config describes a different convention from the one in use, and
	// the route edges are worth correspondingly less.
	Routes           int `json:"routes"`
	RoutesUnresolved int `json:"routesUnresolved"`

	// Utility counts the nodes marked as infrastructure.
	Utility int `json:"utility"`

	// Cached counts files served from a [Cache] instead of parsed. A rebuild where
	// this is near Files is the fast path working.
	Cached int `json:"cached"`

	// Islands counts the disconnected pieces of the dependency graph, and Detached
	// the nodes no entry point reaches. A healthy application is close to one
	// island; a large count means either genuinely dead subsystems or, more often,
	// calls the resolvers could not follow.
	Islands     int   `json:"islands"`
	Detached    int   `json:"detached"`
	BuildMillis int64 `json:"buildMillis"`
}

// Level is the granularity of a map.
type Level string

// The levels a map can be built or collapsed to.
const (
	// LevelFunction is one node per function, plus one per file for top-level code.
	LevelFunction Level = "function"

	// LevelFile is one node per file.
	LevelFile Level = "file"

	// LevelPackage is one node per directory. This is the level that actually
	// renders as a picture of a large project.
	LevelPackage Level = "package"

	// LevelCall is the strict call graph: function nodes and call edges, nothing
	// else. See [Map.CallGraph].
	LevelCall Level = "call"
)

// Map is the whole thing: what a build produced, or what a view of one holds.
type Map struct {
	Root  string `json:"root"`
	Level Level  `json:"level"`

	// Fingerprint identifies the resolution environment this map was built
	// against — every indexed component and its content. A [Cache] keys on it, and
	// a caller passes it to a store's PruneCache so the generations that can never
	// match again are dropped.
	Fingerprint string `json:"fingerprint,omitempty"`
	Nodes       []Node `json:"nodes"`
	Edges       []Edge `json:"edges"`
	Stats       Stats  `json:"stats"`
}

// FuncID is the canonical identifier for a function: its file, then its name
// case-folded. CFML is case-insensitive about function names, so folding here is
// what stops GetUser and getUser becoming two nodes that never join up.
func FuncID(relFile, name string) string {
	return relFile + "::" + strings.ToLower(name)
}

// FileID is the canonical identifier for a file: its path relative to the map root,
// slash-separated so an id means the same thing on every platform.
func FileID(relFile string) string {
	return relFile
}

// ExternalID identifies a call the build could not follow to a file. The "?" prefix
// cannot collide with a relative path, so these never merge with real nodes.
func ExternalID(label string) string {
	return "?" + strings.ToLower(label)
}

// PackageID identifies a directory. The map root itself collapses to ".".
func PackageID(dir string) string {
	if dir == "" || dir == "." {
		return "."
	}

	return dir
}

// NodeIndex is a by-id lookup over a map's nodes, for the views and renderers that
// need to go from an edge endpoint back to what it points at.
func (m *Map) NodeIndex() map[string]*Node {
	idx := make(map[string]*Node, len(m.Nodes))
	for i := range m.Nodes {
		idx[m.Nodes[i].ID] = &m.Nodes[i]
	}

	return idx
}

// dirOf returns the directory a node belongs to, for collapsing.
func dirOf(n *Node) string {
	if n.File == "" {
		return "."
	}

	return PackageID(path.Dir(n.File))
}
