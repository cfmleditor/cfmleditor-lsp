package cflint

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/cfmleditor/clif/internal/knownissues"
)

// TestSuppressionsReadCodesAsCFLintDoes: both forms CFLint honours count, a
// comma list counts each code, and a space ends the list — CFLint suppresses
// only A in "@CFLintIgnore A, B", so counting B would claim a suppression that
// does not exist.
func TestSuppressionsReadCodesAsCFLintDoes(t *testing.T) {
	s := &Suppressions{Rules: map[string]int{}}

	n := s.add(`<!--- @CFLintIgnore AVOID_USING_CFDUMP_TAG --->
<!--- @CFLintIgnore CFQUERYPARAM_REQ,AVOID_USING_CFFILE_TAG --->
<!--- @CFLintIgnore MISSING_VAR, GLOBAL_VAR --->
x = 1; // cflint ignore:IMPLICIT_SCOPE,MISSING_VAR
// @CFLintIgnore
`)

	want := map[string]int{
		"AVOID_USING_CFDUMP_TAG": 1,
		"CFQUERYPARAM_REQ":       1,
		"AVOID_USING_CFFILE_TAG": 1,
		"MISSING_VAR":            2,
		"IMPLICIT_SCOPE":         1,
	}

	if n != 6 || s.Total != 6 || len(s.Rules) != len(want) {
		t.Fatalf("n %d total %d rules %v; want 6 and %v", n, s.Total, s.Rules, want)
	}

	for code, c := range want {
		if s.Rules[code] != c {
			t.Errorf("%s = %d, want %d", code, s.Rules[code], c)
		}
	}
}

func TestCompareFindsWhatRose(t *testing.T) {
	baseline := &Suppressions{Total: 5, Rules: map[string]int{"A": 3, "B": 2}}
	now := &Suppressions{Total: 6, Rules: map[string]int{"A": 2, "B": 2, "C": 2}}

	rose, fell := Compare(baseline, now)

	if len(rose) != 2 || rose[0].Code != "" || rose[1].Code != "C" || !rose[1].IsNewRule {
		t.Errorf("rose = %+v; want the total and C, flagged as new", rose)
	}

	if len(fell) != 1 || fell[0].Code != "A" {
		t.Errorf("fell = %+v; want A", fell)
	}

	// A rule falling while another rises by the same amount leaves the total
	// unchanged, and must still fail: the rule that rose is new debt.
	rose, _ = Compare(baseline, &Suppressions{Total: 5, Rules: map[string]int{"A": 2, "B": 3}})
	if len(rose) != 1 || rose[0].Code != "B" {
		t.Errorf("an equal swap: rose = %+v, want B", rose)
	}
}

func TestSuppressionBaselineRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "suppressions.json")
	s := &Suppressions{Total: 3, Files: 2, Rules: map[string]int{"B": 1, "A": 2}}

	if err := WriteSuppressionBaseline(path, s); err != nil {
		t.Fatal(err)
	}

	got, err := ReadSuppressionBaseline(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Total != 3 || got.Files != 2 || got.Rules["A"] != 2 || got.Rules["B"] != 1 {
		t.Errorf("read back %+v", got)
	}
}

func issueAt(file string, line int, id, message string) RunIssue {
	return RunIssue{ID: id, Locations: []Location{{File: file, Line: line, Message: message}}}
}

func entryAt(file string, line int, code, message string) knownissues.Entry {
	return knownissues.Entry{Path: file, Line: line - 1, Code: code, Message: message}
}

func lines(run *Run) []int {
	var out []int
	for i := range run.Issues {
		out = append(out, run.Issues[i].Locations[0].Line)
	}

	return out
}

// TestBaselineMatchesByFileRuleAndMessage: an edit above a finding moves it,
// and must not make it new; a finding not in the baseline is new; an entry
// with nothing left to match is stale.
func TestBaselineMatchesByFileRuleAndMessage(t *testing.T) {
	run := &Run{Issues: []RunIssue{
		issueAt("/p/a.cfm", 14, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"), // was line 4
		issueAt("/p/a.cfm", 20, "MISSING_VAR", "Variable x is not declared"),
		// The same rule and message as a.cfm's baseline entry, in another file.
		issueAt("/p/c.cfm", 4, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"),
	}}

	matched, stale := run.Subtract([]knownissues.Entry{
		entryAt("/p/a.cfm", 4, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"),
		entryAt("/p/b.cfm", 1, "AVOID_USING_CFFILE_TAG", "Avoid cffile"),
	})

	if matched != 1 || !slices.Equal(lines(run), []int{20, 4}) {
		t.Errorf("matched %d, left %v; want 1 matched, and a.cfm's MISSING_VAR and c.cfm's cfdump new", matched, lines(run))
	}

	if len(stale) != 1 || stale[0].Path != "/p/b.cfm" {
		t.Errorf("stale = %+v; want b.cfm's entry", stale)
	}
}

// TestBaselinePairsIdenticalFindingsByLine: a file with one more finding of a
// kind than its baseline lists has one new one, and it is the one added, not
// the old one an edit pushed a little further down.
func TestBaselinePairsIdenticalFindingsByLine(t *testing.T) {
	run := &Run{Issues: []RunIssue{
		issueAt("/p/a.cfm", 5, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"),  // added
		issueAt("/p/a.cfm", 12, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"), // was 10
		issueAt("/p/a.cfm", 40, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"), // was 38
	}}

	matched, stale := run.Subtract([]knownissues.Entry{
		entryAt("/p/a.cfm", 10, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"),
		entryAt("/p/a.cfm", 38, "AVOID_USING_CFDUMP_TAG", "Avoid cfdump"),
	})

	if matched != 2 || len(stale) != 0 || !slices.Equal(lines(run), []int{5}) {
		t.Errorf("matched %d, stale %d, new at %v; want 2, 0 and the one added at 5", matched, len(stale), lines(run))
	}
}
