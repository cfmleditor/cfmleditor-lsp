package cflint

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

// levels is com.cflint.Levels in its declared order, which is the order
// CFLint's text report prints its per-severity totals in.
var levels = []string{"FATAL", "CRITICAL", "ERROR", "WARNING", "CAUTION", "INFO", "COSMETIC", "UNKNOWN"}

// ruleCount is one rule's count, in the order rules first appear in the run.
type ruleCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type severityCount struct {
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

// countByRule counts the run's issues by rule, in first-seen order.
func (run *Run) countByRule() []ruleCount {
	var out []ruleCount

	index := map[string]int{}

	for i := range run.Issues {
		id := run.Issues[i].ID

		if j, ok := index[id]; ok {
			out[j].Count++

			continue
		}

		index[id] = len(out)
		out = append(out, ruleCount{Code: id, Count: 1})
	}

	return out
}

// countBySeverity counts the run's issues by CFLint level, in levels' order.
func (run *Run) countBySeverity() []severityCount {
	counts := map[string]int{}
	for i := range run.Issues {
		counts[strings.ToUpper(run.Issues[i].Severity)]++
	}

	out := []severityCount{}

	for _, l := range levels {
		if counts[l] > 0 {
			out = append(out, severityCount{Severity: l, Count: counts[l]})
		}
	}

	return out
}

// WriteText writes the run as CFLint's -text report does
// (com.cflint.TextOutput), field for field: the issues grouped by rule, then
// the totals. The totals are the contract — scripts gate on its
// "Total issues:N" line — so they are reproduced exactly.
func WriteText(w io.Writer, run *Run) error {
	var b strings.Builder

	groups := map[string][]*RunIssue{}

	var order []string

	for i := range run.Issues {
		issue := &run.Issues[i]
		if _, ok := groups[issue.ID]; !ok {
			order = append(order, issue.ID)
		}

		groups[issue.ID] = append(groups[issue.ID], issue)
	}

	for _, id := range order {
		group := groups[id]

		b.WriteString("Issue")

		for _, issue := range group {
			for j := range issue.Locations {
				loc := &issue.Locations[j]

				fmt.Fprintf(&b, "\nSeverity:%s", group[0].Severity)
				fmt.Fprintf(&b, "\nMessage code:%s", id)
				fmt.Fprintf(&b, "\n\tFile:%s", loc.File)
				fmt.Fprintf(&b, "\n\tColumn:%d", max(loc.Column, 0))
				fmt.Fprintf(&b, "\n\tLine:%d", loc.Line)
				fmt.Fprintf(&b, "\n\t\tMessage:%s", loc.Message)
				fmt.Fprintf(&b, "\n\t\tVariable:'%s' in function: %s", loc.Variable, loc.Function)
				fmt.Fprintf(&b, "\n\t\tExpression:%s", loc.Expression)
				b.WriteString("\n")
			}
		}
	}

	byRule := run.countByRule()

	fmt.Fprintf(&b, "\n\nTotal files:%d", run.TotalFiles)
	fmt.Fprintf(&b, "\nTotal lines:%d", run.TotalLines)
	fmt.Fprintf(&b, "\n\nIssue counts:%d", len(byRule))

	for _, c := range byRule {
		fmt.Fprintf(&b, "\n%s:%d", c.Code, c.Count)
	}

	fmt.Fprintf(&b, "\n\nTotal issues:%d", len(run.Issues))

	for _, c := range run.countBySeverity() {
		fmt.Fprintf(&b, "\nTotal %ss:%d", strings.ToLower(c.Severity), c.Count)
	}

	b.WriteString("\n")

	_, err := io.WriteString(w, b.String())

	return err
}

// WriteJSON writes the run in CFLint's -json schema: "issues" is each issue's
// own JSON, as CFLint produced it, and "counts" is recomputed over the whole
// run, where CFLint's are per process.
func WriteJSON(w io.Writer, run *Run) error {
	issues := make([]json.RawMessage, 0, len(run.Issues))
	for i := range run.Issues {
		issues = append(issues, run.Issues[i].Raw)
	}

	report := struct {
		Version   string            `json:"version"`
		Timestamp int64             `json:"timestamp"`
		Issues    []json.RawMessage `json:"issues"`
		Counts    struct {
			TotalFiles      int             `json:"totalFiles"`
			TotalLines      int             `json:"totalLines"`
			CountByCode     []ruleCount     `json:"countByCode"`
			CountBySeverity []severityCount `json:"countBySeverity"`
		} `json:"counts"`
	}{Version: run.Version, Timestamp: time.Now().Unix(), Issues: issues}

	report.Counts.TotalFiles = run.TotalFiles
	report.Counts.TotalLines = run.TotalLines
	report.Counts.CountByCode = run.countByRule()
	report.Counts.CountBySeverity = run.countBySeverity()

	if report.Counts.CountByCode == nil {
		report.Counts.CountByCode = []ruleCount{}
	}

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	_, err = w.Write(append(out, '\n'))

	return err
}

// sarifLevel maps a CFLint level onto SARIF's three.
func sarifLevel(severity string) string {
	switch strings.ToUpper(severity) {
	case "FATAL", "CRITICAL", "ERROR":
		return "error"
	case "INFO", "COSMETIC":
		return "note"
	default:
		return "warning"
	}
}

// WriteSARIF writes the run as a SARIF 2.1.0 log, the format CI servers and
// code-quality tools import. Each rule id is CFLint's code, so suppressions,
// baselines and quality profiles keyed on CFLint codes line up. Paths under
// root are written relative to it, against the SRCROOT base, so the log reads
// the same on any checkout of the project.
func WriteSARIF(w io.Writer, run *Run, root string) error {
	type message struct {
		Text string `json:"text"`
	}

	type region struct {
		StartLine   int `json:"startLine,omitempty"`
		StartColumn int `json:"startColumn,omitempty"`
	}

	type artifactLocation struct {
		URI       string `json:"uri"`
		URIBaseID string `json:"uriBaseId,omitempty"`
	}

	type physicalLocation struct {
		ArtifactLocation artifactLocation `json:"artifactLocation"`
		Region           region           `json:"region"`
	}

	type location struct {
		PhysicalLocation physicalLocation `json:"physicalLocation"`
	}

	type result struct {
		RuleID    string     `json:"ruleId"`
		Level     string     `json:"level"`
		Message   message    `json:"message"`
		Locations []location `json:"locations"`
	}

	type rule struct {
		ID string `json:"id"`
	}

	results := make([]result, 0, len(run.Issues))
	ruleIDs := []rule{}
	seen := map[string]bool{}

	for i := range run.Issues {
		issue := &run.Issues[i]

		if !seen[issue.ID] {
			seen[issue.ID] = true
			ruleIDs = append(ruleIDs, rule{ID: issue.ID})
		}

		for j := range issue.Locations {
			loc := &issue.Locations[j]
			al := artifactLocation{URI: filepath.ToSlash(loc.File)}

			if rel, err := filepath.Rel(root, loc.File); err == nil && root != "" && filepath.IsAbs(loc.File) && !strings.HasPrefix(rel, "..") {
				al = artifactLocation{URI: (&url.URL{Path: filepath.ToSlash(rel)}).EscapedPath(), URIBaseID: "SRCROOT"}
			}

			results = append(results, result{
				RuleID:  issue.ID,
				Level:   sarifLevel(issue.Severity),
				Message: message{Text: loc.Message},
				Locations: []location{{PhysicalLocation: physicalLocation{
					ArtifactLocation: al,
					Region:           region{StartLine: loc.Line, StartColumn: max(loc.Column, 0)},
				}}},
			})
		}
	}

	type driver struct {
		Name           string `json:"name"`
		Version        string `json:"version,omitempty"`
		InformationURI string `json:"informationUri"`
		Rules          []rule `json:"rules"`
	}

	type sarifRun struct {
		Tool struct {
			Driver driver `json:"driver"`
		} `json:"tool"`
		OriginalURIBaseIDs map[string]artifactLocation `json:"originalUriBaseIds,omitempty"`
		Results            []result                    `json:"results"`
	}

	r := sarifRun{Results: results}
	r.Tool.Driver = driver{
		Name:           "CFLint",
		Version:        run.Version,
		InformationURI: "https://github.com/cfmleditor/CFLint",
		Rules:          ruleIDs,
	}

	if root != "" {
		// A Windows root (C:\src) is file:///C:/src/: the path takes a leading
		// slash the drive letter does not have.
		p := filepath.ToSlash(root)
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}

		base := (&url.URL{Scheme: "file", Path: strings.TrimSuffix(p, "/") + "/"}).String()
		r.OriginalURIBaseIDs = map[string]artifactLocation{"SRCROOT": {URI: base}}
	}

	log := struct {
		Schema  string     `json:"$schema"`
		Version string     `json:"version"`
		Runs    []sarifRun `json:"runs"`
	}{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs:    []sarifRun{r},
	}

	out, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}

	_, err = w.Write(append(out, '\n'))

	return err
}
