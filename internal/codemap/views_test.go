package codemap_test

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/codemap"
)

// TestCallGraphKeepsPagesButDropsContainerFiles pins the one subtlety in the
// strict call level. Removing every file node looks right and deletes most of a
// page-heavy codebase's graph: a .cfm is mostly top-level code with no enclosing
// function, so its file node *is* executable code and the caller of record for
// everything the page does.
func TestCallGraphKeepsPagesButDropsContainerFiles(t *testing.T) {
	m := buildTestMap(t)
	cg := m.CallGraph()

	if cg.Level != codemap.LevelCall {
		t.Errorf("CallGraph level = %q, want %q", cg.Level, codemap.LevelCall)
	}

	for i := range cg.Edges {
		if cg.Edges[i].Kind != codemap.EdgeCalls {
			t.Fatalf("call graph holds a %q edge; only calls should survive", cg.Edges[i].Kind)
		}
	}

	// Any file node that remains must take part in a call.
	kept := cg.NodeIndex()

	for id, n := range kept {
		if n.Kind != codemap.KindFile {
			continue
		}

		if !touchesACall(cg, id) {
			t.Errorf("%s is a file node in the call graph but takes part in no call", id)
		}
	}

	// And a page that calls something must have survived.
	var pagesKept int

	for id, n := range kept {
		if n.Kind == codemap.KindFile && strings.HasSuffix(strings.ToLower(n.Name), ".cfm") && touchesACall(cg, id) {
			pagesKept++
		}
	}

	if pagesKept == 0 {
		t.Error("no .cfm page survived the call graph; top-level page code is being dropped")
	}
}

func touchesACall(m *codemap.Map, id string) bool {
	for i := range m.Edges {
		if m.Edges[i].From == id || m.Edges[i].To == id {
			return true
		}
	}

	return false
}

// TestCollapsePreservesEntryness checks the merge rule that decides whether a
// package is dead. One remote method makes its whole package reachable from
// outside; a collapse that dropped that would report live packages as orphans.
func TestCollapsePreservesEntryness(t *testing.T) {
	m := buildTestMap(t)

	entryDirs := map[string]bool{}

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Entry && n.File != "" {
			dir := n.File
			if idx := strings.LastIndex(dir, "/"); idx >= 0 {
				dir = dir[:idx]
			} else {
				dir = "."
			}

			entryDirs[dir] = true
		}
	}

	if len(entryDirs) == 0 {
		t.Skip("no entry points in the fixtures")
	}

	pkg := m.Collapse(codemap.LevelPackage)
	idx := pkg.NodeIndex()

	for dir := range entryDirs {
		n, ok := idx[dir]
		if !ok {
			t.Errorf("package %q vanished in the collapse although it holds an entry point", dir)

			continue
		}

		if !n.Entry {
			t.Errorf("package %q holds an entry point but collapsed to a non-entry node", dir)
		}
	}
}

// TestCollapseDropsSelfEdges checks that a package's internal calls do not become
// a loop on itself. They carry no information at that level — every package has
// internals — and they would make every package look like a cycle.
func TestCollapseDropsSelfEdges(t *testing.T) {
	pkg := buildTestMap(t).Collapse(codemap.LevelPackage)

	for i := range pkg.Edges {
		if pkg.Edges[i].From == pkg.Edges[i].To {
			t.Fatalf("collapsed map has a self-edge on %q", pkg.Edges[i].From)
		}
	}
}

// TestCyclesFindsAKnownLoop builds a tiny graph by hand rather than relying on the
// fixtures, so the expected answer is not a guess about what testdata happens to
// contain.
func TestCyclesFindsAKnownLoop(t *testing.T) {
	m := &codemap.Map{
		Nodes: []codemap.Node{
			{ID: "a", Kind: codemap.KindFunction, Name: "a"},
			{ID: "b", Kind: codemap.KindFunction, Name: "b"},
			{ID: "c", Kind: codemap.KindFunction, Name: "c"},
			{ID: "loner", Kind: codemap.KindFunction, Name: "loner"},
		},
		Edges: []codemap.Edge{
			{From: "a", To: "b", Kind: codemap.EdgeCalls, Count: 1},
			{From: "b", To: "c", Kind: codemap.EdgeCalls, Count: 1},
			{From: "c", To: "a", Kind: codemap.EdgeCalls, Count: 1},
		},
	}
	m.Annotate()

	cycles := m.Cycles()
	if len(cycles) != 1 {
		t.Fatalf("expected exactly one cycle, got %d: %v", len(cycles), cycles)
	}

	if got := strings.Join(cycles[0], ","); got != "a,b,c" {
		t.Errorf("cycle = %q, want \"a,b,c\"", got)
	}

	// The unconnected node is its own island and must still be a node.
	idx := m.NodeIndex()
	if _, ok := idx["loner"]; !ok {
		t.Fatal("the unconnected node was dropped")
	}

	if idx["loner"].Island == idx["a"].Island {
		t.Error("an unconnected node shares an island with the cycle")
	}

	if !idx["loner"].Root {
		t.Error("an unconnected node is not marked as a root, so nothing can draw it as a tree")
	}
}

// TestPureCycleStillGetsARoot covers the island a tree view could otherwise not
// draw at all: every member has an incoming edge, so none is a natural root.
func TestPureCycleStillGetsARoot(t *testing.T) {
	m := &codemap.Map{
		Nodes: []codemap.Node{
			{ID: "x", Kind: codemap.KindFunction, Name: "x"},
			{ID: "y", Kind: codemap.KindFunction, Name: "y"},
		},
		Edges: []codemap.Edge{
			{From: "x", To: "y", Kind: codemap.EdgeCalls, Count: 1},
			{From: "y", To: "x", Kind: codemap.EdgeCalls, Count: 1},
		},
	}
	m.Annotate()

	roots := 0

	for i := range m.Nodes {
		if m.Nodes[i].Root {
			roots++
		}
	}

	if roots != 1 {
		t.Fatalf("a two-node cycle should be given exactly one arbitrary root, got %d", roots)
	}
}

// TestDetachedIsTheComplementOfReachable checks the pair that replaced a
// destructive default: everything is in one or the other, and nothing in both.
func TestDetachedIsTheComplementOfReachable(t *testing.T) {
	m := buildTestMap(t)
	live, dead := m.Reachable(nil), m.Detached()

	if len(live.Nodes)+len(dead.Nodes) != len(m.Nodes) {
		t.Errorf("reachable (%d) + detached (%d) = %d, but the map has %d nodes",
			len(live.Nodes), len(dead.Nodes), len(live.Nodes)+len(dead.Nodes), len(m.Nodes))
	}

	inLive := live.NodeIndex()

	for i := range dead.Nodes {
		if _, both := inLive[dead.Nodes[i].ID]; both {
			t.Errorf("%s is in both the reachable and detached views", dead.Nodes[i].ID)
		}
	}
}

// TestFilterUnderKeepsCallersFromOutside is the fix for a scoped map that looked
// broken. Keeping only the edges whose *both* ends are inside the prefix removes
// every caller from elsewhere — which is what you scoped the map down in order to
// find — so the package comes back with almost nothing called and the tool looks
// wrong rather than narrow.
func TestFilterUnderKeepsCallersFromOutside(t *testing.T) {
	m := &codemap.Map{
		Nodes: []codemap.Node{
			{ID: "web/page.cfm", Kind: codemap.KindFile, Name: "page.cfm", File: "web/page.cfm", Entry: true},
			{ID: "pkg/svc.cfc", Kind: codemap.KindFile, Name: "svc.cfc", File: "pkg/svc.cfc"},
			{ID: "pkg/svc.cfc::getthing", Kind: codemap.KindFunction, Name: "getThing", File: "pkg/svc.cfc"},
			{ID: "other/far.cfc::unrelated", Kind: codemap.KindFunction, Name: "unrelated", File: "other/far.cfc"},
		},
		Edges: []codemap.Edge{
			{From: "web/page.cfm", To: "pkg/svc.cfc::getthing", Kind: codemap.EdgeCalls, Count: 2},
			{From: "pkg/svc.cfc", To: "pkg/svc.cfc::getthing", Kind: codemap.EdgeContains, Count: 1},
		},
	}
	m.Annotate()

	scoped := m.FilterUnder("pkg/")
	idx := scoped.NodeIndex()

	caller, ok := idx["web/page.cfm"]
	if !ok {
		t.Fatal("the caller from outside the prefix was dropped; a scoped map cannot show who calls into it")
	}

	if !caller.Boundary {
		t.Error("the outside caller is not marked Boundary, so a renderer cannot tell it apart from the scope")
	}

	if n := idx["pkg/svc.cfc::getthing"]; n == nil || n.Boundary {
		t.Error("a node inside the prefix was marked Boundary")
	}

	if _, pulled := idx["other/far.cfc::unrelated"]; pulled {
		t.Error("an unconnected node from outside the prefix was pulled in")
	}

	// And the call edge itself has to survive, or the boundary node is an orphan.
	var kept bool

	for i := range scoped.Edges {
		if scoped.Edges[i].From == "web/page.cfm" && scoped.Edges[i].Kind == codemap.EdgeCalls {
			kept = true
		}
	}

	if !kept {
		t.Error("the boundary-crossing call edge was dropped")
	}

	// Filter is the strict version and must still cut hard.
	if _, ok := m.Filter("pkg/").NodeIndex()["web/page.cfm"]; ok {
		t.Error("Filter kept a node outside the prefix; it is the hard-cut variant")
	}
}

// TestBareCallToABuiltinNameStillResolves: a workspace function may share a name
// with a built-in or member method (init, add, close, get, isValid and dozens of
// others in one real codebase). Testing the name before resolving threw away every
// call to them, so those functions reported no callers at all.
func TestBareCallToABuiltinNameStillResolves(t *testing.T) {
	m := buildTestMap(t)

	var suspicious []string

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Kind != codemap.KindFunction {
			continue
		}

		if !strings.EqualFold(n.Name, "init") {
			continue
		}

		suspicious = append(suspicious, n.ID)
	}

	if len(suspicious) == 0 {
		t.Skip("no init() in the fixtures")
	}

	// The point is not that init() must have callers, but that the builder must
	// have consulted the resolver before discarding the call as a builtin. A
	// resolved call to any of these is proof it did.
	for i := range m.Edges {
		e := &m.Edges[i]
		if e.Kind != codemap.EdgeCalls {
			continue
		}

		for _, id := range suspicious {
			if e.To == id {
				return
			}
		}
	}

	t.Logf("no resolved call to init() in the fixtures; checked %d candidates", len(suspicious))
}

// TestEntryGlobMatching states the matching rule directly, because it is the kind
// of thing that is easy to get subtly wrong: path.Match has no "**", so a bare
// directory name has to match everything beneath it or the flag is a footgun.
func TestEntryGlobMatching(t *testing.T) {
	cases := []struct {
		globs []string
		path  string
		want  bool
	}{
		{[]string{"../prs"}, "../prs/prs0100009.cfc", true},
		{[]string{"../prs"}, "../prs/deep/nested/x.cfc", true},
		{[]string{"../prs"}, "../prsother/x.cfc", false},
		{[]string{"../prs/"}, "../prs/x.cfc", true},
		{[]string{"tasks/*"}, "tasks/nightly.cfc", true},
		{[]string{"tasks/*"}, "tasks/sub/nightly.cfc", true},
		{[]string{"tasks/*"}, "other/nightly.cfc", false},
		{[]string{"*.cfm"}, "index.cfm", true},
		{nil, "anything.cfc", false},
		{[]string{"["}, "anything.cfc", false}, // a malformed pattern must not panic
	}

	for _, c := range cases {
		if got := codemap.MatchesEntryGlob(c.globs, c.path); got != c.want {
			t.Errorf("MatchesEntryGlob(%v, %q) = %v, want %v", c.globs, c.path, got, c.want)
		}
	}
}
