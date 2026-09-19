//go:build wasip1

package store

import (
	"fmt"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
)

// The wasip1 build has no local database. Rather than removing the graph
// subcommand from that build — which would make the CLI's behaviour differ by
// platform in a way nothing announces — the store is present and every entry
// point declines with the same message. The map itself still builds and renders;
// only persistence and the MCP server are unavailable.

var errUnsupported = fmt.Errorf("the code-map database is not available in the wasm build")

// Store is the unavailable database.
type Store struct{}

// Cache is the unavailable file cache.
type Cache struct{}

// Open always fails on wasip1.
func Open(string) (*Store, error) { return nil, errUnsupported }

// Close is a no-op.
func (*Store) Close() error { return nil }

// Save always fails.
func (*Store) Save(*codemap.Map) error { return errUnsupported }

// NewCache returns a cache that never hits.
func (*Store) NewCache() *Cache { return &Cache{} }

// PruneCache always fails.
func (*Store) PruneCache(string) error { return errUnsupported }

// SearchSymbols always fails.
func (*Store) SearchSymbols(SearchOptions) ([]Symbol, error) { return nil, errUnsupported }

// Node always fails.
func (*Store) Node(string) (Symbol, error) { return Symbol{}, errUnsupported }

// Callers always fails.
func (*Store) Callers(string, int) ([]Neighbour, error) { return nil, errUnsupported }

// Callees always fails.
func (*Store) Callees(string, int) ([]Neighbour, error) { return nil, errUnsupported }

// Path always fails.
func (*Store) Path(string, string, int) ([]Symbol, error) { return nil, errUnsupported }

// Islands always fails.
func (*Store) Islands(bool, int) ([]IslandSummary, error) { return nil, errUnsupported }

// Orphans always fails.
func (*Store) Orphans(string, int) ([]Symbol, error) { return nil, errUnsupported }

// Stats always fails.
func (*Store) Stats() (codemap.Stats, error) { return codemap.Stats{}, errUnsupported }

// Meta always fails.
func (*Store) Meta(string) (string, error) { return "", errUnsupported }

// Load never hits.
func (*Cache) Load(string, string) (*codemap.FileGraph, bool) { return nil, false }

// Save is a no-op.
func (*Cache) Save(string, string, *codemap.FileGraph) error { return nil }

// Flush is a no-op.
func (*Cache) Flush() error { return nil }

// Hits reports nothing.
func (*Cache) Hits() (int, int) { return 0, 0 }

var _ codemap.Cache = (*Cache)(nil)
