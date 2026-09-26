package codemap

import (
	"path"
	"slices"
	"sort"
	"strings"
)

// structuralKinds are the edges that mean "this depends on that". EdgeContains is
// excluded on purpose: it is the file→function hierarchy, not a dependency, and
// counting it makes every function look called by its own file.
func structural(kind EdgeKind) bool {
	return kind != EdgeContains
}

// Collapse rolls a map up to a coarser level, merging nodes and summing the edge
// counts between them.
//
// This is the operation that makes a large project viewable at all. A function-level
// map of a real codebase is tens of thousands of nodes — a fine thing to query and a
// useless thing to look at. The same map at package level is usually under a hundred,
// and it is the picture people mean when they ask to see the project.
//
// Self-edges are dropped: once a directory's files are one node, the calls between
// them become a loop on that node, which says only "this package has internals".
func (m *Map) Collapse(level Level) *Map {
	if level == m.Level || level == LevelFunction {
		return m
	}

	remap := make(map[string]string, len(m.Nodes))
	out := &Map{Root: m.Root, Level: level, Stats: m.Stats, Fingerprint: m.Fingerprint}
	merged := make(map[string]*Node)

	for i := range m.Nodes {
		n := &m.Nodes[i]

		target := collapseTarget(n, level)
		remap[n.ID] = target.ID

		existing, ok := merged[target.ID]
		if !ok {
			cp := target
			merged[target.ID] = &cp

			continue
		}

		// Entry-ness survives a merge: a package holding one remote method is
		// reachable from outside, and a collapse that forgot that would report the
		// whole package as dead code.
		existing.Entry = existing.Entry || target.Entry

		// Utility does the opposite: a package is infrastructure only if all of it
		// is. One utility file among twenty application ones does not make the
		// package something a reader can set aside.
		existing.Utility = existing.Utility && target.Utility
	}

	edges := make(map[edgeKey]*Edge)

	for i := range m.Edges {
		e := &m.Edges[i]
		if !structural(e.Kind) {
			continue
		}

		from, to := remap[e.From], remap[e.To]
		if from == "" || to == "" || from == to {
			continue
		}

		k := edgeKey{from, to, e.Kind}

		existing, ok := edges[k]
		if !ok {
			edges[k] = &Edge{From: from, To: to, Kind: e.Kind, Count: e.Count, Dynamic: e.Dynamic}

			continue
		}

		existing.Count += e.Count

		if !e.Dynamic {
			existing.Dynamic = false
		}
	}

	out.Nodes = sortedNodes(merged)
	out.Edges = sortedEdges(edges)
	out.Annotate()

	return out
}

func collapseTarget(n *Node, level Level) Node {
	switch level {
	case LevelFile:
		if n.Kind == KindExternal || n.File == "" {
			return *n
		}

		return Node{
			ID: FileID(n.File), Kind: KindFile, Name: path.Base(n.File),
			File: n.File, Component: n.Component, Entry: n.Entry && n.Kind != KindFunction,
			Utility: n.Utility,
		}
	case LevelPackage:
		if n.Kind == KindExternal {
			return *n
		}

		dir := dirOf(n)

		return Node{
			ID: PackageID(dir), Kind: KindPackage, Name: dir, File: dir,
			Entry: n.Entry, Abstract: true, Utility: n.Utility,
		}
	case LevelFunction, LevelCall:
		// Neither is a collapse: LevelFunction is the built level, and LevelCall is
		// a filter (see Map.CallGraph), so there is nothing to merge a node into.
		return *n
	default:
		return *n
	}
}

// Reachable keeps only what can be reached from the given node ids, following
// edges forwards. With no ids it starts from every entry point — the .cfm pages,
// the remote methods and the Application.cfc lifecycle hooks — which makes the
// result "the live code".
//
// This is a destructive filter and is never the default. A full map keeps every
// declared function whether or not anything reaches it, marked with
// [Node.Reachable] and grouped into its own [Node.Island] — a function nothing
// calls is a finding to look at, and a view that silently dropped it would report
// a tidier codebase than the one on disk. Use [Map.Detached] for the complement,
// or read Reachable/Island off the full map and let the renderer decide.
func (m *Map) Reachable(from []string) *Map {
	roots := from
	if len(roots) == 0 {
		for i := range m.Nodes {
			if m.Nodes[i].Entry {
				roots = append(roots, m.Nodes[i].ID)
			}
		}
	}

	adj := m.adjacency(true)
	seen := make(map[string]bool, len(roots))
	queue := make([]string, 0, len(roots))

	for _, r := range roots {
		if !seen[r] {
			seen[r] = true

			queue = append(queue, r)
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, next := range adj[cur] {
			if seen[next] {
				continue
			}

			seen[next] = true

			queue = append(queue, next)
		}
	}

	return m.subgraph(seen)
}

// Orphans lists the nodes nothing reaches: no structural edge points at them and
// they are not entry points. On a function-level map this is the dead-code
// candidate list, and it is the one query a call graph answers that nothing else
// can — grep finds a name, not the absence of a caller.
//
// It is a candidate list, not a verdict. A call the resolver could not follow is
// an edge this map does not have, so a function reached only through such a call
// shows up here. Read it next to Stats.Unresolved.
func (m *Map) Orphans() []Node {
	reached := make(map[string]bool)

	for i := range m.Edges {
		if structural(m.Edges[i].Kind) {
			reached[m.Edges[i].To] = true
		}
	}

	var out []Node

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Kind == KindExternal || n.Entry || reached[n.ID] {
			continue
		}

		out = append(out, *n)
	}

	return out
}

// Hubs ranks nodes by how many distinct nodes depend on them, most first. The top
// of this list is the codebase's de-facto API, whether or not anyone declared it
// one, and it is where a breaking change costs the most.
//
// Everything is ranked, utility included. Splitting the two is the caller's job
// because which one is wanted depends on the question: [Map.HubsWhere] does it.
func (m *Map) Hubs(limit int) []Node {
	return m.HubsWhere(limit, nil)
}

// HubsWhere ranks only the nodes keep accepts.
//
// It exists for the one split that is always wanted. Infrastructure outranks
// everything — on one workspace the top twelve were all of it, a context accessor
// with 1,436 callers ahead of the most-used application function at 290 — so an
// unsplit ranking answers "what is the logging component" every time, which
// nobody asked. Ranking application code separately is the question people mean,
// and ranking infrastructure separately is still worth seeing, so neither is
// dropped.
func (m *Map) HubsWhere(limit int, keep func(*Node) bool) []Node {
	in := make(map[string]int, len(m.Nodes))

	for i := range m.Edges {
		if structural(m.Edges[i].Kind) {
			in[m.Edges[i].To]++
		}
	}

	ranked := make([]Node, 0, len(in))
	idx := m.NodeIndex()

	for id := range in {
		n, ok := idx[id]
		if !ok || (keep != nil && !keep(n)) {
			continue
		}

		ranked = append(ranked, *n)
	}

	sort.Slice(ranked, func(i, j int) bool {
		a, b := in[ranked[i].ID], in[ranked[j].ID]
		if a != b {
			return a > b
		}

		return ranked[i].ID < ranked[j].ID
	})

	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}

	return ranked
}

// InDegree counts, per node, how many distinct nodes depend on it.
func (m *Map) InDegree() map[string]int {
	in := make(map[string]int, len(m.Nodes))

	for i := range m.Edges {
		if structural(m.Edges[i].Kind) {
			in[m.Edges[i].To]++
		}
	}

	return in
}

// Cycles returns the strongly connected components with more than one member, plus
// any node that depends on itself, largest first.
//
// A cycle is the thing a dependency graph shows that no amount of reading files
// does: it is invisible from inside any one of the files involved. Tarjan's
// algorithm, iterative rather than recursive, because a call graph of a large
// project has chains far deeper than a comfortable stack.
func (m *Map) Cycles() [][]string {
	adj := m.adjacency(true)

	var (
		index   int
		stack   []string
		onStack = make(map[string]bool)
		indices = make(map[string]int)
		low     = make(map[string]int)
		out     [][]string
	)

	type frame struct {
		node string
		next int
	}

	for i := range m.Nodes {
		root := m.Nodes[i].ID
		if _, done := indices[root]; done {
			continue
		}

		work := []frame{{node: root}}
		indices[root] = index
		low[root] = index
		index++

		stack = append(stack, root)
		onStack[root] = true

		for len(work) > 0 {
			f := &work[len(work)-1]
			children := adj[f.node]

			if f.next < len(children) {
				child := children[f.next]
				f.next++

				if _, seen := indices[child]; !seen {
					indices[child] = index
					low[child] = index
					index++

					stack = append(stack, child)
					onStack[child] = true
					work = append(work, frame{node: child})
				} else if onStack[child] {
					low[f.node] = min(low[f.node], indices[child])
				}

				continue
			}

			if low[f.node] == indices[f.node] {
				var component []string

				for {
					last := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[last] = false
					component = append(component, last)

					if last == f.node {
						break
					}
				}

				if len(component) > 1 || selfLoop(adj, f.node) {
					sort.Strings(component)
					out = append(out, component)
				}
			}

			node := f.node
			work = work[:len(work)-1]

			if len(work) > 0 {
				parent := work[len(work)-1].node
				low[parent] = min(low[parent], low[node])
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}

		return out[i][0] < out[j][0]
	})

	return out
}

func selfLoop(adj map[string][]string, node string) bool {
	return slices.Contains(adj[node], node)
}

// FilterUnder scopes the map to a path prefix, keeping the nodes just outside it
// that an edge reaches across.
//
// Dropping everything beyond the prefix is the obvious implementation and it hides
// the answer: scoping to a package and then keeping only the edges whose *both*
// endpoints are inside it removes every caller from outside, which is what you
// scoped the map down in order to look at. The result is a package where almost
// nothing appears to be called, and the map looks broken rather than narrow.
//
// So an edge with one end in scope keeps its far end too, marked [Node.Boundary]
// so a renderer can draw it as the edge of the world rather than as part of the
// package. Islands and roots are recomputed over what is left, because both are
// relative to the graph in front of you.
func (m *Map) FilterUnder(prefix string) *Map {
	if prefix == "" {
		return m
	}

	inScope := make(map[string]bool, len(m.Nodes))

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if strings.HasPrefix(n.File, prefix) || strings.HasPrefix(n.ID, prefix) {
			inScope[n.ID] = true
		}
	}

	keep := make(map[string]bool, len(inScope))
	for id := range inScope {
		keep[id] = true
	}

	for i := range m.Edges {
		e := &m.Edges[i]
		if !structural(e.Kind) {
			continue
		}

		if inScope[e.From] {
			keep[e.To] = true
		}

		if inScope[e.To] {
			keep[e.From] = true
		}
	}

	out := m.subgraph(keep)
	for i := range out.Nodes {
		out.Nodes[i].Boundary = !inScope[out.Nodes[i].ID]
	}

	return out
}

// Filter scopes the map strictly: only nodes under the prefix, and only edges
// between them. Use [Map.FilterUnder] unless you specifically want the hard cut —
// this one cannot show you who calls into the scope from outside.
func (m *Map) Filter(prefix string) *Map {
	if prefix == "" {
		return m
	}

	keep := make(map[string]bool, len(m.Nodes))

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if strings.HasPrefix(n.File, prefix) || strings.HasPrefix(n.ID, prefix) {
			keep[n.ID] = true
		}
	}

	return m.subgraph(keep)
}

func (m *Map) adjacency(structuralOnly bool) map[string][]string {
	adj := make(map[string][]string, len(m.Nodes))

	for i := range m.Edges {
		e := &m.Edges[i]
		if structuralOnly && !structural(e.Kind) {
			continue
		}

		adj[e.From] = append(adj[e.From], e.To)
	}

	return adj
}

func (m *Map) subgraph(keep map[string]bool) *Map {
	out := &Map{Root: m.Root, Level: m.Level, Stats: m.Stats, Fingerprint: m.Fingerprint}

	for i := range m.Nodes {
		if keep[m.Nodes[i].ID] {
			out.Nodes = append(out.Nodes, m.Nodes[i])
		}
	}

	for i := range m.Edges {
		e := &m.Edges[i]
		if keep[e.From] && keep[e.To] {
			out.Edges = append(out.Edges, *e)
		}
	}

	// Islands and roots are relative to the graph you are looking at: cutting a
	// subgraph out splits islands and creates roots that did not exist in the whole.
	out.Annotate()

	return out
}

func sortedNodes(byID map[string]*Node) []Node {
	out := make([]Node, 0, len(byID))
	for _, n := range byID {
		out = append(out, *n)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	return out
}

func sortedEdges(byKey map[edgeKey]*Edge) []Edge {
	out := make([]Edge, 0, len(byKey))
	for _, e := range byKey {
		out = append(out, *e)
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.From != b.From {
			return a.From < b.From
		}

		if a.To != b.To {
			return a.To < b.To
		}

		return a.Kind < b.Kind
	})

	return out
}

// invokes reports whether an edge means "this runs that".
//
// A route is an invocation, not a lesser relationship: a page names a route and
// the dispatcher calls the controller method. Excluding it from the call graph
// would drop exactly the paths route resolution exists to recover — on one
// workspace, 779 edges reaching 542 controller methods that nothing else calls —
// and leave the strict call graph reporting them as unreachable again.
//
// It stays a distinct EdgeKind so a reader can still tell a parsed call site from
// a resolved convention, and the viewer can filter one from the other.
func invokes(kind EdgeKind) bool {
	return kind == EdgeCalls || kind == EdgeRoute
}

// CallGraph reduces the map to functions and the calls between them, including
// the calls a dispatcher makes on a route's behalf.
//
// LevelFunction is deliberately a hybrid — functions *and* files — because three
// of the four relationships a codebase has are between files, not functions. A
// component extends a component; a page includes a page; `new Foo()` names a
// component and may never call a method on it. Dropping the file nodes drops all
// of that, which is why it is a separate level rather than the default.
//
// What survives is the question "what calls what", answered without the file layer
// in the way: shorter paths, a cleaner cycle list, and an unreferenced list that is
// strictly about functions nobody invokes.
//
// File nodes are not removed wholesale. A .cfm page is mostly top-level code with
// no enclosing function, so its file node *is* a unit of executable code and the
// caller of record for everything that page does. Removing it would silently
// delete every call a page makes — on a page-heavy codebase, most of the graph.
// So a file node is kept exactly when it takes part in a call, and dropped when it
// was only ever a container.
func (m *Map) CallGraph() *Map {
	calls := make(map[string]bool)

	for i := range m.Edges {
		e := &m.Edges[i]
		if !invokes(e.Kind) {
			continue
		}

		calls[e.From] = true
		calls[e.To] = true
	}

	keep := make(map[string]bool, len(m.Nodes))

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if n.Kind == KindFunction || n.Kind == KindExternal || calls[n.ID] {
			keep[n.ID] = true
		}
	}

	out := &Map{Root: m.Root, Level: LevelCall, Stats: m.Stats, Fingerprint: m.Fingerprint}

	for i := range m.Nodes {
		if keep[m.Nodes[i].ID] {
			out.Nodes = append(out.Nodes, m.Nodes[i])
		}
	}

	for i := range m.Edges {
		e := &m.Edges[i]
		if invokes(e.Kind) && keep[e.From] && keep[e.To] {
			out.Edges = append(out.Edges, *e)
		}
	}

	// Re-annotate rather than inherit: without the contains edges every function
	// is now grouped by who calls it rather than by which file declares it, and
	// entry-reachability changes too, since an entry .cfc file node may have gone.
	out.Annotate()

	return out
}
