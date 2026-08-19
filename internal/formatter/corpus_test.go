package formatter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/cfmleditor/cfmleditor-lsp/internal/language"
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
	// verdictPanic crashed the formatter. In the LSP this takes down the daemon.
	verdictPanic
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
	case verdictPanic:
		return "panic"
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

	return opts
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

	return verdictClean, ""
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
type corpusTally [verdictPanic + 1]int

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

	t.Logf("%-18s %6s %6s %6s %6s %6s %6s %6s", "root", "files", "clean", "parse", "script", "guard", "unstab", "panic")

	for _, root := range roots {
		tally, ok := byRoot[root]
		if !ok {
			continue
		}

		t.Logf("%-18s %6d %6d %6d %6d %6d %6d %6d", filepath.Base(root), tally.total(),
			tally[verdictClean], tally[verdictParseRefused], tally[verdictScriptRefused],
			tally[verdictGuardRejected], tally[verdictUnstable], tally[verdictPanic])
	}

	t.Logf("%-18s %6d %6d %6d %6d %6d %6d %6d", "TOTAL", overall.total(),
		overall[verdictClean], overall[verdictParseRefused], overall[verdictScriptRefused],
		overall[verdictGuardRejected], overall[verdictUnstable], overall[verdictPanic])

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
