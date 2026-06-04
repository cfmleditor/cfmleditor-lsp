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

func nodeID(name string) string {
	r := strings.NewReplacer(".", "_", "/", "_", ":", "_", " ", "_", "-", "_")

	return r.Replace(name)
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
