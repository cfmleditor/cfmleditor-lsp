//go:build !wasip1

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
)

// Cache is the [codemap.Cache] backed by this store's file_cache table.
//
// Writes are batched rather than committed one per file. A scan of 11,700 files
// makes 11,700 cache writes, and SQLite's per-transaction fsync turns that into
// minutes of disk sync on a build that otherwise takes ten seconds — the cache
// would cost more than the parsing it saves. Buffering and flushing in blocks
// makes the write cost proportional to the work, and a crash mid-scan loses cache
// rows, which is a slower next build and nothing else.
type Cache struct {
	store *Store

	mu      sync.Mutex
	pending []cacheRow
	hits    int
	misses  int
}

type cacheRow struct {
	fingerprint string
	hash        string
	payload     []byte
}

// cacheFlushAt is how many rows accumulate before a commit. Large enough that the
// fsync cost disappears into the scan, small enough that a huge workspace does not
// hold every file's edges in memory at once.
const cacheFlushAt = 512

// NewCache returns a cache over this store. Call Flush when the build finishes.
func (s *Store) NewCache() *Cache {
	return &Cache{store: s}
}

// Load returns a cached file graph, if one was stored for this content under this
// workspace fingerprint.
func (c *Cache) Load(fingerprint, hash string) (*codemap.FileGraph, bool) {
	var payload []byte

	err := c.store.db.QueryRow(
		`SELECT payload FROM file_cache WHERE fingerprint = ? AND hash = ?`,
		fingerprint, hash).Scan(&payload)
	if err != nil {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()

		return nil, false
	}

	var g codemap.FileGraph
	if json.Unmarshal(payload, &g) != nil {
		return nil, false
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	return &g, true
}

// Save buffers a file graph, flushing to disk in blocks.
func (c *Cache) Save(fingerprint, hash string, g *codemap.FileGraph) error {
	payload, err := json.Marshal(g)
	if err != nil {
		return fmt.Errorf("encoding cache entry: %w", err)
	}

	c.mu.Lock()
	c.pending = append(c.pending, cacheRow{fingerprint, hash, payload})

	if len(c.pending) < cacheFlushAt {
		c.mu.Unlock()

		return nil
	}

	batch := c.pending
	c.pending = nil
	c.mu.Unlock()

	return c.write(batch)
}

// Flush writes everything still buffered.
func (c *Cache) Flush() error {
	c.mu.Lock()
	batch := c.pending
	c.pending = nil
	c.mu.Unlock()

	return c.write(batch)
}

// Hits reports how many files were served from the cache and how many were not.
func (c *Cache) Hits() (hits, misses int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.hits, c.misses
}

func (c *Cache) write(batch []cacheRow) error {
	if len(batch) == 0 {
		return nil
	}

	tx, err := c.store.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning cache write: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO file_cache (fingerprint, hash, payload) VALUES (?,?,?)`)
	if err != nil {
		return fmt.Errorf("preparing cache write: %w", err)
	}

	defer func() { _ = stmt.Close() }()

	for _, row := range batch {
		if _, err := stmt.Exec(row.fingerprint, row.hash, row.payload); err != nil {
			return fmt.Errorf("writing cache entry: %w", err)
		}
	}

	return tx.Commit()
}

// PruneCache drops every cache row not belonging to the given fingerprint.
//
// Without this the table grows by a full workspace every time any .cfc changes,
// since each edit produces a new fingerprint and none of the old rows will ever
// match again. Keeping one generation is the right amount: the previous one cannot
// be hit, because the fingerprint that would select it no longer describes the
// workspace.
func (s *Store) PruneCache(keep string) error {
	res, err := s.db.Exec(`DELETE FROM file_cache WHERE fingerprint != ?`, keep)
	if err != nil {
		return fmt.Errorf("pruning cache: %w", err)
	}

	if n, err := res.RowsAffected(); err == nil && n > 0 {
		// VACUUM cannot run inside a transaction, and there is none here. Freeing
		// the pages matters: a cached workspace is tens of megabytes per generation.
		if _, err := s.db.Exec(`VACUUM`); err != nil && !errors.Is(err, sql.ErrTxDone) {
			return fmt.Errorf("compacting after prune: %w", err)
		}
	}

	return nil
}

var _ codemap.Cache = (*Cache)(nil)
