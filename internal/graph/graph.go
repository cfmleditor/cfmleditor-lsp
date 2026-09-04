// Package graph provides structured graph types and renderers.
package graph

import (
	"fmt"
	"strings"
)

// Edge represents a directed edge between two nodes.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Dashed bool   `json:"dashed,omitempty"`
}

// Graph represents a directed graph with labelled nodes and edges.
type Graph struct {
	Direction string `json:"direction"` // "TD" or "LR"
	Edges     []Edge `json:"edges"`
}

// Mermaid renders the graph as a Mermaid diagram string.
func (g *Graph) Mermaid() string {
	dir := g.Direction
	if dir == "" {
		dir = "LR"
	}

	var lines []string

	lines = append(lines, "graph "+dir)
	seen := make(map[string]bool)

	for _, e := range g.Edges {
		key := e.From + "|" + e.To
		if seen[key] {
			continue
		}

		seen[key] = true

		arrow := "-->"
		if e.Dashed {
			arrow = "-.->"
		}

		lines = append(lines, fmt.Sprintf("    %s[\"%s\"] %s %s[\"%s\"]", nodeID(e.From), escapeMermaid(e.From), arrow, nodeID(e.To), escapeMermaid(e.To)))
	}

	return strings.Join(lines, "\n")
}

// DOT renders the graph in Graphviz DOT format.
func (g *Graph) DOT() string {
	dir := "LR"
	if g.Direction == "TD" || g.Direction == "TB" {
		dir = "TB"
	}

	var lines []string

	lines = append(lines, "digraph {")
	lines = append(lines, fmt.Sprintf("    rankdir=%s;", dir))
	seen := make(map[string]bool)

	for _, e := range g.Edges {
		key := e.From + "|" + e.To
		if seen[key] {
			continue
		}

		seen[key] = true

		style := ""
		if e.Dashed {
			style = " [style=dashed]"
		}

		lines = append(lines, fmt.Sprintf("    \"%s\" -> \"%s\"%s;", escapeDOT(e.From), escapeDOT(e.To), style))
	}

	lines = append(lines, "}")

	return strings.Join(lines, "\n")
}

// nodeID turns a node label into a Mermaid node identifier.
//
// Everything outside [A-Za-z0-9_] is replaced, rather than the handful of
// characters this used to list. Mermaid reads several of the others as shape
// delimiters, so leaving them in produced a diagram that would not render at
// all: every dependency label carries a "(line N)" suffix, and an ID like
// UserService_cfc_(line_0) ends at the "(", leaving the ["label"] that follows
// it as a syntax error. The label itself was already escaped by
// [escapeMermaid]; only the identifier was not.
func nodeID(name string) string {
	var b strings.Builder

	b.Grow(len(name))

	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}

	// A label made entirely of punctuation would otherwise yield an empty
	// identifier, which is a syntax error rather than an unnamed node.
	if b.Len() == 0 {
		return "node"
	}

	return b.String()
}

func escapeMermaid(s string) string {
	r := strings.NewReplacer(
		`"`, `#quot;`, `[`, `#lsqb;`, `]`, `#rsqb;`, `#`, `#35;`,
		`:`, `#colon;`, `-`, `#45;`, `(`, `#40;`, `)`, `#41;`,
		`{`, `#123;`, `}`, `#125;`, `;`, `#59;`, `>`, `#gt;`, `<`, `#lt;`,
	)

	return r.Replace(s)
}

func escapeDOT(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)

	return r.Replace(s)
}
