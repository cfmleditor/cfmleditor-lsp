package codemap

import "sort"

// Annotate fills in the per-node structure that nothing can compute from a single
// file: which island each node sits in, whether an entry point reaches it, and
// which nodes are the roots a tree view of each island should start from.
//
// It runs at the end of every build and again after any view that changes the node
// set, because all three answers are properties of the whole graph. Collapsing to
// package level merges islands; filtering to a subtree creates new ones.
//
// The point of this pass is that unreachable code stays *in* the map. A function
// no .cfm and no remote method can reach is not dropped and not quietly folded
// into the main graph — it becomes an island with its own root, which a renderer
// can lay out as a separate tree and a reader can recognise as a detached
// subsystem. Deleting it instead would make the map agree with itself and disagree
// with the codebase.
func (m *Map) Annotate() {
	islands := m.assignIslands()
	m.markReachable()
	m.markRoots()

	m.Stats.Islands = islands
	m.Stats.Detached = 0

	for i := range m.Nodes {
		if !m.Nodes[i].Reachable {
			m.Stats.Detached++
		}
	}
}

// assignIslands labels each node with its connected component over the undirected
// graph, and returns how many there are. Island numbers are assigned in node-id
// order so they are stable between runs — a number that shuffled on every build
// would make two maps undiffable.
//
// This walk counts the file→function `contains` edges, unlike every other
// traversal here. Islands are the grouping a drawing hangs off, and a function
// belongs beside the file that declares it: without contains, every uncalled
// helper became a one-node island, which turned "103 islands" into a count of
// uncalled functions rather than of disconnected subsystems, and scattered a
// file's own methods across the canvas. With it, an island is a component and
// everything it drags along, and a detached island is a genuinely separate piece
// of the codebase.
//
// Reachability deliberately does not count contains — see markReachable.
func (m *Map) assignIslands() int {
	adj := make(map[string][]string, len(m.Nodes))

	for i := range m.Edges {
		e := &m.Edges[i]
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}

	island := make(map[string]int, len(m.Nodes))
	next := 0

	ids := make([]string, len(m.Nodes))
	for i := range m.Nodes {
		ids[i] = m.Nodes[i].ID
	}

	sort.Strings(ids)

	for _, id := range ids {
		if _, seen := island[id]; seen {
			continue
		}

		island[id] = next
		queue := []string{id}

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]

			for _, n := range adj[cur] {
				if _, seen := island[n]; seen {
					continue
				}

				island[n] = next

				queue = append(queue, n)
			}
		}

		next++
	}

	for i := range m.Nodes {
		m.Nodes[i].Island = island[m.Nodes[i].ID]
	}

	return next
}

// markReachable does a forward walk from every entry point, over dependency edges
// only.
//
// Excluding `contains` is the whole point. Reaching a file does not reach the
// functions it declares — that is the difference between "this page runs" and
// "every method on this page is called". Following contains would mark every
// method of every live component as reachable, which is the one thing this field
// is for ruling out, and would report a codebase with no dead code at all.
//
// A .cfm page's top-level code needs no special case: the file node *is* that
// code, since callerID attributes a call outside any function to the file itself.
func (m *Map) markReachable() {
	adj := make(map[string][]string, len(m.Nodes))

	for i := range m.Edges {
		e := &m.Edges[i]
		if !structural(e.Kind) {
			continue
		}

		adj[e.From] = append(adj[e.From], e.To)
	}

	seen := make(map[string]bool, len(m.Nodes))

	var queue []string

	for i := range m.Nodes {
		if m.Nodes[i].Entry && !seen[m.Nodes[i].ID] {
			seen[m.Nodes[i].ID] = true

			queue = append(queue, m.Nodes[i].ID)
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, n := range adj[cur] {
			if seen[n] {
				continue
			}

			seen[n] = true

			queue = append(queue, n)
		}
	}

	for i := range m.Nodes {
		m.Nodes[i].Reachable = seen[m.Nodes[i].ID]
	}
}

// markRoots names, per island, the nodes nothing in the graph depends on. Those are
// where a tree drawing of that island starts.
//
// An island can have none — a set of components that only call each other is a
// cycle with no top. Leaving it rootless would make it the one island a tree view
// could not draw, so its lowest id is named root. That is arbitrary and deliberate:
// an arbitrary root renders, and no root does not.
func (m *Map) markRoots() {
	in := make(map[string]int, len(m.Nodes))

	for i := range m.Edges {
		if structural(m.Edges[i].Kind) {
			in[m.Edges[i].To]++
		}
	}

	hasRoot := make(map[int]bool)
	lowest := make(map[int]string)

	for i := range m.Nodes {
		n := &m.Nodes[i]
		n.Root = in[n.ID] == 0

		if n.Root {
			hasRoot[n.Island] = true
		}

		if cur, ok := lowest[n.Island]; !ok || n.ID < cur {
			lowest[n.Island] = n.ID
		}
	}

	for i := range m.Nodes {
		n := &m.Nodes[i]
		if !hasRoot[n.Island] && lowest[n.Island] == n.ID {
			n.Root = true
		}
	}
}

// Island is one connected piece of the graph.
type Island struct {
	ID        int      `json:"id"`
	Nodes     []string `json:"nodes"`
	Roots     []string `json:"roots"`
	Reachable bool     `json:"reachable"` // any entry point reaches this island
}

// Islands groups the map's nodes into their connected components, largest first.
// Island 0 of a healthy application holds most of the codebase; the rest are the
// detached trees — dead subsystems, or live ones the resolvers cannot follow into.
func (m *Map) Islands() []Island {
	byID := make(map[int]*Island)

	for i := range m.Nodes {
		n := &m.Nodes[i]

		isl, ok := byID[n.Island]
		if !ok {
			isl = &Island{ID: n.Island}
			byID[n.Island] = isl
		}

		isl.Nodes = append(isl.Nodes, n.ID)

		if n.Root {
			isl.Roots = append(isl.Roots, n.ID)
		}

		if n.Reachable {
			isl.Reachable = true
		}
	}

	out := make([]Island, 0, len(byID))
	for _, isl := range byID {
		sort.Strings(isl.Nodes)
		sort.Strings(isl.Roots)
		out = append(out, *isl)
	}

	sort.Slice(out, func(i, j int) bool {
		if len(out[i].Nodes) != len(out[j].Nodes) {
			return len(out[i].Nodes) > len(out[j].Nodes)
		}

		return out[i].ID < out[j].ID
	})

	return out
}

// Detached returns the map with every node an entry point reaches removed, leaving
// exactly the parts that are not connected to anything that runs.
//
// This is the complement of Reachable(nil), and the pair is the point: Reachable
// answers "what runs", Detached answers "what is left over", and neither is the
// default because the default keeps both. A detached island is a finding, not
// something to filter away before looking.
func (m *Map) Detached() *Map {
	keep := make(map[string]bool, len(m.Nodes))

	for i := range m.Nodes {
		if !m.Nodes[i].Reachable {
			keep[m.Nodes[i].ID] = true
		}
	}

	return m.subgraph(keep)
}
