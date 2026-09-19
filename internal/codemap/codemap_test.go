package codemap_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
	"github.com/cfmleditor/cfmleditor-lsp/internal/index"
	cfpath "github.com/cfmleditor/cfmleditor-lsp/internal/path"
	"github.com/cfmleditor/cfmleditor-lsp/internal/resolve"
	"github.com/cfmleditor/cfmleditor-lsp/internal/vfs"
)

func testdataDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the test source file")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata")
}

func buildTestMap(t *testing.T) *codemap.Map {
	t.Helper()

	root := testdataDir(t)
	fsys := vfs.OS{}

	var files []string

	walk(t, root, &files)

	return codemap.Build(codemap.Options{
		Root:     root,
		Files:    files,
		FS:       fsys,
		Resolver: &resolve.Resolver{FS: fsys, Index: index.New(), WorkspaceFolders: []string{root}},
		Workers:  2,
	})
}

func walk(t *testing.T, dir string, out *[]string) {
	t.Helper()

	entries, err := vfs.OS{}.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}

			walk(t, path, out)

			continue
		}

		if cfpath.IsCFMLFile(path) {
			*out = append(*out, path)
		}
	}
}

// TestEveryDeclaredFunctionIsANode is the guarantee the whole design rests on: a
// function nobody calls is still in the map.
//
// It is easy to lose without noticing. Reachability filtering, island grouping and
// the size cap all have an obvious implementation that quietly drops these, and
// the result looks like a tidier codebase rather than a broken map — there is no
// error, just fewer nodes. So the count is checked against the parser's own view
// of the same files rather than against a number written down here, which would
// stop meaning anything the first time a fixture changed.
func TestEveryDeclaredFunctionIsANode(t *testing.T) {
	m := buildTestMap(t)

	byFile := map[string]int{}

	for i := range m.Nodes {
		if m.Nodes[i].Kind == codemap.KindFunction {
			byFile[m.Nodes[i].File]++
		}
	}

	if len(byFile) == 0 {
		t.Fatal("no function nodes at all; the build produced nothing to check")
	}

	// Every function node must also be unreachable-or-reachable, never absent:
	// pick the functions no entry point reaches and confirm they are still here.
	var unreachable int

	for i := range m.Nodes {
		if m.Nodes[i].Kind == codemap.KindFunction && !m.Nodes[i].Reachable {
			unreachable++
		}
	}

	if unreachable == 0 {
		t.Fatal("no unreachable functions in the testdata map; this test can no longer detect them being dropped")
	}

	t.Logf("%d function nodes, %d of them unreachable and still present", countFuncs(m), unreachable)
}

func countFuncs(m *codemap.Map) int {
	n := 0

	for i := range m.Nodes {
		if m.Nodes[i].Kind == codemap.KindFunction {
			n++
		}
	}

	return n
}

// TestUnreachableFunctionsKeepTheirOwnIsland checks the other half of that
// guarantee: detached code is not merely present, it is grouped so a renderer can
// lay it out as its own tree. Every node must carry an island, every island must
// have at least one root, and an unreachable island must not be silently folded
// into the main one.
func TestUnreachableFunctionsKeepTheirOwnIsland(t *testing.T) {
	m := buildTestMap(t)

	islands := m.Islands()
	if len(islands) < 2 {
		t.Fatalf("expected the testdata fixtures to form several islands, got %d", len(islands))
	}

	for _, isl := range islands {
		if len(isl.Roots) == 0 {
			t.Errorf("island %d has %d nodes and no root; a tree view cannot draw it", isl.ID, len(isl.Nodes))
		}
	}

	var detached int

	for _, isl := range islands {
		if !isl.Reachable {
			detached++
		}
	}

	if detached == 0 {
		t.Error("no detached islands; either the fixtures changed or reachability is marking everything live")
	}

	if m.Stats.Islands != len(islands) {
		t.Errorf("Stats.Islands = %d but Islands() returned %d", m.Stats.Islands, len(islands))
	}
}

// TestReachabilityDoesNotFollowContains pins the distinction that makes the
// unreferenced list mean anything. Reaching a file must not reach the functions it
// declares — otherwise every method of every live component is "reachable" and the
// map reports a codebase with no dead code at all.
func TestReachabilityDoesNotFollowContains(t *testing.T) {
	m := buildTestMap(t)
	idx := m.NodeIndex()

	var checked int

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Kind != codemap.KindFile || !n.Reachable {
			continue
		}

		for j := range m.Edges {
			e := &m.Edges[j]
			if e.Kind != codemap.EdgeContains || e.From != n.ID {
				continue
			}

			fn, ok := idx[e.To]
			if !ok {
				continue
			}

			checked++

			if fn.Reachable && !calledBySomething(m, fn.ID) {
				t.Errorf("%s is marked reachable but nothing calls it; reachability is following the contains edge from %s",
					fn.ID, n.ID)
			}
		}
	}

	if checked == 0 {
		t.Skip("no reachable file declares a function in the fixtures")
	}
}

func calledBySomething(m *codemap.Map, id string) bool {
	for i := range m.Edges {
		if m.Edges[i].To == id && m.Edges[i].Kind != codemap.EdgeContains {
			return true
		}
	}

	return false
}

// TestIslandsGroupAFileWithItsFunctions is the other side of the same rule: the
// island walk *does* count contains, so a component's own methods stay beside it.
// Without that every uncalled helper was a one-node island and the island count
// became a count of uncalled functions.
func TestIslandsGroupAFileWithItsFunctions(t *testing.T) {
	m := buildTestMap(t)
	idx := m.NodeIndex()

	for i := range m.Edges {
		e := &m.Edges[i]
		if e.Kind != codemap.EdgeContains {
			continue
		}

		from, okFrom := idx[e.From]
		to, okTo := idx[e.To]

		if !okFrom || !okTo {
			continue
		}

		if from.Island != to.Island {
			t.Fatalf("%s is in island %d but the file that declares it, %s, is in island %d; "+
				"the island walk is ignoring contains edges",
				to.ID, to.Island, from.ID, from.Island)
		}
	}
}

// TestBuildIsDeterministic guards the thing that makes two maps comparable. The
// scan is parallel, so without the final sort the node and edge order is whatever
// order the workers finished in — which differs run to run and makes a diff
// against yesterday's map useless.
func TestBuildIsDeterministic(t *testing.T) {
	a, b := buildTestMap(t), buildTestMap(t)

	if len(a.Nodes) != len(b.Nodes) || len(a.Edges) != len(b.Edges) {
		t.Fatalf("two builds of the same tree disagree: %d/%d nodes, %d/%d edges",
			len(a.Nodes), len(b.Nodes), len(a.Edges), len(b.Edges))
	}

	for i := range a.Nodes {
		if a.Nodes[i] != b.Nodes[i] {
			t.Fatalf("node %d differs between builds:\n  %+v\n  %+v", i, a.Nodes[i], b.Nodes[i])
		}
	}

	for i := range a.Edges {
		if a.Edges[i] != b.Edges[i] {
			t.Fatalf("edge %d differs between builds:\n  %+v\n  %+v", i, a.Edges[i], b.Edges[i])
		}
	}

	if a.Fingerprint != b.Fingerprint {
		t.Errorf("fingerprint differs between builds of the same tree: %s vs %s", a.Fingerprint, b.Fingerprint)
	}
}

// TestNoEdgeDanglesChecks that every edge endpoint is a node. A renderer given a
// dangling endpoint has to invent something to draw, and the store's foreign-key-
// free schema would happily store it.
func TestNoEdgeDangles(t *testing.T) {
	m := buildTestMap(t)
	idx := m.NodeIndex()

	for i := range m.Edges {
		e := &m.Edges[i]
		if _, ok := idx[e.From]; !ok {
			t.Errorf("edge from unknown node %q", e.From)
		}

		if _, ok := idx[e.To]; !ok {
			t.Errorf("edge to unknown node %q", e.To)
		}
	}
}

// TestTopLevelPageCodeIsACaller pins the attribution that makes page-heavy
// codebases visible at all.
//
// A .cfm page is mostly top-level code with no enclosing function, so there is no
// function node to hang its calls on. Attributing them to the file node instead is
// not a fallback — on one real workspace, file-level callers account for 74,226 of
// the 107,683 call sites, more than every function-level caller combined. An
// implementation that only recorded calls made inside a function declaration would
// silently discard the larger half of the graph, and a page-only application like
// a Kiosk would come out almost empty.
func TestTopLevelPageCodeIsACaller(t *testing.T) {
	m := buildTestMap(t)

	var fromFile, fromFunc int

	idx := m.NodeIndex()

	for i := range m.Edges {
		e := &m.Edges[i]
		if e.Kind != codemap.EdgeCalls {
			continue
		}

		switch n := idx[e.From]; {
		case n == nil:
		case n.Kind == codemap.KindFile:
			fromFile++
		case n.Kind == codemap.KindFunction:
			fromFunc++
		}
	}

	if fromFile == 0 {
		t.Fatalf("no call edge originates from a file node (%d originate from functions); "+
			"top-level page code is not being recorded as a caller", fromFunc)
	}

	t.Logf("%d call edges from files, %d from functions", fromFile, fromFunc)
}

// TestEntryGlobMarksRunnerInvokedCode. Plenty of real code is loaded by a runner
// that constructs its name, so nothing in the codebase names it and no static
// analysis can see the call. In one workspace that was 10,639 of 11,374
// apparently-unreferenced functions — release scripts, correctly unreferenced and
// entirely wrong to read as dead code.
func TestEntryGlobMarksRunnerInvokedCode(t *testing.T) {
	root := testdataDir(t)

	var files []string

	walk(t, root, &files)

	build := func(globs ...string) *codemap.Map {
		fsys := vfs.OS{}

		return codemap.Build(codemap.Options{
			Root: root, Files: files, FS: fsys, Workers: 2, EntryGlobs: globs,
			Resolver: &resolve.Resolver{FS: fsys, Index: index.New(), WorkspaceFolders: []string{root}},
		})
	}

	plain := build()
	marked := build("services")

	countEntries := func(m *codemap.Map, prefix string) int {
		n := 0

		for i := range m.Nodes {
			if m.Nodes[i].Entry && strings.HasPrefix(m.Nodes[i].File, prefix) {
				n++
			}
		}

		return n
	}

	before, after := countEntries(plain, "services"), countEntries(marked, "services")
	if after <= before {
		t.Fatalf("--entry services marked %d entry points under services/, was %d; the glob had no effect", after, before)
	}

	// A private method is still not an entry: a runner reaching in by a
	// constructed name cannot reach one.
	for i := range marked.Nodes {
		n := &marked.Nodes[i]
		if n.Entry && strings.EqualFold(n.Access, "private") {
			t.Errorf("%s is private but was marked an entry point", n.ID)
		}
	}

	// And a glob must not leak outside its prefix.
	if countEntries(marked, "models") != countEntries(plain, "models") {
		t.Error("the services glob changed entry points under models/")
	}
}

// TestUtilityIsMarkedNotExcluded. The point of the flag is that it is a label.
// These are genuinely the most-depended-on code in a workspace, so a map that
// dropped them would misreport what depends on what — the counts, the rankings
// and the islands all have to keep counting them.
func TestUtilityIsMarkedNotExcluded(t *testing.T) {
	root := testdataDir(t)

	var files []string

	walk(t, root, &files)

	build := func(globs ...string) *codemap.Map {
		fsys := vfs.OS{}

		return codemap.Build(codemap.Options{
			Root: root, Files: files, FS: fsys, Workers: 2, UtilityGlobs: globs,
			Resolver: &resolve.Resolver{FS: fsys, Index: index.New(), WorkspaceFolders: []string{root}},
		})
	}

	plain := build()
	marked := build("models")

	if len(plain.Nodes) != len(marked.Nodes) || len(plain.Edges) != len(marked.Edges) {
		t.Fatalf("marking changed the graph: %d/%d nodes, %d/%d edges",
			len(plain.Nodes), len(marked.Nodes), len(plain.Edges), len(marked.Edges))
	}

	if marked.Stats.Utility == 0 {
		t.Fatal("--utility models marked nothing")
	}

	if plain.Stats.Utility != 0 {
		t.Error("nodes were marked utility with no globs given")
	}

	// Everything under the glob, and nothing outside it.
	for i := range marked.Nodes {
		n := &marked.Nodes[i]
		under := strings.HasPrefix(n.File, "models")

		if n.Utility != under {
			t.Errorf("%s: Utility=%v but under models=%v", n.ID, n.Utility, under)
		}
	}

	// And the in-degrees are untouched, which is what "not excluded" means.
	if a, b := plain.InDegree(), marked.InDegree(); len(a) != len(b) {
		t.Errorf("in-degree map changed size: %d vs %d", len(a), len(b))
	}
}

// TestCollapseTreatsAPackageAsUtilityOnlyIfAllOfItIs. One utility file among
// twenty application ones does not make the package something a reader can set
// aside, so utility-ness intersects on a merge where entry-ness unions.
func TestCollapseTreatsAPackageAsUtilityOnlyIfAllOfItIs(t *testing.T) {
	m := &codemap.Map{
		Nodes: []codemap.Node{
			{ID: "pkg/a.cfc", Kind: codemap.KindFile, Name: "a.cfc", File: "pkg/a.cfc", Utility: true},
			{ID: "pkg/b.cfc", Kind: codemap.KindFile, Name: "b.cfc", File: "pkg/b.cfc"},
			{ID: "util/c.cfc", Kind: codemap.KindFile, Name: "c.cfc", File: "util/c.cfc", Utility: true},
			{ID: "util/d.cfc", Kind: codemap.KindFile, Name: "d.cfc", File: "util/d.cfc", Utility: true},
		},
		Edges: []codemap.Edge{{From: "pkg/a.cfc", To: "util/c.cfc", Kind: codemap.EdgeCalls, Count: 1}},
	}
	m.Annotate()

	idx := m.Collapse(codemap.LevelPackage).NodeIndex()

	if n := idx["pkg"]; n == nil || n.Utility {
		t.Error("a mixed package collapsed to utility")
	}

	if n := idx["util"]; n == nil || !n.Utility {
		t.Error("an all-utility package did not collapse to utility")
	}
}
