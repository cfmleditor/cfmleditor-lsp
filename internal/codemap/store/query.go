//go:build !wasip1

package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
)

const symbolCols = `id, kind, name, file, line, access, component, entry, reachable, island, in_degree, out_degree`

func scanSymbols(rows *sql.Rows) ([]Symbol, error) {
	defer func() { _ = rows.Close() }()

	var out []Symbol

	for rows.Next() {
		var (
			s                 Symbol
			entry, reach, ln  int
			access, component sql.NullString
		)

		if err := rows.Scan(&s.ID, &s.Kind, &s.Name, &s.File, &ln, &access, &component,
			&entry, &reach, &s.Island, &s.InDegree, &s.OutDegree); err != nil {
			return nil, fmt.Errorf("scanning symbol: %w", err)
		}

		s.Line = uint32(ln)
		s.Access = access.String
		s.Component = component.String
		s.Entry = entry != 0
		s.Reachable = reach != 0
		out = append(out, s)
	}

	return out, rows.Err()
}

// SearchSymbols finds nodes matching the options.
//
// A blank query lists by the other filters, which is what makes "every unreachable
// function under packages/tass" a single call rather than a fetch-everything-then-
// filter. The text match goes through the FTS5 trigram index when there is a
// query, and falls back to the plain table when there is not.
func (s *Store) SearchSymbols(opts SearchOptions) ([]Symbol, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var (
		where []string
		args  []any
	)

	from := "nodes n"

	if q := strings.TrimSpace(opts.Query); q != "" {
		// FTS5 treats a bare string as a query expression, where an unbalanced
		// quote or a bare AND is a syntax error rather than a search for that text.
		// Quoting makes the user's input a phrase, which is what they meant.
		from = "symbols s JOIN nodes n ON n.id = s.id"

		where = append(where, "symbols MATCH ?")
		args = append(args, `"`+strings.ReplaceAll(q, `"`, `""`)+`"`)
	}

	if opts.Kind != "" {
		where = append(where, "n.kind = ?")
		args = append(args, opts.Kind)
	}

	if opts.File != "" {
		where = append(where, "n.file LIKE ?")
		args = append(args, opts.File+"%")
	}

	if opts.Island != nil {
		where = append(where, "n.island = ?")
		args = append(args, *opts.Island)
	}

	if opts.OnlyDead {
		where = append(where, "n.reachable = 0")
	}

	order := "n.in_degree DESC, n.id"
	if !opts.SortByFan {
		order = "n.name_lower, n.id"
	}

	// Every fragment concatenated here is a package constant: the column list, one
	// of two literal FROM clauses, conditions chosen by a switch over fixed
	// strings, and one of two literal ORDER BYs. Not one byte comes from the
	// caller — every value the caller supplies, including the search text, goes in
	// as a bound parameter below.
	//nolint:gosec // G202: the concatenated parts are constants; all values are bound
	query := "SELECT " + prefixCols("n.", symbolCols) + " FROM " + from
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	query += " ORDER BY " + order + " LIMIT ?"

	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("searching symbols: %w", err)
	}

	return scanSymbols(rows)
}

func prefixCols(prefix, cols string) string {
	parts := strings.Split(cols, ", ")
	for i, p := range parts {
		parts[i] = prefix + p
	}

	return strings.Join(parts, ", ")
}

// Node returns one symbol by id.
func (s *Store) Node(id string) (Symbol, error) {
	rows, err := s.db.Query("SELECT "+symbolCols+" FROM nodes WHERE id = ?", id)
	if err != nil {
		return Symbol{}, fmt.Errorf("reading node %s: %w", id, err)
	}

	found, err := scanSymbols(rows)
	if err != nil {
		return Symbol{}, err
	}

	if len(found) == 0 {
		return Symbol{}, fmt.Errorf("%w: node %q", ErrNotFound, id)
	}

	return found[0], nil
}

// Callers returns what depends on id; Callees what id depends on.
func (s *Store) Callers(id string, limit int) ([]Neighbour, error) {
	return s.neighbours(id, "e.to_id = ?", "e.from_id", limit)
}

// Callees returns what id depends on.
func (s *Store) Callees(id string, limit int) ([]Neighbour, error) {
	return s.neighbours(id, "e.from_id = ?", "e.to_id", limit)
}

func (s *Store) neighbours(id, match, join string, limit int) ([]Neighbour, error) {
	if limit <= 0 {
		limit = 200
	}

	// match and join are literals chosen by Callers/Callees, never caller input.
	//nolint:gosec // G202: the concatenated parts are constants; id is bound below
	query := `SELECT ` + prefixCols("n.", symbolCols) + `, e.kind, e.count, e.dynamic
	          FROM edges e JOIN nodes n ON n.id = ` + join + `
	          WHERE ` + match + ` AND e.kind != 'contains'
	          ORDER BY e.count DESC, n.id LIMIT ?`

	rows, err := s.db.Query(query, id, limit)
	if err != nil {
		return nil, fmt.Errorf("reading neighbours of %s: %w", id, err)
	}

	defer func() { _ = rows.Close() }()

	var out []Neighbour

	for rows.Next() {
		var (
			n                 Neighbour
			entry, reach, dyn int
			ln                int
			access, component sql.NullString
		)

		if err := rows.Scan(&n.ID, &n.Kind, &n.Name, &n.File, &ln, &access, &component,
			&entry, &reach, &n.Island, &n.InDegree, &n.OutDegree,
			&n.EdgeKind, &n.Count, &dyn); err != nil {
			return nil, fmt.Errorf("scanning neighbour: %w", err)
		}

		n.Line = uint32(ln)
		n.Access = access.String
		n.Component = component.String
		n.Entry = entry != 0
		n.Reachable = reach != 0
		n.Dynamic = dyn != 0
		out = append(out, n)
	}

	return out, rows.Err()
}

// Path returns the shortest dependency path from one node to another, or nil when
// there is none within maxDepth.
//
// Breadth-first in Go, one query per frontier level, rather than the recursive CTE
// this obviously wants to be. A CTE can express the walk but not a visited set
// shared across branches: SQLite can only stop a trail revisiting its own nodes,
// so every path to a node is explored separately and a search from a high-fanout
// node with no answer to find is exponential in the depth limit. A visited map
// makes the whole search O(V+E) no matter what it is asked, which matters because
// "is there any path" is asked most often about pairs that have none.
//
// Each level is one query over the frontier rather than one per node, so the
// number of round trips is the depth, not the number of nodes visited.
func (s *Store) Path(from, to string, maxDepth int) ([]Symbol, error) {
	if maxDepth <= 0 {
		maxDepth = 12
	}

	if from == to {
		sym, err := s.Node(from)
		if err != nil {
			return nil, err
		}

		return []Symbol{sym}, nil
	}

	came := map[string]string{from: ""}
	frontier := []string{from}

	for depth := 0; depth < maxDepth && len(frontier) > 0; depth++ {
		next, found, err := s.expand(frontier, to, came)
		if err != nil {
			return nil, err
		}

		if found {
			return s.rebuild(came, from, to)
		}

		frontier = next
	}

	return nil, nil
}

// sqlBatch bounds how many ids go into one IN clause. SQLite's default host
// parameter limit is well above this; the smaller number keeps the statement cache
// useful and the query planner honest on a frontier of tens of thousands.
const sqlBatch = 400

// expand reads every edge out of the frontier in batched queries, recording how
// each newly-seen node was reached. It stops as soon as the target appears.
func (s *Store) expand(frontier []string, target string, came map[string]string) ([]string, bool, error) {
	var next []string

	for start := 0; start < len(frontier); start += sqlBatch {
		end := min(start+sqlBatch, len(frontier))
		batch := frontier[start:end]

		args := make([]any, 0, len(batch))
		for _, id := range batch {
			args = append(args, id)
		}

		// The only variable part is the run of "?" placeholders, whose length is
		// the batch size. The ids themselves are bound.
		//nolint:gosec // G202: placeholders only; every id is a bound parameter
		query := `SELECT from_id, to_id FROM edges
		          WHERE kind != 'contains' AND from_id IN (` +
			strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",") + `)`

		rows, err := s.db.Query(query, args...)
		if err != nil {
			return nil, false, fmt.Errorf("expanding path frontier: %w", err)
		}

		found, err := collectFrontier(rows, target, came, &next)
		if err != nil {
			return nil, false, err
		}

		if found {
			return nil, true, nil
		}
	}

	return next, false, nil
}

func collectFrontier(rows *sql.Rows, target string, came map[string]string, next *[]string) (bool, error) {
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var fromID, toID string
		if err := rows.Scan(&fromID, &toID); err != nil {
			return false, fmt.Errorf("scanning edge: %w", err)
		}

		if _, seen := came[toID]; seen {
			continue
		}

		came[toID] = fromID

		if toID == target {
			return true, rows.Err()
		}

		*next = append(*next, toID)
	}

	return false, rows.Err()
}

// rebuild walks the predecessor map backwards from the target and loads each hop.
func (s *Store) rebuild(came map[string]string, from, to string) ([]Symbol, error) {
	var ids []string

	for id := to; id != ""; id = came[id] {
		ids = append(ids, id)

		if id == from {
			break
		}
	}

	slices.Reverse(ids)

	out := make([]Symbol, 0, len(ids))

	for _, id := range ids {
		sym, err := s.Node(id)
		if err != nil {
			return nil, err
		}

		out = append(out, sym)
	}

	return out, nil
}

// Islands lists the connected pieces, largest first. With detachedOnly it lists
// only those no entry point reaches — the detached trees, which is the list worth
// reading first on a codebase nobody has mapped before.
func (s *Store) Islands(detachedOnly bool, limit int) ([]IslandSummary, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT island, COUNT(*),
	                 SUM(kind = 'function'), SUM(kind = 'file'), MAX(reachable)
	          FROM nodes GROUP BY island`
	if detachedOnly {
		query += ` HAVING MAX(reachable) = 0`
	}

	query += ` ORDER BY COUNT(*) DESC LIMIT ?`

	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("listing islands: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var out []IslandSummary

	for rows.Next() {
		var (
			isl   IslandSummary
			reach int
		)

		if err := rows.Scan(&isl.ID, &isl.Nodes, &isl.Functions, &isl.Files, &reach); err != nil {
			return nil, fmt.Errorf("scanning island: %w", err)
		}

		isl.Reachable = reach != 0
		out = append(out, isl)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range out {
		roots, err := s.islandRoots(out[i].ID)
		if err != nil {
			return nil, err
		}

		out[i].Roots = roots
	}

	return out, nil
}

func (s *Store) islandRoots(island int) ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM nodes WHERE island = ? AND root = 1 ORDER BY id LIMIT 5`, island)
	if err != nil {
		return nil, fmt.Errorf("reading island %d roots: %w", island, err)
	}

	defer func() { _ = rows.Close() }()

	var out []string

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning root: %w", err)
		}

		out = append(out, id)
	}

	return out, rows.Err()
}

// Orphans lists nodes nothing depends on and no entry point reaches. These are
// deletion candidates, not dead code proven: an unresolved call is an edge the map
// does not have. Read Stats().Unresolved alongside.
func (s *Store) Orphans(kind string, limit int) ([]Symbol, error) {
	return s.SearchSymbols(SearchOptions{Kind: kind, OnlyDead: true, Limit: limit})
}

// Stats returns the build statistics recorded with the map.
func (s *Store) Stats() (codemap.Stats, error) {
	var raw string

	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'stats'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return codemap.Stats{}, fmt.Errorf("%w: no map has been saved", ErrNotFound)
	}

	if err != nil {
		return codemap.Stats{}, fmt.Errorf("reading stats: %w", err)
	}

	var stats codemap.Stats
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		return codemap.Stats{}, fmt.Errorf("decoding stats: %w", err)
	}

	return stats, nil
}

// Meta reads one stored metadata value: "root", "level" or "stats".
func (s *Store) Meta(key string) (string, error) {
	var v string

	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: meta %q", ErrNotFound, key)
	}

	if err != nil {
		return "", fmt.Errorf("reading meta %q: %w", key, err)
	}

	return v, nil
}
