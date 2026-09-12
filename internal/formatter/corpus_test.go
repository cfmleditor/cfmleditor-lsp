package formatter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
	"github.com/cfmleditor/cfmleditor-lsp/internal/path"
)

// The formatter's correctness claim — that it only ever changes whitespace — is not
// something the unit tests can establish on their own: every defect fixed in
// FORMATTER-ISSUES.md was found by running the formatter over thousands of files
// nobody here wrote, and each one looked like an ordinary CFML construct until it
// destroyed code. That audit was done with a scratch harness that was never checked
// in, so the numbers in FORMATTER-ISSUES.md could not be reproduced or moved without
// rebuilding it from the prose first. This is that harness.
//
// It needs a corpus, which is far too large to vendor, so it is opt-in and silently
// skipped when CFML_CORPUS is unset — CI and `make test` are unaffected.
//
//	CFML_CORPUS=/path/to/corpus make corpus
//	CFML_CORPUS=/src/Lucee:/src/coldbox-platform CFML_CORPUS_REPORT=/tmp/r.tsv make corpus
//
// Roots are separated by os.PathListSeparator, and each is reported separately so a
// regression can be attributed to a project rather than to the pile. CFML_CORPUS_REPORT
// names a TSV of every non-clean file (verdict, path, detail) to work through
// individually — reproduce one with:
//
//	cfmleditor-lsp format --allow-non-whitespace <file>

// corpusVerdict is the outcome of formatting one file.
type corpusVerdict int

const (
	// verdictClean formatted, passed the whitespace-only guard, and is a fixed point.
	verdictClean corpusVerdict = iota
	// verdictParseRefused has an ERROR node, so the LSP declines to format it at all.
	// This is grammar work in tree-sitter-cfml, not a formatter defect.
	verdictParseRefused
	// verdictScriptRefused parses as a document, but one of the embedded cfscript or
	// cfquery regions the formatter re-parses with a sub-grammar does not. Also grammar
	// work, and counted apart from the formatter's own defects — a region the sub-parser
	// cannot see is a region the formatter is rendering blind.
	verdictScriptRefused
	// verdictGuardRejected changed non-whitespace content, so Format refused its own
	// output. In the editor this is "format-on-save silently does nothing".
	verdictGuardRejected
	// verdictUnstable formats cleanly but formatting the output again changes it, so
	// format-on-save produces a diff for an unchanged file.
	verdictUnstable
	// verdictMalformed formatted cleanly, passed the guard and is a fixed point,
	// but the output has a structural defect — see malformedShape.
	//
	// This is the one verdict the rest of the harness cannot reach. Everything
	// above it asks whether the formatter destroyed something or failed to
	// settle; nothing asks whether what it settled on is well formed. A defect
	// that is whitespace-only and idempotent is therefore counted clean, which
	// is how a braced `case` body came out as a brace in column one with the
	// switch's closing brace folded onto the block's, undetected across every
	// run in FORMATTER-ISSUES.md.
	verdictMalformed
	// verdictPanic crashed the formatter. In the LSP this takes down the daemon.
	verdictPanic
	// verdictSkipped is not CFML at all — a file whose extension claims it is
	// and whose bytes say otherwise. Counted apart so it cannot be mistaken for
	// a grammar gap.
	verdictSkipped
)

func (v corpusVerdict) String() string {
	switch v {
	case verdictClean:
		return "clean"
	case verdictParseRefused:
		return "parse-refused"
	case verdictScriptRefused:
		return "script-refused"
	case verdictGuardRejected:
		return "guard-rejected"
	case verdictUnstable:
		return "unstable"
	case verdictMalformed:
		return "malformed"
	case verdictPanic:
		return "panic"
	case verdictSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

// corpusOptions mirrors what internal/server/formatting.go passes for a document with
// default configuration. Formatting a corpus under different options would measure a
// formatter no user is running.
func corpusOptions() Options {
	opts := DefaultOptions()
	opts.ParseScript = func(s []byte) *sitter.Tree { return language.Parse(language.CFScript, s, nil) }
	opts.ParseQuery = func(s []byte) *sitter.Tree { return language.Parse(language.CFQuery, s, nil) }
	opts.ParseCFML = func(s []byte) *sitter.Tree { return language.Parse(language.CFML, s, nil) }
	opts.WhitespaceOnly = true

	applyCorpusOptOverrides(&opts)

	return opts
}

// applyCorpusOptOverrides applies CFML_CORPUS_OPTS, a comma-separated list of
// `field=value` pairs set on the Options struct by name:
//
//	CFML_CORPUS_OPTS=braceStyle=next-line,paramBreakThreshold=3 make corpus CORPUS=...
//
// A formatting setting has to be measured over the corpus in every mode it
// offers, and before this existed each sweep meant hand-editing this function
// and remembering to undo it. Three settings were measured that way and the
// patch was slightly different each time.
//
// Field names are matched as written in Options, with a leading lower-case
// letter also accepted so the config spelling (`braceStyle`) works. Anything
// unrecognised panics rather than being ignored: a typo that silently measures
// the default is worse than no sweep at all, because it reads as a mode having
// been checked.
func applyCorpusOptOverrides(opts *Options) {
	spec := os.Getenv("CFML_CORPUS_OPTS")
	if spec == "" {
		return
	}

	v := reflect.ValueOf(opts).Elem()

	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			panic(fmt.Sprintf("CFML_CORPUS_OPTS: %q is not field=value", pair))
		}

		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		field := v.FieldByName(strings.ToUpper(name[:1]) + name[1:])

		if !field.IsValid() || !field.CanSet() {
			panic(fmt.Sprintf("CFML_CORPUS_OPTS: formatter.Options has no settable field %q", name))
		}

		// Only the three kinds Options actually uses for settings. An if
		// chain rather than a switch on reflect.Kind, which the exhaustive
		// linter would want all twenty-six arms of.
		switch {
		case field.Kind() == reflect.String:
			field.SetString(value)
		case field.Kind() == reflect.Bool:
			b, err := strconv.ParseBool(value)
			if err != nil {
				panic(fmt.Sprintf("CFML_CORPUS_OPTS: %s=%q is not a bool", name, value))
			}

			field.SetBool(b)
		case field.Kind() == reflect.Int:
			n, err := strconv.Atoi(value)
			if err != nil {
				panic(fmt.Sprintf("CFML_CORPUS_OPTS: %s=%q is not an int", name, value))
			}

			field.SetInt(int64(n))
		default:
			panic(fmt.Sprintf("CFML_CORPUS_OPTS: %s is a %s, which cannot be set from the environment",
				name, field.Kind()))
		}
	}
}

// classifyCorpusFile formats src the way the LSP would and reports what happened.
// The detail string is the reason, for the report file.
func classifyCorpusFile(src []byte) (verdict corpusVerdict, detail string) {
	// A panic here would otherwise take the whole run down thousands of files in,
	// losing every result gathered so far — and a crash is itself a finding worth
	// recording against the file that caused it.
	defer func() {
		if r := recover(); r != nil {
			verdict = verdictPanic
			detail = fmt.Sprintf("%v", r)
		}
	}()

	opts := corpusOptions()

	// Format re-parses embedded cfscript and cfquery regions with their own grammars,
	// and an ERROR node there is invisible from the outside: the document parses, the
	// formatter runs, and whatever it renders for that region is a guess. Recording it
	// keeps grammar gaps out of the formatter's own defect count.
	var scriptFailed bool

	parseScript, parseQuery := opts.ParseScript, opts.ParseQuery
	noteErrors := func(parse func([]byte) *sitter.Tree) func([]byte) *sitter.Tree {
		return func(b []byte) *sitter.Tree {
			tree := parse(b)
			if tree != nil && tree.RootNode().HasError() {
				scriptFailed = true
			}

			return tree
		}
	}
	opts.ParseScript = noteErrors(parseScript)
	opts.ParseQuery = noteErrors(parseQuery)

	tree := language.Parse(language.CFML, src, nil)
	defer tree.Close()

	// The LSP refuses a document with an ERROR node before calling Format, so a file
	// the grammar cannot parse is not a formatter result either way.
	if tree.RootNode().HasError() {
		return verdictParseRefused, "grammar produced an ERROR node"
	}

	out, err := Format(src, tree, opts)

	if scriptFailed {
		detail := "embedded cfscript/cfquery produced an ERROR node"
		if err != nil {
			detail += "; " + err.Error()
		}

		return verdictScriptRefused, detail
	}

	if err != nil {
		return verdictGuardRejected, err.Error()
	}

	second := language.Parse(language.CFML, out, nil)
	defer second.Close()

	if second.RootNode().HasError() {
		return verdictUnstable, "formatted output no longer parses"
	}

	again, err := Format(out, second, opts)
	if err != nil {
		return verdictUnstable, "second format refused: " + err.Error()
	}

	if !bytes.Equal(out, again) {
		return verdictUnstable, firstDifferingLine(out, again)
	}

	if shape := malformedShape(out, opts); shape != "" {
		return verdictMalformed, shape
	}

	return verdictClean, ""
}

// malformedShape reports a structural defect in output that is otherwise clean:
// whitespace-only, stable, and so invisible to every other check here.
//
// Both rules were chosen by measuring candidates against the corpus and keeping
// only those that accused no healthy file, because a shape check that cries
// wolf is one nobody reads. Four others were tried and dropped, and are
// recorded here so they are not tried again:
//
//	a space anywhere in a line's indent          1458 files — block comment
//	                                             continuations and verbatim markup
//	a lone "{" indented with mixed tabs/spaces      15 files — verbatim regions and
//	                                             <script> bodies in .cfm
//	a lone "{" less indented than the line above     5 files — a struct-literal
//	                                             argument under a multi-line one
//	any run of two or more braces alone on a line   23 files — nested literals
//
// What is left is narrow on purpose. It does not claim to find every malformed
// output, only to stop this class of defect from being counted clean.
func malformedShape(out []byte, opts Options) string {
	lines := strings.Split(string(out), "\n")
	strs := stringSpansOf(out, scriptRegionsOf(out))

	offset := 0

	for i, line := range lines {
		start := offset
		offset += len(line) + 1

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Inside a string literal a brace is text. The formatter cannot
		// re-indent those lines without changing what they say, so it leaves
		// them alone and neither rule below applies. Both rules need this, not
		// just the second: the corpus happens to contain no multi-line string
		// holding a "}}" line, which made the first one look safe without it.
		if strs.contains(start + len(line) - len(strings.TrimLeft(line, " \t"))) {
			continue
		}

		// Every block's closing brace gets a line of its own, so a line that is
		// nothing but closing braces has exactly one. Two means a brace was
		// written where the cursor happened to be rather than at the start of a
		// line, and the next one landed beside it.
		if len(trimmed) > 1 && strings.Trim(trimmed, "}") == "" {
			return fmt.Sprintf("line %d: closing braces share a line: %q", i+1, trimForReport(line))
		}

		// A brace opening a block is written at its construct's indent, so a
		// lone one in column zero means the indent was lost. The first line has
		// nothing above it to be indented under.
		//
		// Only under same-line braces. Allman puts every brace on a line of its
		// own, so a top-level `component` legitimately has one in column zero —
		// under braceStyle "next-line" this rule accused 3,563 of the corpus's
		// 5,624 files, which is how the sweep that CFML_CORPUS_OPTS makes cheap
		// found it. The other rule holds in both styles.
		if !opts.nextLineBraces() && i > 0 && trimmed == "{" && line == trimmed {
			return fmt.Sprintf("line %d: block brace in column one", i+1)
		}
	}

	return ""
}

// firstDifferingLine locates where two formatter passes diverged, so an unstable file
// can be opened at the right place instead of diffed by hand.
func firstDifferingLine(a, b []byte) string {
	al := strings.Split(string(a), "\n")
	bl := strings.Split(string(b), "\n")

	for i := 0; i < len(al) && i < len(bl); i++ {
		if al[i] != bl[i] {
			return fmt.Sprintf("line %d: %q -> %q", i+1, trimForReport(al[i]), trimForReport(bl[i]))
		}
	}

	return fmt.Sprintf("line count %d -> %d", len(al), len(bl))
}

func trimForReport(s string) string {
	const limit = 60
	if len(s) > limit {
		return s[:limit] + "..."
	}

	return s
}

// corpusResult is one file's verdict.
type corpusResult struct {
	root    string
	path    string
	verdict corpusVerdict
	detail  string
}

// corpusTally counts verdicts for one root (or for everything).
type corpusTally [verdictSkipped + 1]int

func (t *corpusTally) total() int {
	n := 0
	for _, c := range t {
		n += c
	}

	return n
}

func TestFormatterCorpus(t *testing.T) {
	spec := os.Getenv("CFML_CORPUS")
	if spec == "" {
		t.Skip("set CFML_CORPUS to a path list of CFML source trees to run the corpus audit")
	}

	// Build the options once before doing any work. An unusable CFML_CORPUS_OPTS
	// panics inside corpusOptions, and left to the workers that is one panic
	// verdict per file — thousands of identical rows, filed against the corpus
	// rather than against the typo that caused them.
	checkCorpusOpts(t)

	roots := filepath.SplitList(spec)

	files, err := collectCorpusFiles(roots)
	if err != nil {
		t.Fatalf("collecting corpus files: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("no .cfc/.cfm/.cfml files found under %v", roots)
	}

	t.Logf("formatting %d files from %d root(s)", len(files), len(roots))

	results := runCorpus(t, files)

	reportCorpus(t, roots, results)
}

// checkCorpusOpts fails the run with one readable message if CFML_CORPUS_OPTS
// cannot be applied.
func checkCorpusOpts(t *testing.T) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%v", r)
		}
	}()

	_ = corpusOptions()
}

// collectCorpusFiles walks each root for CFML source. A root that does not exist is an
// error rather than an empty result, since the usual cause is a typo in CFML_CORPUS and
// a silent "0 problems found" is the worst possible answer to that.
func collectCorpusFiles(roots []string) ([]corpusResult, error) {
	var files []corpusResult

	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			return nil, fmt.Errorf("corpus root %q: %w", root, err)
		}

		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				// .git in particular holds enough loose objects to dominate the walk.
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
					return filepath.SkipDir
				}

				return nil
			}

			switch strings.ToLower(filepath.Ext(path)) {
			case ".cfc", ".cfm", ".cfml":
				files = append(files, corpusResult{root: root, path: path})
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking %q: %w", root, err)
		}
	}

	return files, nil
}

// runCorpus formats every file, in parallel. Each language.Parse builds its own
// tree-sitter parser, so the workers share no state; a corpus of several thousand files
// takes minutes single-threaded and is the whole point of the exercise being repeatable.
func runCorpus(t *testing.T, files []corpusResult) []corpusResult {
	t.Helper()

	workers := runtime.GOMAXPROCS(0)
	if workers > len(files) {
		workers = len(files)
	}

	var (
		wg   sync.WaitGroup
		next = make(chan int)
	)

	for range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for i := range next {
				src, err := os.ReadFile(files[i].path)
				if err != nil {
					files[i].verdict = verdictPanic
					files[i].detail = "read: " + err.Error()

					continue
				}

				// Real trees contain files whose extension says CFML and whose
				// contents are not — Lucee ships three GIFs named *.gif.cfm.
				// Feeding those to the parser produces a refusal that is
				// indistinguishable from a grammar gap in the report.
				if path.IsBinary(src) {
					files[i].verdict = verdictSkipped
					files[i].detail = "binary content"

					continue
				}

				files[i].verdict, files[i].detail = classifyCorpusFile(src)
			}
		}()
	}

	for i := range files {
		next <- i
	}

	close(next)
	wg.Wait()

	return files
}

// reportCorpus prints the same shape of table FORMATTER-ISSUES.md carries, so a run can
// be compared against the recorded baseline directly.
func reportCorpus(t *testing.T, roots []string, results []corpusResult) {
	t.Helper()

	var (
		overall corpusTally
		byRoot  = make(map[string]*corpusTally, len(roots))
		panics  []corpusResult
	)

	for _, r := range results {
		overall[r.verdict]++

		tally, ok := byRoot[r.root]
		if !ok {
			tally = &corpusTally{}
			byRoot[r.root] = tally
		}

		tally[r.verdict]++

		if r.verdict == verdictPanic {
			panics = append(panics, r)
		}
	}

	t.Logf("%-18s %6s %6s %6s %6s %6s %6s %6s %6s %6s", "root", "files", "clean", "parse", "script", "guard", "unstab", "shape", "panic", "skip")

	for _, root := range roots {
		tally, ok := byRoot[root]
		if !ok {
			continue
		}

		t.Logf("%-18s %6d %6d %6d %6d %6d %6d %6d %6d %6d", filepath.Base(root), tally.total(),
			tally[verdictClean], tally[verdictParseRefused], tally[verdictScriptRefused],
			tally[verdictGuardRejected], tally[verdictUnstable], tally[verdictMalformed],
			tally[verdictPanic], tally[verdictSkipped])
	}

	t.Logf("%-18s %6d %6d %6d %6d %6d %6d %6d %6d %6d", "TOTAL", overall.total(),
		overall[verdictClean], overall[verdictParseRefused], overall[verdictScriptRefused],
		overall[verdictGuardRejected], overall[verdictUnstable], overall[verdictMalformed],
		overall[verdictPanic], overall[verdictSkipped])

	compareCorpusBaseline(t, results)

	if path := os.Getenv("CFML_CORPUS_REPORT"); path != "" {
		if err := writeCorpusReport(path, results); err != nil {
			t.Errorf("writing report to %s: %v", path, err)
		} else {
			t.Logf("per-file report written to %s", path)
		}
	}

	// Guard rejections and instability are known, counted, and tracked in
	// FORMATTER-ISSUES.md — failing on them would make the harness useless as a
	// measurement. A panic is different: it is a crash, and in the LSP it takes the
	// daemon down with it.
	for _, p := range panics {
		t.Errorf("panic formatting %s: %s", p.path, p.detail)
	}
}

// writeCorpusReport writes every non-clean file as TSV, sorted so two runs diff cleanly.
// compareCorpusBaseline checks every file's verdict against a report from an
// earlier run, named by CFML_CORPUS_BASELINE, and fails the test when any of
// them moved.
//
// The totals alone cannot do this. A change that breaks one file and fixes
// another leaves every column identical, and that is exactly the shape a new
// setting produces when its default is not quite what the formatter did
// before. Comparing per file was done by hand with `diff` until now, against a
// baseline that had to be generated from a stashed tree and was once silently
// stale.
//
// A report lists only non-clean files, so a file missing from one side is
// clean on that side; both directions are reported.
func compareCorpusBaseline(t *testing.T, results []corpusResult) {
	t.Helper()

	path := os.Getenv("CFML_CORPUS_BASELINE")
	if path == "" {
		return
	}

	base, err := readCorpusReport(path)
	if err != nil {
		t.Fatalf("reading baseline %q: %v", path, err)
	}

	var moved []string

	for _, r := range results {
		was, ok := base[r.path]
		if !ok {
			was = verdictClean.String()
		}

		if now := r.verdict.String(); now != was {
			moved = append(moved, fmt.Sprintf("%s: %s -> %s", r.path, was, now))
		}

		delete(base, r.path)
	}

	// Anything left was in the baseline and not in this run: the file is gone,
	// which is a corpus change rather than a formatter one, but silently
	// dropping it would let a shrinking corpus pass as a clean sweep.
	for p, was := range base {
		moved = append(moved, fmt.Sprintf("%s: %s -> not scanned", p, was))
	}

	if len(moved) == 0 {
		t.Logf("baseline %s: no file changed verdict", filepath.Base(path))

		return
	}

	sort.Strings(moved)

	t.Errorf("%d file(s) changed verdict against %s:", len(moved), path)

	for _, m := range moved {
		t.Errorf("  %s", m)
	}
}

// readCorpusReport reads a report TSV back into path -> verdict.
func readCorpusReport(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}

	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("line %d is not a report row: %q", i+1, line)
		}

		out[parts[1]] = parts[0]
	}

	return out, nil
}

func writeCorpusReport(path string, results []corpusResult) error {
	var rows []string

	for _, r := range results {
		if r.verdict == verdictClean {
			continue
		}

		rows = append(rows, fmt.Sprintf("%s\t%s\t%s", r.verdict, r.path, strings.ReplaceAll(r.detail, "\n", " ")))
	}

	sort.Strings(rows)

	var buf bytes.Buffer

	buf.WriteString("verdict\tpath\tdetail\n")

	for _, row := range rows {
		buf.WriteString(row)
		buf.WriteByte('\n')
	}

	return os.WriteFile(path, buf.Bytes(), 0o600)
}
