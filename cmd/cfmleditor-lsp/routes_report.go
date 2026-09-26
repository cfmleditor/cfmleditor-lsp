package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// routeShape classifies an unresolved route by what its segments look like.
//
// The classification is the point of the report. 292 unresolved routes is a list
// nobody works through; "135 of them have a dialog segment" is one question to ask
// whoever knows the framework, and the answer is a config change covering all 135.
func routeShape(r string) string {
	segs := strings.Split(r, ".")
	has := func(want string) bool {
		for _, s := range segs {
			if s == want {
				return true
			}
		}

		return false
	}

	switch {
	case has("dialog"):
		return "dialog"
	case has("popup"):
		return "popup"
	case hasAny(segs, "iframe", "image", "attachment", "artifact", "download", "export", "print", "grid"):
		return "asset or action verb"
	case len(segs) == 2:
		return "two segments only"
	default:
		return "other"
	}
}

func hasAny(segs []string, want ...string) bool {
	for _, s := range segs {
		for _, w := range want {
			if s == w {
				return true
			}
		}
	}

	return false
}

type routeGroup struct {
	prefix string
	routes []string
	sites  map[string][]routeFinding
	known  bool // the prefix resolves on other routes, so the controller is found
	shapes map[string]int
	total  int
}

// groupUnresolved organises the findings for a report: by leading segments, with
// every site of each distinct route, and whether that prefix resolves elsewhere.
func groupUnresolved(unresolved, resolved []routeFinding) []routeGroup {
	resolvedPrefixes := map[string]bool{}

	for i := range resolved {
		f := &resolved[i]

		resolvedPrefixes[prefixOf(f.Route)] = true
	}

	byPrefix := map[string]*routeGroup{}

	for i := range unresolved {
		f := &unresolved[i]

		p := prefixOf(f.Route)

		g, ok := byPrefix[p]
		if !ok {
			g = &routeGroup{
				prefix: p, sites: map[string][]routeFinding{},
				known: resolvedPrefixes[p], shapes: map[string]int{},
			}
			byPrefix[p] = g
		}

		if _, seen := g.sites[f.Route]; !seen {
			g.routes = append(g.routes, f.Route)
			g.shapes[routeShape(f.Route)]++
		}

		g.sites[f.Route] = append(g.sites[f.Route], *f)
		g.total++
	}

	out := make([]routeGroup, 0, len(byPrefix))

	for _, g := range byPrefix {
		sort.Strings(g.routes)
		out = append(out, *g)
	}

	sort.Slice(out, func(i, j int) bool {
		if len(out[i].routes) != len(out[j].routes) {
			return len(out[i].routes) > len(out[j].routes)
		}

		return out[i].prefix < out[j].prefix
	})

	return out
}

func prefixOf(r string) string {
	segs := strings.Split(r, ".")
	if len(segs) > 1 {
		return segs[0] + "." + segs[1]
	}

	return segs[0]
}

// writeRouteMarkdown writes a report meant to be worked through, committed beside
// the config, or pasted into a ticket for whoever knows the framework.
func writeRouteMarkdown(w io.Writer, resolved, unresolved []routeFinding, scanned int, limit int) error {
	var b strings.Builder

	b.WriteString("# Unresolved framework routes\n\n")

	distinctUn := distinctRoutes(unresolved)
	distinctRes := distinctRoutes(resolved)

	fmt.Fprintf(&b, "| | occurrences | distinct routes |\n|---|---:|---:|\n")
	fmt.Fprintf(&b, "| resolved | %d | %d |\n", len(resolved), distinctRes)
	fmt.Fprintf(&b, "| **unresolved** | **%d** | **%d** |\n", len(unresolved), distinctUn)
	fmt.Fprintf(&b, "| total | %d | %d |\n\n", scanned, distinctRes+distinctUn)

	if scanned > 0 {
		fmt.Fprintf(&b, "%.1f%% of occurrences resolve.\n\n", float64(len(resolved))*100/float64(scanned))
	}

	writeSourceTable(&b, resolved, unresolved)

	groups := groupUnresolved(unresolved, resolved)

	// The headline finding, because it says where to look: when the prefix
	// resolves on other routes the controller is being found correctly and it is
	// the method name the route does not spell, which is a different problem from
	// a controller the config cannot locate at all.
	known, unknown := 0, 0

	for _, g := range groups {
		if g.known {
			known += len(g.routes)
		} else {
			unknown += len(g.routes)
		}
	}

	b.WriteString("## Where the gap is\n\n")
	fmt.Fprintf(&b, "- **%d** of %d unresolved routes share their leading segments with routes that *do* resolve.\n",
		known, distinctUn)
	b.WriteString("  The controller is being found; it is the method name the route does not spell.\n")
	fmt.Fprintf(&b, "- **%d** have a prefix that resolves nowhere, so the controller itself is unlocated.\n\n", unknown)

	writeShapeTable(&b, unresolved)

	b.WriteString("## By leading segments\n\n")

	for i := range groups {
		writeGroup(&b, &groups[i], limit)
	}

	_, err := io.WriteString(w, b.String())

	return err
}

func writeSourceTable(b *strings.Builder, resolved, unresolved []routeFinding) {
	counts := map[string][2]int{}

	for i := range resolved {
		f := &resolved[i]

		c := counts[f.Source]
		counts[f.Source] = [2]int{c[0] + 1, c[1]}
	}

	for i := range unresolved {
		f := &unresolved[i]

		c := counts[f.Source]
		counts[f.Source] = [2]int{c[0], c[1] + 1}
	}

	kinds := make([]string, 0, len(counts))
	for k := range counts {
		kinds = append(kinds, k)
	}

	sort.Strings(kinds)

	b.WriteString("## By source\n\n| syntax | resolved | unresolved |\n|---|---:|---:|\n")

	for _, k := range kinds {
		c := counts[k]
		fmt.Fprintf(b, "| %s | %d | %d |\n", k, c[0], c[1])
	}

	b.WriteString("\n")
}

func writeShapeTable(b *strings.Builder, unresolved []routeFinding) {
	shapes := map[string]int{}
	seen := map[string]bool{}

	for i := range unresolved {
		f := &unresolved[i]

		if seen[f.Route] {
			continue
		}

		seen[f.Route] = true
		shapes[routeShape(f.Route)]++
	}

	keys := make([]string, 0, len(shapes))
	for k := range shapes {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		if shapes[keys[i]] != shapes[keys[j]] {
			return shapes[keys[i]] > shapes[keys[j]]
		}

		return keys[i] < keys[j]
	})

	b.WriteString("## By shape\n\nOne question per row, not one per route: a whole row is usually a single missing rule.\n\n")
	b.WriteString("| shape | distinct routes |\n|---|---:|\n")

	for _, k := range keys {
		fmt.Fprintf(b, "| %s | %d |\n", k, shapes[k])
	}

	b.WriteString("\n")
}

func writeGroup(b *strings.Builder, g *routeGroup, limit int) {
	status := "controller not located"
	if g.known {
		status = "controller resolves on other routes"
	}

	fmt.Fprintf(b, "### `%s.*` — %d routes, %d occurrences\n\n", g.prefix, len(g.routes), g.total)
	fmt.Fprintf(b, "_%s._\n\n", status)

	shown := 0

	for _, r := range g.routes {
		if limit > 0 && shown >= limit {
			fmt.Fprintf(b, "\n_… and %d more routes under this prefix._\n", len(g.routes)-shown)

			break
		}

		shown++

		sites := g.sites[r]
		fmt.Fprintf(b, "- `%s` — %s\n", r, routeShape(r))

		for i := range sites {
			s := &sites[i]

			if i >= 3 {
				fmt.Fprintf(b, "  - _… %d more %s_\n", len(sites)-i, plural(len(sites)-i, "site", "sites"))

				break
			}

			fmt.Fprintf(b, "  - `%s:%d` (%s `%s`)\n", s.File, s.Line, s.Source, s.Name)
		}
	}

	b.WriteString("\n")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}

func distinctRoutes(f []routeFinding) int {
	seen := map[string]bool{}

	for i := range f {
		x := &f[i]

		seen[x.Route] = true
	}

	return len(seen)
}
