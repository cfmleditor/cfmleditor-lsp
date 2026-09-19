package store

import "errors"

// This file has no build tag: the shapes a query returns are part of the package's
// API on every platform, and only the SQLite implementation is excluded from
// wasip1. Without the split, a caller could not so much as name a Symbol in code
// that also builds for wasm.

// Symbol is a node as the query layer returns it: the map's own fields plus the
// degrees, which is what a caller almost always wants next and what would
// otherwise be a second query per row.
type Symbol struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	File      string `json:"file,omitempty"`
	Line      uint32 `json:"line,omitempty"`
	Access    string `json:"access,omitempty"`
	Component string `json:"component,omitempty"`
	Entry     bool   `json:"entry,omitempty"`
	Reachable bool   `json:"reachable"`
	Utility   bool   `json:"utility,omitempty"`
	Island    int    `json:"island"`
	InDegree  int    `json:"inDegree"`
	OutDegree int    `json:"outDegree"`
}

// SearchOptions narrows a symbol search.
type SearchOptions struct {
	Query     string // matched against name, file and component
	Kind      string // "function", "file", "package", "external"; empty for any
	File      string // path prefix
	Island    *int
	OnlyDead  bool // no entry point reaches it
	Limit     int
	SortByFan bool // most depended on first, rather than by name
}

// Neighbour is a symbol reached across one edge, with that edge's own detail.
type Neighbour struct {
	Symbol
	EdgeKind string `json:"edgeKind"`
	Count    int    `json:"count"`
	Dynamic  bool   `json:"dynamic,omitempty"`
}

// IslandSummary describes one connected piece of the graph.
type IslandSummary struct {
	ID        int      `json:"id"`
	Nodes     int      `json:"nodes"`
	Functions int      `json:"functions"`
	Files     int      `json:"files"`
	Reachable bool     `json:"reachable"`
	Roots     []string `json:"roots"`
}

// ErrNotFound is returned when a query names a node that is not in the store.
var ErrNotFound = errors.New("not found")
