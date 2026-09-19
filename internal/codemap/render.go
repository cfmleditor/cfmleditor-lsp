package codemap

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/graph"
)

// WriteJSON writes the map as indented JSON. This is the canonical artifact: every
// other format here is a view of it, and a consumer that wants a question none of
// them answers should read this rather than parse a picture.
func (m *Map) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(m)
}

// WriteJSONL writes one node or edge per line. At a few hundred thousand edges the
// indented form is a file no editor will open and no stream processor should have
// to buffer; this one pipes into jq line by line.
func (m *Map) WriteJSONL(w io.Writer) error {
	enc := json.NewEncoder(w)

	for i := range m.Nodes {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			Node
		}{"node", m.Nodes[i]}); err != nil {
			return err
		}
	}

	for i := range m.Edges {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			Edge
		}{"edge", m.Edges[i]}); err != nil {
			return err
		}
	}

	return nil
}

// Graph converts the map to the shared renderer's shape, for Mermaid and plain DOT.
// Dynamic edges come out dashed, which is the one distinction that format can carry.
func (m *Map) Graph() *graph.Graph {
	idx := m.NodeIndex()
	g := &graph.Graph{Direction: "LR"}

	for i := range m.Edges {
		e := &m.Edges[i]
		if !structural(e.Kind) {
			continue
		}

		g.Edges = append(g.Edges, graph.Edge{
			From:   label(idx, e.From),
			To:     label(idx, e.To),
			Dashed: e.Dynamic,
		})
	}

	return g
}

func label(idx map[string]*Node, id string) string {
	n, ok := idx[id]
	if !ok {
		return id
	}

	if n.Kind == KindFunction {
		return path.Base(n.File) + "." + n.Name
	}

	if n.Kind == KindPackage {
		return n.Name
	}

	return n.Name
}

// WriteDOT writes Graphviz DOT with a cluster per directory, node colour by kind
// and edge style by relationship.
//
// This is the richer sibling of Map.Graph().DOT(): clusters are what make a
// mid-sized graph readable, and Graphviz is the only one of these formats that
// does the layout well enough for them to be worth expressing.
func (m *Map) WriteDOT(w io.Writer) error {
	var b strings.Builder

	b.WriteString("digraph codemap {\n")
	b.WriteString("  rankdir=LR;\n  compound=true;\n")
	b.WriteString("  node [shape=box style=\"rounded,filled\" fontname=\"Helvetica\" fontsize=10];\n")
	b.WriteString("  edge [fontname=\"Helvetica\" fontsize=8];\n")

	groups := make(map[string][]*Node)
	order := []string{}

	for i := range m.Nodes {
		n := &m.Nodes[i]
		dir := dirOf(n)

		if _, ok := groups[dir]; !ok {
			order = append(order, dir)
		}

		groups[dir] = append(groups[dir], n)
	}

	sort.Strings(order)

	for i, dir := range order {
		fmt.Fprintf(&b, "  subgraph cluster_%d {\n    label=%q;\n    style=dotted;\n", i, dir)

		for _, n := range groups[dir] {
			fmt.Fprintf(&b, "    %q [label=%q fillcolor=%q];\n", n.ID, dotLabel(n), fillFor(n))
		}

		b.WriteString("  }\n")
	}

	for i := range m.Edges {
		e := &m.Edges[i]
		if !structural(e.Kind) {
			continue
		}

		attrs := []string{fmt.Sprintf("color=%q", colorFor(e.Kind))}
		if e.Dynamic {
			attrs = append(attrs, "style=dashed")
		}

		if e.Count > 1 {
			attrs = append(attrs, fmt.Sprintf("label=%q", fmt.Sprint(e.Count)))
		}

		fmt.Fprintf(&b, "  %q -> %q [%s];\n", e.From, e.To, strings.Join(attrs, " "))
	}

	b.WriteString("}\n")

	_, err := io.WriteString(w, b.String())

	return err
}

func dotLabel(n *Node) string {
	if n.Kind == KindFunction {
		return n.Name + "()"
	}

	return n.Name
}

func fillFor(n *Node) string {
	if n.Entry {
		return "#ffe0b2"
	}

	switch n.Kind {
	case KindFunction:
		return "#e3f2fd"
	case KindFile:
		return "#f1f8e9"
	case KindPackage:
		return "#ede7f6"
	case KindExternal:
		return "#fbe9e7"
	default:
		return "#ffffff"
	}
}

func colorFor(kind EdgeKind) string {
	switch kind {
	case EdgeCalls:
		return "#546e7a"
	case EdgeInstantiates:
		return "#8e24aa"
	case EdgeExtends:
		return "#2e7d32"
	case EdgeIncludes:
		return "#ef6c00"
	case EdgeRoute:
		return "#00838f"
	case EdgeContains:
		return "#bdbdbd"
	default:
		return "#000000"
	}
}

// WriteMermaid writes a Mermaid diagram, for pasting into a README or a ticket.
//
// It is capped, and that is not a limitation to work around: Mermaid renders in a
// browser and falls over in the low thousands of nodes. A map big enough to need a
// cap should be collapsed to package level first, where it fits and says something.
func (m *Map) WriteMermaid(w io.Writer, maxEdges int) error {
	g := m.Graph()
	if maxEdges > 0 && len(g.Edges) > maxEdges {
		g.Edges = g.Edges[:maxEdges]
	}

	_, err := io.WriteString(w, g.Mermaid()+"\n")

	return err
}

// WriteText writes the summary a terminal wants: the counts, then the findings that
// are worth acting on. A 40,000-node picture is not a report; these five lists are.
func (m *Map) WriteText(w io.Writer, limit int) error {
	var b strings.Builder

	fmt.Fprintf(&b, "Code map of %s (%s level)\n\n", m.Root, m.Level)
	fmt.Fprintf(&b, "  %d nodes, %d edges\n", len(m.Nodes), len(m.Edges))
	fmt.Fprintf(&b, "  %d files, %d functions\n", m.Stats.Files, m.Stats.Functions)
	fmt.Fprintf(&b, "  %d call sites: %d resolved, %d dynamic, %d unresolved, %d builtin\n",
		m.Stats.CallSites, m.Stats.Resolved, m.Stats.Dynamic, m.Stats.Unresolved, m.Stats.Builtin)

	if m.Stats.CallSites > 0 {
		pct := float64(m.Stats.Resolved) * 100 / float64(m.Stats.CallSites)
		fmt.Fprintf(&b, "  %.1f%% of call sites resolved to a definition\n", pct)
	}

	if m.Stats.Unreadable > 0 {
		fmt.Fprintf(&b, "  %d files unreadable or binary\n", m.Stats.Unreadable)
	}

	if m.Stats.Routes > 0 {
		resolved := m.Stats.Routes - m.Stats.RoutesUnresolved
		fmt.Fprintf(&b, "  %d framework routes: %d resolved (%.1f%%), %d not\n",
			m.Stats.Routes, resolved, float64(resolved)*100/float64(m.Stats.Routes), m.Stats.RoutesUnresolved)
	}

	if m.Stats.Utility > 0 {
		fmt.Fprintf(&b, "  %d nodes marked utility\n", m.Stats.Utility)
	}

	fmt.Fprintf(&b, "  %d islands, %d nodes no entry point reaches\n", m.Stats.Islands, m.Stats.Detached)
	fmt.Fprintf(&b, "  built in %dms\n", m.Stats.BuildMillis)

	islands := m.Islands()
	if len(islands) > 1 {
		shown := islands[1:]
		if limit > 0 && len(shown) > limit {
			shown = shown[:limit]
		}

		fmt.Fprintf(&b, "\nDetached islands (%d besides the main graph):\n", len(islands)-1)

		idx := m.NodeIndex()

		for i := range shown {
			isl := &shown[i]

			live := ""
			if isl.Reachable {
				live = " [reachable]"
			}

			fmt.Fprintf(&b, "  island %d: %d nodes%s, rooted at %s\n",
				isl.ID, len(isl.Nodes), live, strings.Join(describeAll(idx, trim(isl.Roots, 3)), ", "))
		}
	}

	hubs := m.Hubs(limit)
	in := m.InDegree()

	if len(hubs) > 0 {
		fmt.Fprintf(&b, "\nMost depended on (%d):\n", len(hubs))

		for _, n := range hubs {
			mark := ""
			if n.Utility {
				mark = "  [utility]"
			}

			fmt.Fprintf(&b, "  %5d  %s%s\n", in[n.ID], describe(&n), mark)
		}
	}

	cycles := m.Cycles()
	if len(cycles) > 0 {
		shown := cycles
		if limit > 0 && len(shown) > limit {
			shown = shown[:limit]
		}

		fmt.Fprintf(&b, "\nCycles (%d, largest first):\n", len(cycles))

		for _, c := range shown {
			fmt.Fprintf(&b, "  %d nodes: %s\n", len(c), strings.Join(trim(c, 6), " -> "))
		}
	}

	orphans := m.Orphans()
	if len(orphans) > 0 {
		shown := orphans
		if limit > 0 && len(shown) > limit {
			shown = shown[:limit]
		}

		fmt.Fprintf(&b, "\nUnreferenced (%d — candidates, not a verdict; see unresolved count above):\n", len(orphans))

		for i := range shown {
			fmt.Fprintf(&b, "  %s\n", describe(&shown[i]))
		}
	}

	_, err := io.WriteString(w, b.String())

	return err
}

func describe(n *Node) string {
	if n.Kind == KindFunction {
		return fmt.Sprintf("%s:%d %s()", n.File, n.Line+1, n.Name)
	}

	if n.File != "" {
		return n.File
	}

	return n.Name
}

// describeAll maps node ids back to something a reader can act on. An id that is
// not a node is a trim marker and passes straight through.
func describeAll(idx map[string]*Node, ids []string) []string {
	out := make([]string, 0, len(ids))

	for _, id := range ids {
		n, ok := idx[id]
		if !ok {
			out = append(out, id)

			continue
		}

		out = append(out, describe(n))
	}

	return out
}

func trim(items []string, most int) []string {
	if len(items) <= most {
		return items
	}

	out := append([]string{}, items[:most]...)

	return append(out, fmt.Sprintf("… +%d more", len(items)-most))
}
