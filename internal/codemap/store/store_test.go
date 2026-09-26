//go:build !wasip1

package store_test

import (
	"path/filepath"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap/store"
)

func sample() *codemap.Map {
	m := &codemap.Map{
		Root: "/w", Level: codemap.LevelFunction, Fingerprint: "fp1",
		Nodes: []codemap.Node{
			{ID: "page.cfm", Kind: codemap.KindFile, Name: "page.cfm", File: "page.cfm", Entry: true},
			{ID: "svc/user.cfc", Kind: codemap.KindFile, Name: "user.cfc", File: "svc/user.cfc"},
			{ID: "svc/user.cfc::getuser", Kind: codemap.KindFunction, Name: "getUser", File: "svc/user.cfc", Line: 9, Access: "public"},
			{ID: "svc/user.cfc::checkpermslist", Kind: codemap.KindFunction, Name: "checkPermsList", File: "svc/user.cfc", Line: 40, Access: "public"},
			{ID: "svc/dead.cfc::nevercalled", Kind: codemap.KindFunction, Name: "neverCalled", File: "svc/dead.cfc", Line: 3, Access: "private"},
		},
		Edges: []codemap.Edge{
			{From: "page.cfm", To: "svc/user.cfc::getuser", Kind: codemap.EdgeCalls, Count: 4},
			{From: "svc/user.cfc::getuser", To: "svc/user.cfc::checkpermslist", Kind: codemap.EdgeCalls, Count: 1},
			{From: "svc/user.cfc", To: "svc/user.cfc::getuser", Kind: codemap.EdgeContains, Count: 1},
		},
		Stats: codemap.Stats{Files: 3, Functions: 3, CallSites: 5, Resolved: 2},
	}
	m.Annotate()

	return m
}

func open(t *testing.T) *store.Store {
	t.Helper()

	db, err := store.Open(filepath.Join(t.TempDir(), "map.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if err := db.Save(sample()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	return db
}

// TestSearchFindsAMidWordMatch is why the FTS index is trigram-tokenised. CFML
// function names start getX, setX, doX, so a prefix-only index would miss the
// distinguishing word in most of them — searching "perms" has to find
// checkPermsList.
func TestSearchFindsAMidWordMatch(t *testing.T) {
	db := open(t)

	found, err := db.SearchSymbols(&store.SearchOptions{Query: "perms"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(found) != 1 || found[0].Name != "checkPermsList" {
		t.Fatalf("searching \"perms\" gave %+v, want just checkPermsList", found)
	}
}

// TestSearchQueryIsAPhraseNotAnExpression: FTS5 reads a bare string as a query
// expression, where a stray quote or a bare AND is a syntax error rather than a
// search for that text. A user typing a component path with quotes in it should
// get results or nothing, never an error.
func TestSearchQueryIsAPhraseNotAnExpression(t *testing.T) {
	s := open(t)

	for _, q := range []string{`user" OR `, "AND", "a AND b", `"`, "*", "NEAR(a b)"} {
		if _, err := s.SearchSymbols(&store.SearchOptions{Query: q}); err != nil {
			t.Errorf("searching %q returned an error rather than no results: %v", q, err)
		}
	}
}

// TestCallersIgnoresContains checks that the file→function hierarchy does not show
// up as a caller. It is not a dependency, and counting it would make every
// function look called by its own file.
func TestCallersIgnoresContains(t *testing.T) {
	s := open(t)

	callers, err := s.Callers("svc/user.cfc::getuser", 0)
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}

	for _, c := range callers {
		if c.EdgeKind == string(codemap.EdgeContains) {
			t.Fatalf("Callers returned a contains edge from %s", c.ID)
		}
	}

	if len(callers) != 1 || callers[0].ID != "page.cfm" || callers[0].Count != 4 {
		t.Fatalf("callers = %+v, want just page.cfm with count 4", callers)
	}
}

// TestPathIsShortestAndBounded covers both halves of the BFS: it finds the answer,
// and a search with no answer terminates rather than exploring every trail.
func TestPathIsShortestAndBounded(t *testing.T) {
	s := open(t)

	path, err := s.Path("page.cfm", "svc/user.cfc::checkpermslist", 5)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}

	want := []string{"page.cfm", "svc/user.cfc::getuser", "svc/user.cfc::checkpermslist"}
	if len(path) != len(want) {
		t.Fatalf("path has %d hops, want %d: %+v", len(path), len(want), path)
	}

	for i := range want {
		if path[i].ID != want[i] {
			t.Errorf("hop %d = %q, want %q", i, path[i].ID, want[i])
		}
	}

	missing, err := s.Path("page.cfm", "svc/dead.cfc::nevercalled", 10)
	if err != nil {
		t.Fatalf("Path to an unreachable node: %v", err)
	}

	if missing != nil {
		t.Errorf("found a path to an unreachable node: %+v", missing)
	}

	// A depth limit shorter than the answer must return nothing, not the answer.
	short, err := s.Path("page.cfm", "svc/user.cfc::checkpermslist", 1)
	if err != nil {
		t.Fatalf("Path with a short limit: %v", err)
	}

	if short != nil {
		t.Errorf("max_depth 1 returned a 2-hop path: %+v", short)
	}
}

// TestOrphansFindsTheUncalledFunction is the store's side of the guarantee that
// unreachable code stays in the map: it has to be stored to be listed.
func TestOrphansFindsTheUncalledFunction(t *testing.T) {
	s := open(t)

	orphans, err := s.Orphans("function", 0)
	if err != nil {
		t.Fatalf("Orphans: %v", err)
	}

	var found bool

	for _, o := range orphans {
		if o.ID == "svc/dead.cfc::nevercalled" {
			found = true
		}
	}

	if !found {
		t.Fatalf("the uncalled function is not in the orphan list: %+v", orphans)
	}
}

// TestSaveReplacesRatherThanMerges: half of yesterday's graph joined to half of
// today's is a graph that never existed.
func TestSaveReplacesRatherThanMerges(t *testing.T) {
	s := open(t)

	second := sample()
	second.Nodes = second.Nodes[:1]
	second.Edges = nil
	second.Annotate()

	if err := s.Save(second); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	all, err := s.SearchSymbols(&store.SearchOptions{Limit: 100})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(all) != 1 {
		t.Fatalf("after replacing the map the store holds %d nodes, want 1: %+v", len(all), all)
	}
}

// TestCacheRoundTrip covers the two-part key. Same content under a new workspace
// fingerprint must miss, because the edges it holds were resolved against an index
// that no longer describes the workspace.
func TestCacheRoundTrip(t *testing.T) {
	s := open(t)
	c := s.NewCache()

	g := &codemap.FileGraph{Nodes: []codemap.Node{{ID: "a", Kind: codemap.KindFunction, Name: "a"}}}
	if err := c.Save("fp1", "hash1", g); err != nil {
		t.Fatalf("cache Save: %v", err)
	}

	if err := c.Flush(); err != nil {
		t.Fatalf("cache Flush: %v", err)
	}

	got, ok := c.Load("fp1", "hash1")
	if !ok {
		t.Fatal("a just-saved entry did not come back")
	}

	if len(got.Nodes) != 1 || got.Nodes[0].ID != "a" {
		t.Errorf("cached graph came back as %+v", got.Nodes)
	}

	if _, ok := c.Load("fp2", "hash1"); ok {
		t.Error("the same file content hit under a different workspace fingerprint; " +
			"a cached edge set would outlive the index it was resolved against")
	}

	if _, ok := c.Load("fp1", "other"); ok {
		t.Error("a different content hash hit")
	}
}

// TestPruneCacheKeepsOnlyTheCurrentGeneration. Without it the table grows by a
// full workspace on every .cfc edit, and none of the old rows can ever be hit.
func TestPruneCacheKeepsOnlyTheCurrentGeneration(t *testing.T) {
	s := open(t)
	c := s.NewCache()

	g := &codemap.FileGraph{}
	for _, fp := range []string{"old1", "old2", "current"} {
		if err := c.Save(fp, "h", g); err != nil {
			t.Fatalf("cache Save: %v", err)
		}
	}

	if err := c.Flush(); err != nil {
		t.Fatalf("cache Flush: %v", err)
	}

	if err := s.PruneCache("current"); err != nil {
		t.Fatalf("PruneCache: %v", err)
	}

	if _, ok := c.Load("current", "h"); !ok {
		t.Error("prune removed the generation it was told to keep")
	}

	for _, fp := range []string{"old1", "old2"} {
		if _, ok := c.Load(fp, "h"); ok {
			t.Errorf("generation %q survived the prune", fp)
		}
	}
}

// TestStatsSurviveTheRoundTrip: the resolved/unresolved ratio is how a reader
// knows how much to trust an empty caller list, so it has to reach the store.
func TestStatsSurviveTheRoundTrip(t *testing.T) {
	stats, err := open(t).Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	if stats.CallSites != 5 || stats.Resolved != 2 {
		t.Errorf("stats came back as %+v", stats)
	}
}

// TestUtilityReachesTheStore. The store is the queryable artifact, and the
// question a utility marking exists for — rank what everything depends on, with
// the infrastructure separable — is a SQL query. A flag the map carries and the
// store drops makes that query impossible to write and says nothing about why.
func TestUtilityReachesTheStore(t *testing.T) {
	s := open(t)

	m := sample()
	for i := range m.Nodes {
		if m.Nodes[i].ID == "svc/user.cfc::getuser" {
			m.Nodes[i].Utility = true
		}
	}

	if err := s.Save(m); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := s.SearchSymbols(&store.SearchOptions{Query: "getUser"})
	if err != nil {
		t.Fatalf("SearchSymbols: %v", err)
	}

	if len(found) != 1 || !found[0].Utility {
		t.Fatalf("the utility flag did not survive the round trip: %+v", found)
	}

	// And it has to be queryable, not merely stored.
	var n int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM nodes WHERE utility = 1`).Scan(&n); err != nil {
		t.Fatalf("querying the utility column: %v", err)
	}

	if n != 1 {
		t.Errorf("SELECT ... WHERE utility = 1 found %d rows, want 1", n)
	}
}
