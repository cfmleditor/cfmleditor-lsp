//go:build ignore

// candidates_report writes a Markdown report of what `unresolved --json
// --candidates` found, one file per scan, for a person to evaluate: what each
// group of findings is missing, whether the method exists anywhere, and the
// liberal matches the scan offers for it, every one with its evidence.
//
//	go run scripts/candidates_report.go [-out <dir>] [-baseline <dir>] name=report.json[@root] ...
//
// A root makes the report's paths relative to it. With -baseline, each scan is
// also compared finding by finding with <baseline>/<name>.json and the
// findings removed and added are printed: a count of either alone hides one
// file breaking while another is fixed. Nothing here resolves a finding; it
// only arranges them.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type candidate struct {
	File       string   `json:"file"`
	Line       uint32   `json:"line"`
	Confidence string   `json:"confidence"`
	Basis      []string `json:"basis"`
}

type finding struct {
	File           string      `json:"file"`
	Line           uint32      `json:"line"`
	Caller         string      `json:"caller"`
	Variable       string      `json:"variable"`
	Function       string      `json:"function"`
	Reason         string      `json:"reason"`
	Category       string      `json:"category"`
	Candidates     []candidate `json:"candidates"`
	CandidateCount int         `json:"candidateCount"`
	Definitions    *int        `json:"definitions"`
	Unchecked      int         `json:"unchecked"`
}

type group struct {
	key      string
	findings []finding
}

var categories = []string{"variable", "return-type", "method", "object"}

var categoryTitle = map[string]string{
	"variable":    "Variable definitions — a receiver whose component is unknown",
	"return-type": "Return types — a call chained on a method that declares no component",
	"method":      "Method definitions — a method not found where it was looked for",
	"object":      "Object references — a component path that names no file",
}

func main() {
	out := flag.String("out", "resolution-candidates", "directory to write the reports to")
	baseline := flag.String("baseline", "", "directory of earlier <name>.json reports to compare with")
	flag.Parse()

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}

	var index []string

	for _, arg := range flag.Args() {
		name, path, ok := strings.Cut(arg, "=")
		if !ok {
			fail(fmt.Errorf("argument %q is not name=report.json[@root]", arg))
		}

		path, root, _ := strings.Cut(path, "@")
		relRoot = strings.TrimSuffix(root, "/")

		data, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}

		var findings []finding
		if err := json.Unmarshal(data, &findings); err != nil {
			fail(fmt.Errorf("%s: %w", path, err))
		}

		md := report(name, findings)
		if err := os.WriteFile(filepath.Join(*out, name+".md"), []byte(md), 0o644); err != nil {
			fail(err)
		}

		index = append(index, summaryRow(name, findings))

		if *baseline != "" {
			compare(name, filepath.Join(*baseline, name+".json"), findings)
		}
	}

	head := "| Scan | Findings | Object | Variable | Return type | Method | Method defined nowhere | With a high-confidence candidate |\n|---|---:|---:|---:|---:|---:|---:|---:|\n"
	if err := os.WriteFile(filepath.Join(*out, "summary.md"), []byte(head+strings.Join(index, "")), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "candidates_report:", err)
	os.Exit(1)
}

var (
	corpusRe = regexp.MustCompile(`^.*/corpus/[^/]+/`)
	relRoot  string
)

func rel(path string) string {
	if relRoot != "" {
		if r, ok := strings.CutPrefix(path, relRoot+"/"); ok {
			return r
		}
	}

	return corpusRe.ReplaceAllString(path, "")
}

var uncheckedRe = regexp.MustCompile(`; \d+ (?:inherited )?calls? not checked`)

// compare prints how the findings differ from the baseline's, finding by
// finding: a count changed only by a number in its reason is not a change.
func compare(name, path string, findings []finding) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("%-10s %6d  (no baseline: %v)\n", name, len(findings), err)

		return
	}

	var before []finding
	if err := json.Unmarshal(data, &before); err != nil {
		fail(fmt.Errorf("%s: %w", path, err))
	}

	key := func(f *finding) string {
		return fmt.Sprintf("%s\x00%d\x00%s\x00%s", rel(f.File), f.Line, f.Function, uncheckedRe.ReplaceAllString(f.Reason, ""))
	}

	counts := map[string]int{}
	for i := range before {
		counts[key(&before[i])]++
	}

	added := 0

	for i := range findings {
		k := key(&findings[i])
		if counts[k] > 0 {
			counts[k]--
		} else {
			added++
		}
	}

	removed := 0
	for _, n := range counts {
		removed += n
	}

	fmt.Printf("%-10s %6d -> %6d  removed %5d  added %5d\n", name, len(before), len(findings), removed, added)
}

func top(f *finding) *candidate {
	if len(f.Candidates) == 0 {
		return nil
	}

	return &f.Candidates[0]
}

func confidence(f *finding) string {
	if c := top(f); c != nil {
		return c.Confidence
	}

	return "none"
}

func defined(f *finding) int {
	if f.Definitions == nil {
		return -1
	}

	return *f.Definitions
}

func summaryRow(name string, fs []finding) string {
	by := map[string]int{}
	nowhere, high := 0, 0

	for i := range fs {
		f := &fs[i]
		by[f.Category]++

		if defined(f) == 0 {
			nowhere++
		}

		if confidence(f) == "high" {
			high++
		}
	}

	return fmt.Sprintf("| [%s](%s.md) | %d | %d | %d | %d | %d | %d | %d |\n", name, name, len(fs), by["object"], by["variable"], by["return-type"], by["method"], nowhere, high)
}

// groupKey is what a finding is grouped by: for a receiver, its name and the
// component offered for it; for a method, its name; for an object, the reason.
func groupKey(f *finding) string {
	switch f.Category {
	case "variable", "return-type":
		k := strings.ToLower(f.Variable)
		if f.Category == "return-type" {
			k = f.Reason
			if i := strings.Index(k, " (chain"); i >= 0 {
				k = k[:i]
			}
		}

		if c := top(f); c != nil {
			k += " → " + rel(c.File)
		}

		return k
	case "method":
		return strings.ToLower(f.Function)
	default:
		return f.Reason
	}
}

func report(name string, fs []finding) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s — unresolved findings and candidates\n\n", name)
	b.WriteString("Every finding below is still reported. A candidate is a liberal match offered for a person to weigh, never an answer the resolver took: ")
	b.WriteString("**high** is the only component declaring every method the function calls on the receiver (or the only one, named like it); ")
	b.WriteString("**medium** is a sole match on weaker evidence, or the one named like the receiver among several; **low** is one of several. ")
	b.WriteString("*Defined* is how many indexed files declare a method of that name: 0 means it is missing from the workspace, more means the resolver could not connect the call to it.\n\n")

	b.WriteString("| Category | Findings | high | medium | low | none | method defined nowhere |\n|---|---:|---:|---:|---:|---:|---:|\n")

	for _, cat := range categories {
		n, nowhere := 0, 0
		conf := map[string]int{}

		for i := range fs {
			f := &fs[i]
			if f.Category != cat {
				continue
			}

			n++
			conf[confidence(f)]++

			if defined(f) == 0 {
				nowhere++
			}
		}

		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d |\n", cat, n, conf["high"], conf["medium"], conf["low"], conf["none"], nowhere)
	}

	for _, cat := range categories {
		groups := map[string]*group{}

		var order []*group

		for i := range fs {
			f := fs[i]
			if f.Category != cat {
				continue
			}

			k := groupKey(&f)
			g, ok := groups[k]

			if !ok {
				g = &group{key: k}
				groups[k] = g
				order = append(order, g)
			}

			g.findings = append(g.findings, f)
		}

		if len(order) == 0 {
			continue
		}

		sort.SliceStable(order, func(i, j int) bool {
			if len(order[i].findings) != len(order[j].findings) {
				return len(order[i].findings) > len(order[j].findings)
			}

			return order[i].key < order[j].key
		})

		fmt.Fprintf(&b, "\n## %s\n\n", categoryTitle[cat])
		b.WriteString("| Findings | Group | Defined | Confidence | Evidence | Example |\n|---:|---|---:|---|---|---|\n")

		for _, g := range order {
			f := &g.findings[0]
			evidence := ""
			if c := top(f); c != nil {
				evidence = strings.Join(c.Basis, "; ")
				if f.CandidateCount > 1 {
					evidence += fmt.Sprintf(" (%d candidates)", f.CandidateCount)
				}
			}

			def := "?"
			if d := defined(f); d >= 0 {
				def = fmt.Sprint(d)
			}

			fmt.Fprintf(&b, "| %d | `%s` | %s | %s | %s | `%s:%d` %s |\n",
				len(g.findings), cell(g.key), def, confidence(f), cell(evidence), rel(f.File), f.Line+1, cell(f.Reason))
		}

		// Several candidates are listed in full, so a person can choose.
		var several []*group

		for _, g := range order {
			if g.findings[0].CandidateCount > 1 {
				several = append(several, g)
			}
		}

		if len(several) > 0 {
			b.WriteString("\n<details><summary>Groups with several candidates</summary>\n\n")

			for _, g := range several {
				f := &g.findings[0]
				fmt.Fprintf(&b, "- `%s` — %d finding(s), %d candidate(s):\n", cell(g.key), len(g.findings), f.CandidateCount)

				for _, c := range f.Candidates {
					line := ""
					if c.Line > 0 {
						line = fmt.Sprintf(":%d", c.Line+1)
					}

					fmt.Fprintf(&b, "  - %s `%s%s` — %s\n", c.Confidence, rel(c.File), line, strings.Join(c.Basis, "; "))
				}

				if f.CandidateCount > len(f.Candidates) {
					fmt.Fprintf(&b, "  - … %d more\n", f.CandidateCount-len(f.Candidates))
				}
			}

			b.WriteString("\n</details>\n")
		}
	}

	return b.String()
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	if len(s) > 160 {
		s = s[:157] + "…"
	}

	return s
}
