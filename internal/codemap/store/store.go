//go:build !wasip1

// Package store persists a code map in SQLite so it can be queried without
// rebuilding, and caches per-file parse results so a rebuild only re-reads what
// changed.
//
// SQLite rather than a bespoke file because the useful questions about a code
// graph are relational and open-ended — "which remote methods reach this
// component", "what is in the largest detached island", "which functions named
// like this have no callers" — and a fixed set of Go accessors can only answer the
// ones someone thought of. A schema with indexes answers the rest, from this
// package, from the MCP server, or from a `sqlite3` prompt.
//
// The driver is modernc.org/sqlite, which is pure Go: the CGO build already has to
// work for tree-sitter, and adding a second CGO dependency would have put the
// wasip1 cross-build at risk for a feature that build does not need. This file is
// still tagged out of wasip1, where a database on local disk has no meaning.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite"
)

// Store is an open code-map database.
type Store struct {
	db *sql.DB
}

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS nodes (
  id         TEXT PRIMARY KEY,
  kind       TEXT NOT NULL,
  name       TEXT NOT NULL,
  name_lower TEXT NOT NULL,
  file       TEXT NOT NULL DEFAULT '',
  line       INTEGER NOT NULL DEFAULT 0,
  access     TEXT NOT NULL DEFAULT '',
  component  TEXT NOT NULL DEFAULT '',
  entry      INTEGER NOT NULL DEFAULT 0,
  island     INTEGER NOT NULL DEFAULT 0,
  reachable  INTEGER NOT NULL DEFAULT 0,
  root       INTEGER NOT NULL DEFAULT 0,
  utility    INTEGER NOT NULL DEFAULT 0,
  boundary   INTEGER NOT NULL DEFAULT 0,
  in_degree  INTEGER NOT NULL DEFAULT 0,
  out_degree INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS nodes_name   ON nodes(name_lower);
CREATE INDEX IF NOT EXISTS nodes_file   ON nodes(file);
CREATE INDEX IF NOT EXISTS nodes_island ON nodes(island);
CREATE INDEX IF NOT EXISTS nodes_kind   ON nodes(kind);
-- Answers "what is unreferenced" without a scan, which is the query most likely
-- to be run against the whole table.
CREATE INDEX IF NOT EXISTS nodes_dead   ON nodes(reachable, in_degree);
-- The question a utility marking is for: rank what everything depends on, with
-- the infrastructure separable.
CREATE INDEX IF NOT EXISTS nodes_util   ON nodes(utility, in_degree);

CREATE TABLE IF NOT EXISTS edges (
  from_id TEXT NOT NULL,
  to_id   TEXT NOT NULL,
  kind    TEXT NOT NULL,
  count   INTEGER NOT NULL DEFAULT 1,
  dynamic INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (from_id, to_id, kind)
) WITHOUT ROWID;

-- Both directions are indexed because callers and callees are asked for equally
-- often, and the primary key only serves one of them.
CREATE INDEX IF NOT EXISTS edges_to ON edges(to_id);

-- Trigram tokenizing, so a search for "perms" finds checkPermsList. A prefix-only
-- index would miss every function whose distinguishing word is not at the front,
-- which in CFML is most of them: the names start getX, setX, doX.
CREATE VIRTUAL TABLE IF NOT EXISTS symbols USING fts5(
  id UNINDEXED, name, file, component,
  tokenize = 'trigram'
);

CREATE TABLE IF NOT EXISTS file_cache (
  fingerprint TEXT NOT NULL,
  hash        TEXT NOT NULL,
  payload     BLOB NOT NULL,
  PRIMARY KEY (fingerprint, hash)
) WITHOUT ROWID;
`

// Open opens or creates a database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("creating schema in %s: %w", path, err)
	}

	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for callers that want SQL this package does not wrap.
// The schema is documented above and is the contract; the Go accessors are a
// convenience over it, not a fence around it.
func (s *Store) DB() *sql.DB { return s.db }

// Save replaces the stored graph with m.
//
// Replacement rather than merge: a map is a whole-workspace answer, and half of
// yesterday's graph joined to half of today's is a graph that never existed. The
// file cache is untouched, because that is keyed by content and survives.
func (s *Store) Save(m *codemap.Map) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning save: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	for _, stmt := range []string{`DELETE FROM nodes`, `DELETE FROM edges`, `DELETE FROM symbols`} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("clearing previous map: %w", err)
		}
	}

	in, out := degrees(m)

	nodeStmt, err := tx.Prepare(`INSERT INTO nodes
      (id, kind, name, name_lower, file, line, access, component, entry, island, reachable, root,
       utility, boundary, in_degree, out_degree)
      VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("preparing node insert: %w", err)
	}

	defer func() { _ = nodeStmt.Close() }()

	symStmt, err := tx.Prepare(`INSERT INTO symbols (id, name, file, component) VALUES (?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("preparing symbol insert: %w", err)
	}

	defer func() { _ = symStmt.Close() }()

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if _, err := nodeStmt.Exec(n.ID, string(n.Kind), n.Name, strings.ToLower(n.Name),
			n.File, n.Line, n.Access, n.Component,
			b2i(n.Entry), n.Island, b2i(n.Reachable), b2i(n.Root),
			b2i(n.Utility), b2i(n.Boundary),
			in[n.ID], out[n.ID]); err != nil {
			return fmt.Errorf("inserting node %s: %w", n.ID, err)
		}

		if _, err := symStmt.Exec(n.ID, n.Name, n.File, n.Component); err != nil {
			return fmt.Errorf("indexing node %s: %w", n.ID, err)
		}
	}

	edgeStmt, err := tx.Prepare(`INSERT OR REPLACE INTO edges (from_id, to_id, kind, count, dynamic) VALUES (?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("preparing edge insert: %w", err)
	}

	defer func() { _ = edgeStmt.Close() }()

	for i := range m.Edges {
		e := &m.Edges[i]
		if _, err := edgeStmt.Exec(e.From, e.To, string(e.Kind), e.Count, b2i(e.Dynamic)); err != nil {
			return fmt.Errorf("inserting edge %s->%s: %w", e.From, e.To, err)
		}
	}

	stats, err := json.Marshal(m.Stats)
	if err != nil {
		return fmt.Errorf("encoding stats: %w", err)
	}

	for k, v := range map[string]string{
		"root": m.Root, "level": string(m.Level), "stats": string(stats),
	} {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES (?,?)`, k, v); err != nil {
			return fmt.Errorf("writing meta %s: %w", k, err)
		}
	}

	return tx.Commit()
}

func degrees(m *codemap.Map) (in, out map[string]int) {
	in = make(map[string]int, len(m.Nodes))
	out = make(map[string]int, len(m.Nodes))

	for i := range m.Edges {
		e := &m.Edges[i]
		if e.Kind == codemap.EdgeContains {
			continue
		}

		in[e.To]++
		out[e.From]++
	}

	return in, out
}

func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}
