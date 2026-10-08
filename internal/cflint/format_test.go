package cflint

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// capturedRun is CFLint 1.5.17's own JSON report for testdata's fixture, two
// files with errors and warnings, read into a Run the way Lint builds one.
func capturedRun(t *testing.T, minRank int) *Run {
	t.Helper()

	data, err := os.ReadFile("testdata/cflint-1.5.17.json")
	if err != nil {
		t.Fatal(err)
	}

	var report rawReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}

	run := &Run{}
	if err := run.add(&report, minRank); err != nil {
		t.Fatal(err)
	}

	run.sort()

	return run
}

// splitText separates a CFLint text report into its issue blocks and its
// summary lines.
func splitText(text string) (blocks, summary []string) {
	body, tail, _ := strings.Cut(text, "\n\n\nTotal files:")

	for block := range strings.SplitSeq(strings.ReplaceAll(body, "Issue\n", "\n"), "\n\n") {
		if block = strings.Trim(block, "\n"); block != "" && block != "Issue" {
			blocks = append(blocks, block)
		}
	}

	summary = strings.Split(strings.TrimSpace("Total files:"+tail), "\n")

	slices.Sort(blocks)

	return blocks, summary
}

// TestTextMatchesCFLints: the text report is what hook scripts parse — they
// gate on "Total issues:N" — so it is compared against CFLint's own -text
// output for the same files. CFLint prints rules in hash-map order, so the
// issue blocks and the per-rule lines are compared as sets; everything else,
// the totals included, line for line.
func TestTextMatchesCFLints(t *testing.T) {
	want, err := os.ReadFile("testdata/cflint-1.5.17.txt")
	if err != nil {
		t.Fatal(err)
	}

	var got strings.Builder
	if err := WriteText(&got, capturedRun(t, noSeverityFloor)); err != nil {
		t.Fatal(err)
	}

	gotBlocks, gotSummary := splitText(got.String())
	wantBlocks, wantSummary := splitText(string(want))

	if !slices.Equal(gotBlocks, wantBlocks) {
		t.Errorf("issue blocks differ:\ngot:\n%s\nwant:\n%s", strings.Join(gotBlocks, "\n--\n"), strings.Join(wantBlocks, "\n--\n"))
	}

	sortRules := func(lines []string) []string {
		out := slices.Clone(lines)
		// The per-rule lines sit between "Issue counts:N" and the blank line.
		start := slices.IndexFunc(out, func(l string) bool { return strings.HasPrefix(l, "Issue counts:") }) + 1
		end := start + slices.Index(out[start:], "")
		slices.Sort(out[start:end])

		return out
	}

	if !slices.Equal(sortRules(gotSummary), sortRules(wantSummary)) {
		t.Errorf("summary differs:\ngot:\n%s\nwant:\n%s", strings.Join(gotSummary, "\n"), strings.Join(wantSummary, "\n"))
	}

	if !strings.Contains(got.String(), "\nTotal issues:6\n") {
		t.Errorf("no Total issues:6 line:\n%s", got.String())
	}
}

// TestTextReportsZeroIssues: a clean run still prints "Total issues:0", which
// a script reads as a pass, rather than nothing.
func TestTextReportsZeroIssues(t *testing.T) {
	var got strings.Builder
	if err := WriteText(&got, &Run{TotalFiles: 1, TotalLines: 2}); err != nil {
		t.Fatal(err)
	}

	if want := "\n\nTotal files:1\nTotal lines:2\n\nIssue counts:0\n\nTotal issues:0\n"; got.String() != want {
		t.Errorf("got %q, want %q", got.String(), want)
	}
}

// TestJSONKeepsCFLintsIssues: each issue is CFLint's own JSON, so a field this
// package never parses (abbrev, category) still reaches the consumer, and the
// counts cover the run.
func TestJSONKeepsCFLintsIssues(t *testing.T) {
	var got strings.Builder
	if err := WriteJSON(&got, capturedRun(t, noSeverityFloor)); err != nil {
		t.Fatal(err)
	}

	var report struct {
		Issues []map[string]any `json:"issues"`
		Counts struct {
			TotalFiles      int             `json:"totalFiles"`
			CountByCode     []ruleCount     `json:"countByCode"`
			CountBySeverity []severityCount `json:"countBySeverity"`
		} `json:"counts"`
	}

	if err := json.Unmarshal([]byte(got.String()), &report); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, got.String())
	}

	if len(report.Issues) != 6 || report.Counts.TotalFiles != 2 {
		t.Fatalf("issues %d, files %d; want 6 and 2", len(report.Issues), report.Counts.TotalFiles)
	}

	for _, issue := range report.Issues {
		if issue["abbrev"] == nil || issue["category"] == nil {
			t.Errorf("an issue lost a field CFLint gave it: %v", issue)
		}
	}

	if !slices.Contains(report.Counts.CountBySeverity, severityCount{Severity: "ERROR", Count: 2}) {
		t.Errorf("countBySeverity = %v, want ERROR 2", report.Counts.CountBySeverity)
	}
}

// TestSeverityFloorAppliesToEveryFormat: what is reported is what fails the
// run, so the floor removes issues before any writer sees them.
func TestSeverityFloorAppliesToEveryFormat(t *testing.T) {
	rank, _ := MinSeverityRank("ERROR")

	run := capturedRun(t, rank)
	if len(run.Issues) != 2 {
		t.Fatalf("at ERROR and above: %d issues, want the 2 MISSING_VAR", len(run.Issues))
	}

	for i := range run.Issues {
		if run.Issues[i].ID != "MISSING_VAR" {
			t.Errorf("kept %s", run.Issues[i].ID)
		}
	}
}

func TestSARIF(t *testing.T) {
	var got strings.Builder
	if err := WriteSARIF(&got, capturedRun(t, noSeverityFloor), "/fixture"); err != nil {
		t.Fatal(err)
	}

	var log struct {
		Version string `json:"version"`
		Runs    []struct {
			OriginalURIBaseIDs map[string]struct {
				URI string `json:"uri"`
			} `json:"originalUriBaseIds"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				Level     string `json:"level"`
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct {
							URI       string `json:"uri"`
							URIBaseID string `json:"uriBaseId"`
						} `json:"artifactLocation"`
						Region struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
			} `json:"results"`
		} `json:"runs"`
	}

	if err := json.Unmarshal([]byte(got.String()), &log); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	if log.Version != "2.1.0" || len(log.Runs) != 1 || len(log.Runs[0].Results) != 6 {
		t.Fatalf("got version %q, %d runs; want 2.1.0 with 6 results", log.Version, len(log.Runs))
	}

	if base := log.Runs[0].OriginalURIBaseIDs["SRCROOT"].URI; base != "file:///fixture/" {
		t.Errorf("SRCROOT = %q", base)
	}

	levels := map[string]string{}

	for _, r := range log.Runs[0].Results {
		levels[r.RuleID] = r.Level
		loc := r.Locations[0].PhysicalLocation

		if loc.ArtifactLocation.URIBaseID != "SRCROOT" || strings.HasPrefix(loc.ArtifactLocation.URI, "/") || loc.Region.StartLine == 0 {
			t.Errorf("%s: location %+v; want a relative URI against SRCROOT and a line", r.RuleID, loc)
		}
	}

	if levels["MISSING_VAR"] != "error" || levels["AVOID_USING_CFDUMP_TAG"] != "warning" {
		t.Errorf("levels = %v; want MISSING_VAR error, AVOID_USING_CFDUMP_TAG warning", levels)
	}
}
