// Package knownissues reads a file of documented findings — known issues, TODOs,
// a baseline of unresolved calls — and turns each into an editor diagnostic.
//
// The format is one finding per line:
//
//	path:line: message
//	path:line:col: message
//
// A message may start with a tag naming its code, and its severity ahead of
// that, for a finding from a tool with rule IDs:
//
//	path:line:col: [warning CFQUERYPARAM_REQ] <cfquery> should use <cfqueryparam/>
//	path:line: [TODO] move this to the service
//
// A tagged entry's diagnostic carries that code, where an untagged one's is
// known-issue, and its severity overrides the file's.
//
// Blank lines and lines starting with # are ignored. A path is relative to the
// directory holding the file, and never absolute: an absolute path is ignored,
// since the file is committed to a project and read on other machines, where it
// would not name the same file. A "../" path is kept — it reaches a sibling
// project the way a .cfmleditor.json's workspacePaths do, and holds wherever
// the projects are checked out side by side. It is the format
// `cfmleditor-lsp unresolved --known-issues` writes, and it is short enough to
// write by hand.
package knownissues

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.lsp.dev/protocol"
)

// Entry is one finding from a known-issues file.
type Entry struct {
	Path     string // the file's path joined to the known-issues file's directory
	Line     int    // 0-based
	Col      int    // 0-based, -1 when the entry names none
	Message  string
	Code     string // from the message's [tag], empty when it has none
	Severity string // from the message's [severity CODE] tag, empty when it names none
}

// entryLine reads path:line[:col]: message. A drive letter is matched only so
// that an absolute Windows path is recognised, and refused, rather than
// misread as a path named "C".
var entryLine = regexp.MustCompile(`^((?:[A-Za-z]:)?[^:]+):(\d+)(?::(\d+))?:\s*(.+)$`)

// entryTag reads a message's leading [CODE] or [severity CODE].
var entryTag = regexp.MustCompile(`^\[(?:(?i:(error|warning|warn|information|info|hint))\s+)?([A-Za-z0-9_.:-]+)\]\s*(.*)$`)

// Parse reads the entries in content. baseDir anchors relative paths.
func Parse(content, baseDir string) []Entry {
	var out []Entry

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		m := entryLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		n, err := strconv.Atoi(m[2])
		if err != nil || n < 1 {
			continue
		}

		col := -1

		if m[3] != "" {
			if c, err := strconv.Atoi(m[3]); err == nil && c >= 1 {
				col = c - 1
			}
		}

		if isAbsoluteAnywhere(m[1]) {
			continue
		}

		rel := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(m[1], `\`, "/")))

		e := Entry{Path: filepath.Join(baseDir, rel), Line: n - 1, Col: col, Message: strings.TrimSpace(m[4])}
		if t := entryTag.FindStringSubmatch(e.Message); t != nil {
			e.Severity, e.Code, e.Message = strings.ToLower(t[1]), t[2], t[3]
		}

		out = append(out, e)
	}

	return out
}

// isAbsoluteAnywhere reports whether p is absolute on any platform — a drive
// letter, a leading slash or backslash — since a file written on one machine is
// read on others.
func isAbsoluteAnywhere(p string) bool {
	if p == "" {
		return false
	}

	if p[0] == '/' || p[0] == '\\' {
		return true
	}

	return len(p) >= 2 && p[1] == ':' && (p[0]|0x20 >= 'a' && p[0]|0x20 <= 'z')
}

// Severity maps a config severity name to the LSP value. Unknown or empty is
// information, which every editor lists: an LSP hint is not shown in VS Code's
// Problems panel.
func Severity(name string) protocol.DiagnosticSeverity {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "error":
		return protocol.DiagnosticSeverityError
	case "warning", "warn":
		return protocol.DiagnosticSeverityWarning
	case "hint":
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityInformation
	}
}

// SeverityName is the tag word for an LSP severity, the inverse of Severity.
func SeverityName(sev protocol.DiagnosticSeverity) string {
	switch sev {
	case protocol.DiagnosticSeverityError:
		return "error"
	case protocol.DiagnosticSeverityWarning:
		return "warning"
	case protocol.DiagnosticSeverityHint:
		return "hint"
	case protocol.DiagnosticSeverityInformation:
		return "information"
	default:
		return "information"
	}
}

// Row is one entry to write: 1-based line, and column (0 for none).
type Row struct {
	Path     string // relative, with forward slashes
	Line     int
	Col      int
	Severity string
	Code     string
	Message  string
}

// Write writes rows under header, each header line as a # comment, sorted by
// path, line and column. A row's severity and code become its [tag].
func Write(w io.Writer, header []string, rows []Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}

		if a.Line != b.Line {
			return a.Line < b.Line
		}

		if a.Col != b.Col {
			return a.Col < b.Col
		}

		if a.Code != b.Code {
			return a.Code < b.Code
		}

		return a.Message < b.Message
	})

	for _, h := range header {
		_, _ = fmt.Fprintf(w, "# %s\n", h)
	}

	for _, r := range rows {
		pos := strconv.Itoa(r.Line)
		if r.Col > 0 {
			pos += ":" + strconv.Itoa(r.Col)
		}

		tag := strings.TrimSpace(r.Severity + " " + r.Code)
		if r.Code == "" {
			tag = ""
		}

		if tag != "" {
			tag = "[" + tag + "] "
		}

		// A message is one line in this format.
		msg := strings.Join(strings.Fields(r.Message), " ")

		_, _ = fmt.Fprintf(w, "%s:%s: %s%s\n", r.Path, pos, tag, msg)
	}
}

// reanchorWindow is how far from its recorded line an entry's call is looked
// for when the line no longer holds it: an edit above a finding moves it, and
// a baseline is only as fresh as its last regeneration.
const reanchorWindow = 25

// Diagnostic builds the diagnostic for e over the file's lines (nil when the
// file cannot be read, which leaves the entry on its recorded line).
//
// An entry in the unresolved format, `svc.method (reason)`, is underlined at
// the method on its line, and found again within reanchorWindow lines when an
// edit has moved it. Anything else covers the line's text.
func Diagnostic(e Entry, lines []string, severity protocol.DiagnosticSeverity, source string) protocol.Diagnostic {
	line, start, end := e.Line, 0, 0

	if e.Line < len(lines) {
		end = len(lines[e.Line])
		start = len(lines[e.Line]) - len(strings.TrimLeft(lines[e.Line], " \t"))
	}

	if name := calledName(e.Message); name != "" && e.Col < 0 {
		if l, c, ok := findNear(lines, e.Line, name); ok {
			line, start, end = l, c, c+len(name)
		}
	}

	if e.Col >= 0 {
		start = e.Col
		if end < start {
			end = start
		}
	}

	code := "known-issue"
	if e.Code != "" {
		code = e.Code
	}

	if e.Severity != "" {
		severity = Severity(e.Severity)
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: uint32(start)}, //nolint:gosec // line and column come from a file's own lines
			End:   protocol.Position{Line: uint32(line), Character: uint32(end)},   //nolint:gosec // as above
		},
		Severity: severity,
		Source:   protocol.NewOptional(source),
		Code:     protocol.String(code),
		Message:  protocol.String(e.Message),
	}
}

// calledName is the method an unresolved-format message names: the last
// segment of the text before " (", when that text is a call expression.
func calledName(msg string) string {
	head, _, ok := strings.Cut(msg, " (")
	if !ok || head == "" || strings.ContainsAny(head, " \t") {
		return ""
	}

	if dot := strings.LastIndexByte(head, '.'); dot >= 0 {
		head = head[dot+1:]
	}

	head = strings.TrimSuffix(head, "[]")
	for _, c := range head {
		if c != '_' && c != '$' && (c < '0' || c > '9') && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return ""
		}
	}

	return head
}

// findNear finds name as a whole word nearest to line, within reanchorWindow.
func findNear(lines []string, line int, name string) (int, int, bool) {
	for d := 0; d <= reanchorWindow; d++ {
		for _, l := range []int{line - d, line + d} {
			if l < 0 || l >= len(lines) || (d > 0 && l == line) {
				continue
			}

			if c := wordIndexFold(lines[l], name); c >= 0 {
				return l, c, true
			}

			if d == 0 {
				break
			}
		}
	}

	return 0, 0, false
}

func wordIndexFold(s, word string) int {
	ls, lw := strings.ToLower(s), strings.ToLower(word)

	for from := 0; ; {
		i := strings.Index(ls[from:], lw)
		if i < 0 {
			return -1
		}

		i += from
		before := i == 0 || !isWordByte(ls[i-1])
		after := i+len(lw) >= len(ls) || !isWordByte(ls[i+len(lw)])

		if before && after {
			return i
		}

		from = i + 1
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
