package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cfmleditor/clif/internal/cflint"
	"github.com/cfmleditor/clif/internal/vfs"
)

const suppressionsUsage = `usage: clif suppressions [--format text|json] [--baseline <file> [--update-baseline]] [-q] <path> [...]

Count the CFLint suppressions in the .cfm/.cfc files under the paths given, by
rule: @CFLintIgnore tags and "// cflint ignore:" comments, each code a comment
names counting once. A code list ends at the first space, as CFLint reads it,
so "@CFLintIgnore A, B" counts (and suppresses) A only.

  --format <f>        text (default) or json
  --baseline <file>   compare with a committed baseline and fail (exit 1) if
                      the total or any rule's count rose: a ratchet, so new
                      suppressions need a deliberate baseline change
  --update-baseline   write the current counts to the --baseline file
  -q, --quiet         with --baseline, print only what rose

Exit status:
  0  counted; with --baseline, nothing rose
  1  with --baseline, the total or a rule's count rose
  2  a path or the baseline does not exist or cannot be read, or a usage error
`

type suppressionsFlags struct {
	format   string
	baseline string
	update   bool
	quiet    bool
	roots    []string
}

func parseSuppressionsFlags(args []string) suppressionsFlags {
	fl := suppressionsFlags{format: "text"}
	rest := args

	for len(rest) > 0 {
		arg := rest[0]
		rest = rest[1:]

		value := func() string {
			if len(rest) == 0 {
				cflintFailf("%s needs a value\n\n%s", arg, suppressionsUsage)
			}

			v := rest[0]
			rest = rest[1:]

			return v
		}

		switch arg {
		case "--format":
			fl.format = value()
		case "--baseline":
			fl.baseline = value()
		case "--update-baseline":
			fl.update = true
		case "-q", "--quiet":
			fl.quiet = true
		default:
			fl.roots = append(fl.roots, positionalOr(arg, suppressionsUsage, exitCFLintError))
		}
	}

	switch {
	case fl.format != "text" && fl.format != "json":
		cflintFailf("--format %q: want text or json\n\n%s", fl.format, suppressionsUsage)
	case fl.update && fl.baseline == "":
		cflintFailf("--update-baseline needs --baseline <file>\n\n%s", suppressionsUsage)
	case len(fl.roots) == 0:
		fmt.Fprint(os.Stderr, suppressionsUsage)
		os.Exit(exitCFLintError)
	}

	return fl
}

// cmdSuppressions counts CFLint suppressions by rule and, against a baseline,
// fails when they rise: a codebase adopting a rule set suppresses what was
// already there, and without a ratchet the count only ever grows.
func cmdSuppressions(args []string) {
	fl := parseSuppressionsFlags(args)

	roots := make([]string, 0, len(fl.roots))

	for _, r := range fl.roots {
		abs, err := filepath.Abs(r)
		if err != nil {
			cflintFailf("%s: %v\n", r, err)
		}

		if _, err := os.Stat(abs); err != nil {
			cflintFailf("%s does not exist\n", r)
		}

		roots = append(roots, abs)
	}

	counts, err := cflint.CountSuppressions(collectCFMLFiles(vfs.OS{}, roots))
	if err != nil {
		cflintFailf("%v\n", err)
	}

	if fl.baseline == "" {
		printSuppressions(&fl, counts)

		return
	}

	if fl.update {
		if err := cflint.WriteSuppressionBaseline(fl.baseline, counts); err != nil {
			cflintFailf("could not write %s: %v\n", fl.baseline, err)
		}

		fmt.Fprintf(os.Stderr, "Wrote %s: %d suppressions over %d rules\n", fl.baseline, counts.Total, len(counts.Rules))

		return
	}

	baseline, err := cflint.ReadSuppressionBaseline(fl.baseline)
	if err != nil {
		cflintFailf("--baseline: %v\n", err)
	}

	if ratchetSuppressions(&fl, baseline, counts) {
		os.Exit(exitCFLintFindings)
	}
}

func printSuppressions(fl *suppressionsFlags, counts *cflint.Suppressions) {
	if fl.format == "json" {
		out, err := json.MarshalIndent(counts, "", "  ")
		if err != nil {
			cflintFailf("%v\n", err)
		}

		fmt.Println(string(out))

		return
	}

	fmt.Printf("%d suppressions in %d files\n", counts.Total, counts.Files)

	for _, r := range counts.ByCount() {
		fmt.Printf("  %-36s %d\n", r.Code, r.Count)
	}
}

// ratchetSuppressions prints how the counts moved against the baseline and
// reports whether anything rose.
func ratchetSuppressions(fl *suppressionsFlags, baseline, counts *cflint.Suppressions) bool {
	rose, fell := cflint.Compare(baseline, counts)

	name := func(c cflint.Change) string {
		if c.Code == "" {
			return "total"
		}

		if c.IsNewRule {
			return c.Code + " (not in the baseline)"
		}

		return c.Code
	}

	for _, c := range rose {
		fmt.Printf("rose  %-48s %d -> %d (+%d)\n", name(c), c.Was, c.Is, c.Is-c.Was)
	}

	if !fl.quiet {
		for _, c := range fell {
			fmt.Printf("fell  %-48s %d -> %d (%d)\n", name(c), c.Was, c.Is, c.Is-c.Was)
		}
	}

	switch {
	case len(rose) > 0:
		fmt.Fprintf(os.Stderr, "Suppressions rose against %s. Fix the findings instead, or accept them with: clif suppressions --baseline %s --update-baseline <path>\n", fl.baseline, fl.baseline)
	case len(fell) > 0 && !fl.quiet:
		fmt.Fprintf(os.Stderr, "Suppressions fell. Lock that in with: clif suppressions --baseline %s --update-baseline <path>\n", fl.baseline)
	case !fl.quiet:
		fmt.Fprintf(os.Stderr, "%d suppressions, as in %s\n", counts.Total, fl.baseline)
	}

	return len(rose) > 0
}
