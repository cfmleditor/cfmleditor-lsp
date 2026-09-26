package resolve

import (
	"fmt"
	"io"
	"strings"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

// CallsOnLine returns the call sites on a 0-based line, in the order
// [parser.ParseResult.AllCalls] gives them. A non-empty filter keeps only the
// calls whose function name or receiver contains it, ignoring case.
//
// The `explain` CLI and cfmleditor.explainCall both select through here, so the
// same file, line and filter pick the same calls in both.
func CallsOnLine(pr *parser.ParseResult, line int, filter string) []parser.CallSite {
	filter = strings.ToLower(filter)

	var out []parser.CallSite

	for _, call := range pr.AllCalls() {
		if int(call.Line) != line {
			continue
		}

		if filter != "" && !strings.Contains(strings.ToLower(call.FuncName), filter) &&
			!strings.Contains(strings.ToLower(call.Variable), filter) {
			continue
		}

		out = append(out, call)
	}

	return out
}

// CallText renders a call site as the explain report heads it: the receiver,
// any chained hops, then the method.
func CallText(call parser.CallSite) string {
	if len(call.Chain) > 0 {
		return call.Variable + "." + strings.Join(call.Chain, "().") + "()." + call.FuncName
	}

	if call.Variable != "" {
		return call.Variable + "." + call.FuncName
	}

	return call.FuncName
}

// WriteExplanation writes the explain report for calls, which sit on the
// 0-based line of file: per call, a heading, each step [Resolver.ExplainCall]
// recorded, and the verdict. Calls are separated by a blank line.
//
// This is the report text itself, shared by the `explain` CLI and
// cfmleditor.explainCall, so the two cannot drift apart.
func (r *Resolver) WriteExplanation(w io.Writer, file string, line int, calls []parser.CallSite, pr *parser.ParseResult, baseDir string) {
	for i, call := range calls {
		if i > 0 {
			_, _ = fmt.Fprintln(w)
		}

		_, _ = fmt.Fprintf(w, "%s:%d: %s\n", file, line+1, CallText(call))

		reason, steps := r.ExplainCall(call, pr, baseDir)
		for _, s := range steps {
			_, _ = fmt.Fprintf(w, "  - %s\n", s)
		}

		if reason == "" {
			_, _ = fmt.Fprintf(w, "  => resolved\n")
		} else {
			_, _ = fmt.Fprintf(w, "  => unresolved: %s\n", reason)
		}
	}
}
