package formatter

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
)

// The corpus harness counts files the grammar cannot parse; this turns those
// counts into constructs. "Lucee/test/tags/Imap.cfc does not parse" is not
// something a grammar maintainer can act on — `component { function f()
// access:remote { } }` is, and getting from one to the other by hand across
// dozens of files is not worth anyone's afternoon.
//
// It reads the TSV that `make corpus` writes and reduces every parse-refused
// and script-refused entry:
//
//	make corpus CORPUS=/src/Lucee REPORT=/tmp/corpus.tsv
//	make shrink REPORT=/tmp/corpus.tsv
//
// Output is one TSV row per file: grammar, source line range, the reduced
// fragment, and the path.
//
// # Why the reduction is contiguous
//
// The obvious algorithm — delete any lines that still leave the fragment
// failing (delta debugging) — reduces harder but manufactures failures that
// are not in the file. A ColdBox signature reduced that way came out as
// `function href( target ="" struct data = {} )`, apparently a missing comma
// between parameters; the real source is comma-correct, and the shrinker had
// deleted the lines *between* two unrelated parameters and pushed them
// together. Joining non-adjacent lines invents syntax.
//
// So this only ever trims from the ends, and every fragment it prints is a
// contiguous slice of the original. That reduces less aggressively — a file
// whose failure sits in the middle of a long function still reports the whole
// function — but nothing it prints is an artefact of the reduction itself.
//
// Two further invariants keep the trimming honest:
//
//   - the fragment must stay structurally whole (balanced braces, parens and
//     brackets, no cut into a string or block comment). Without this the
//     reduction happily lands on a lone "}", which fails to parse for a reason
//     that has nothing to do with the construct being hunted;
//   - it must keep failing the *same way*. "Still fails" alone is useless here,
//     because nearly any fragment of CFML fails to parse: the reduction
//     converges on whatever scrap is left — a lone "}", a stray "</cfoutput>",
//     a line of backticks. The text of the first ERROR node has to match the
//     one the whole region produced.
//
// Even so, the output is a starting point rather than a verdict: a fragment
// that fails in isolation may be failing for a different reason than the file
// did. Re-read it before filing anything.

// shrinkErrs reports whether src fails to parse under g.
func shrinkErrs(g language.Grammar, src []byte) bool {
	tree := language.Parse(g, src, nil)
	defer tree.Close()

	return tree.RootNode().HasError()
}

// shrinkSignature identifies *which* failure a fragment has, as the text of the
// node that failed. Reducing against "still fails" alone is not enough: almost
// any fragment of CFML fails to parse, so the reduction converges on whatever
// scrap happens to be last — a lone "}", a stray "</cfoutput>", a line of
// backticks — none of which is the construct being hunted. Requiring the
// signature to survive keeps the reduction on the original failure.
//
// The empty string means "parses", and nothing else may return it. tree-sitter
// reports a failure in two shapes — an ERROR node covering text it could not
// fit, and a MISSING node marking a token it expected and inserted — and only
// the first has a `Kind()` of "ERROR". Reading the ERROR node alone therefore
// gave "" for every MISSING-node failure, which made the invariant collapse
// back into "has no ERROR node": a fragment that parses cleanly compared equal
// to one that failed, and the reduction trimmed away the failing line. That is
// the third time this same trap has been walked into while building this tool,
// hence the sentinel below rather than another bare "".
func shrinkSignature(g language.Grammar, src []byte) string {
	tree := language.Parse(g, src, nil)
	defer tree.Close()

	root := tree.RootNode()
	if !root.HasError() {
		return ""
	}

	node := firstFailingNode(root)
	if node == nil {
		// The tree reports an error the walk cannot locate. Distinct from both
		// "parses" and any real signature, so it can never compare equal to
		// another fragment's failure.
		return "\x00unlocated"
	}

	sig := strings.Join(strings.Fields(string(src[node.StartByte():node.EndByte()])), " ")
	if len(sig) > 160 {
		sig = sig[:160]
	}

	// A MISSING node is zero-width, so its text is empty; the kind keeps the
	// signature non-empty and distinguishes one missing token from another.
	return node.Kind() + "\x00" + sig
}

// firstFailingNode returns the first ERROR or MISSING node in document order.
func firstFailingNode(n *sitter.Node) *sitter.Node {
	if n.IsError() || n.IsMissing() {
		return n
	}

	for i := uint(0); i < n.ChildCount(); i++ {
		if e := firstFailingNode(n.Child(i)); e != nil {
			return e
		}
	}

	return nil
}

// shrinkWhole reports whether the fragment is structurally self-contained:
// balanced brackets, with strings and comments neither entered nor left
// halfway. A fragment that fails this is failing to parse because it was cut
// badly, not because of anything the file contains.
func shrinkWhole(src []byte) bool {
	var curly, paren, bracket int

	for i := 0; i < len(src); i++ {
		switch c := src[i]; c {
		case '"', '\'':
			end := indexBytesFrom(src, i+1, string(c))
			if end < 0 {
				return false
			}

			i = end
		case '/':
			if i+1 >= len(src) {
				continue
			}

			switch src[i+1] {
			case '*':
				end := indexBytesFrom(src, i+2, "*/")
				if end < 0 {
					return false
				}

				i = end + 1
			case '/':
				for i < len(src) && src[i] != '\n' {
					i++
				}
			}
		case '{':
			curly++
		case '}':
			curly--
		case '(':
			paren++
		case ')':
			paren--
		case '[':
			bracket++
		case ']':
			bracket--
		}

		if curly < 0 || paren < 0 || bracket < 0 {
			return false
		}
	}

	return curly == 0 && paren == 0 && bracket == 0
}

// shrinkWindow trims lines off each end for as long as the fragment stays
// whole and still fails, and returns the surviving slice with its 1-based line
// range within src.
func shrinkWindow(g language.Grammar, src []byte) (frag []byte, first, last int) {
	lines := strings.Split(string(src), "\n")
	lo, hi := 0, len(lines)
	sig := shrinkSignature(g, src)

	keeps := func(a, b int) bool {
		cand := []byte(strings.Join(lines[a:b], "\n"))

		return shrinkWhole(cand) && shrinkErrs(g, cand) && shrinkSignature(g, cand) == sig
	}

	for chunk := (hi - lo) / 2; chunk >= 1; chunk /= 2 {
		for lo+chunk < hi && keeps(lo+chunk, hi) {
			lo += chunk
		}

		for hi-chunk > lo && keeps(lo, hi-chunk) {
			hi -= chunk
		}
	}

	return []byte(strings.Join(lines[lo:hi], "\n")), lo + 1, hi
}

// escapeFragment renders a fragment as one TSV field without destroying it.
// Collapsing the whitespace was wrong: a fragment containing a cfscript "//"
// comment then swallowed everything after it on the joined line, so the repro
// pasted into a grammar issue no longer reproduced. Newlines and tabs become
// their two-character escapes, which keeps one row per fragment and leaves the
// text recoverable.
func escapeFragment(frag []byte) string {
	r := strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "", "\t", "\\t")

	return r.Replace(string(frag))
}

// failingRegion returns the source the grammar choked on, and which grammar it
// was. For a document the grammar cannot parse that is the file; for one whose
// embedded cfscript or cfquery fails it is the region the formatter handed to
// the sub-parser, which is not otherwise recoverable from the outside.
func failingRegion(verdict string, src []byte) (region []byte, g language.Grammar, name string, ok bool) {
	if verdict == "parse-refused" {
		return src, language.CFML, "cfml", true
	}

	opts := corpusOptions()

	capture := func(sub language.Grammar, subName string) func([]byte) *sitter.Tree {
		return func(b []byte) *sitter.Tree {
			tree := language.Parse(sub, b, nil)

			if region == nil && tree.RootNode().HasError() {
				region = append([]byte{}, b...)
				g, name, ok = sub, subName, true
			}

			return tree
		}
	}
	opts.ParseScript = capture(language.CFScript, "cfscript")
	opts.ParseQuery = capture(language.CFQuery, "cfquery")

	tree := language.Parse(language.CFML, src, nil)
	defer tree.Close()

	if tree.RootNode().HasError() {
		return nil, 0, "", false
	}

	// A panic here is the corpus harness's business, not this tool's.
	defer func() { _ = recover() }()

	_, _ = Format(src, tree, opts)

	return region, g, name, ok
}

func TestShrinkRefusals(t *testing.T) {
	report := os.Getenv("CFML_SHRINK_REPORT")
	if report == "" {
		t.Skip("set CFML_SHRINK_REPORT to a report written by `make corpus REPORT=...`")
	}

	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("reading %s: %v", report, err)
	}

	type row struct {
		grammar  string
		lines    string
		fragment string
		path     string
	}

	var (
		out      []row
		unsolved []string
	)

	for _, line := range strings.Split(string(data), "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) < 2 || (cols[0] != "script-refused" && cols[0] != "parse-refused") {
			continue
		}

		path := cols[1]

		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		region, g, name, ok := failingRegion(cols[0], src)
		if !ok || region == nil {
			unsolved = append(unsolved, path+" (no failing region located)")

			continue
		}

		frag, first, last := shrinkWindow(g, region)
		if !shrinkErrs(g, frag) {
			unsolved = append(unsolved, path+" (reduction did not survive re-checking)")

			continue
		}

		// Line ranges are counted within the region the grammar was handed. For
		// an embedded cfscript or cfquery region that is not the file, so the
		// region is located in the source and the offset added — printing a
		// region-relative number next to a file path invites opening the wrong
		// line. Regions the formatter rewrote before parsing will not be found
		// verbatim; those stay region-relative and say so.
		lineRange := fmt.Sprintf("%d-%d", first, last)

		if off := bytes.Index(src, region); off >= 0 {
			base := bytes.Count(src[:off], []byte("\n"))
			lineRange = fmt.Sprintf("%d-%d", first+base, last+base)
		} else if name != "cfml" {
			lineRange += " (region-relative)"
		}

		out = append(out, row{
			grammar:  name,
			lines:    lineRange,
			fragment: escapeFragment(frag),
			path:     path,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if len(out[i].fragment) != len(out[j].fragment) {
			return len(out[i].fragment) < len(out[j].fragment)
		}

		return out[i].path < out[j].path
	})

	fmt.Println("grammar\tlines\tfragment\tpath")

	for _, r := range out {
		fmt.Printf("%s\t%s\t%s\t%s\n", r.grammar, r.lines, r.fragment, r.path)
	}

	for _, u := range unsolved {
		t.Logf("not reduced: %s", u)
	}

	t.Logf("reduced %d file(s), %d not reduced", len(out), len(unsolved))
}

// TestShrinkSignatureDistinguishesFailures pins the invariant the reduction
// rests on. It has been broken three times: first by reducing against "still
// fails" (every fragment fails, so it converged on a lone "}"), then by
// bracket-balancing alone (which says nothing about tags, so it converged on a
// stray "</cfoutput>"), and then by reading only ERROR nodes — tree-sitter
// reports a missing token as a MISSING node instead, so the signature came back
// "" and compared equal to a fragment that parses cleanly.
func TestShrinkSignatureDistinguishesFailures(t *testing.T) {
	clean := []byte(`component { function f() { return 1; } }`)
	if sig := shrinkSignature(language.CFScript, clean); sig != "" {
		t.Errorf("source that parses should have an empty signature, got %q", sig)
	}

	// Fails via a MISSING node rather than an ERROR node.
	missing := []byte(`component { function f() { if (x) { return 1; } }`)
	if !shrinkErrs(language.CFScript, missing) {
		t.Fatal("fixture no longer fails to parse; pick another")
	}

	missingSig := shrinkSignature(language.CFScript, missing)
	if missingSig == "" {
		t.Error("a fragment that fails must never share the empty signature with one that parses")
	}

	if missingSig == shrinkSignature(language.CFScript, clean) {
		t.Error("a failing fragment and a clean one must not compare equal")
	}

	// Two unrelated failures must not compare equal either, or the reduction
	// is free to wander from one to the other.
	//
	// These have to be malformed CFML, not merely CFML the grammar does not
	// support yet. The originals were `component { function f() access:remote
	// {} }` and `param url.number;` — both valid, both unparsed at the time,
	// and both parsing cleanly as of grammar v0.26.34, which added name:value
	// annotations and untyped param. Two fixtures chosen for failing then
	// returned the empty signature of success and compared equal, and the test
	// failed for a grammar improvement rather than a defect in what it covers.
	a := shrinkSignature(language.CFScript, []byte(`x = = ;`))
	b := shrinkSignature(language.CFScript, []byte(`y = 1 + ;`))

	if a == "" || b == "" || a == b {
		t.Errorf("unrelated failures share a signature: %q vs %q", a, b)
	}
}
