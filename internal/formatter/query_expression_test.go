package formatter

import (
	"strings"
	"testing"
)

func formatQueryExpr(t *testing.T, src string, tweak func(*Options)) string {
	t.Helper()

	opts := testOpts()
	opts.WhitespaceOnly = true

	if tweak != nil {
		tweak(&opts)
	}

	out, err := Format([]byte(src), parse(t, src), opts)
	if err != nil {
		t.Fatalf("format error: %v", err)
	}

	return string(out)
}

// querySrc is the shape from Lucee's LDEV1576/test.cfm, written — as that file
// is — with no indentation at all.
const querySrc = "<cfscript>\ncomponent {\n\tfunction f() {\n" +
	"local.q = queryExecute(\n" +
	"\" insert into T\n(a,b)\nvalues\n(:x,:y)\",\n" +
	"{\nx: {value: 8}\n},\n" +
	"{\nresult: \"r\"\n}\n" +
	");\n" +
	"\t}\n}\n</cfscript>\n"

// TestQueryExecuteArgumentsAreIndented is the defect. The grammar gives
// `queryExecute` a `query_expression` node of its own rather than the
// call_expression/arguments shape, and with no renderer for it the whole call
// fell to expr's default arm and was emitted verbatim — so a queryExecute call
// was never formatted at all, and where the source had no indentation neither
// did the output.
//
// Whitespace-only and stable, so the guard and the idempotency check both
// passed it; only the corpus shape check saw it.
func TestQueryExecuteArgumentsAreIndented(t *testing.T) {
	t.Parallel()

	out := formatQueryExpr(t, querySrc, nil)

	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, ")") {
			continue
		}

		if line == trimmed {
			t.Errorf("argument left in column one: %q\n%s", line, out)
		}
	}

	assertContains(t, out, "local.q = queryExecute(\n")
}

// TestQueryExecuteSQLIsEmittedVerbatim is the constraint on the fix. The query
// is a string literal, so its interior — the line breaks of a multi-line query
// included — is content, not layout. Re-indenting it would change what the
// query says, and the whitespace-only guard would rightly refuse the file.
func TestQueryExecuteSQLIsEmittedVerbatim(t *testing.T) {
	t.Parallel()

	out := formatQueryExpr(t, querySrc, nil)

	assertContains(t, out, "\" insert into T\n(a,b)\nvalues\n(:x,:y)\",")
}

// TestQueryExecuteShortCallStaysInline is the boundary: a query that fits has
// nothing to gain from being broken across lines.
func TestQueryExecuteShortCallStaysInline(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"<cfscript>\nq = queryExecute(\"select 1\");\n</cfscript>\n",
		"<cfscript>\nq = queryExecute(\"select 1\", {});\n</cfscript>\n",
	} {
		out := formatQueryExpr(t, src, nil)
		if strings.Contains(out, "queryExecute(\n") {
			t.Errorf("short call was broken across lines:\n%s", out)
		}
	}
}

// TestQueryExecuteHonoursFormattingSettings is what the verbatim fallback was
// costing: none of the formatter's settings reached inside a queryExecute call.
func TestQueryExecuteHonoursFormattingSettings(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\nq = queryExecute(\"s\", {a: 1});\n</cfscript>\n"

	assertContains(t, formatQueryExpr(t, src, func(o *Options) { o.ParenSpacing = "pad" }), "queryExecute( ")
	assertContains(t, formatQueryExpr(t, src, func(o *Options) { o.ParenSpacing = "tight" }), "queryExecute(\"s\"")
}

// TestQueryExecuteWithACommentIsLeftAlone covers the one shape the renderer
// declines. A comment among the arguments has nowhere to go in a rendered list
// and a `//` would swallow whatever followed it on the line, so the call is
// reproduced as written — which is what happened to every queryExecute before
// this renderer existed.
func TestQueryExecuteWithACommentIsLeftAlone(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"<cfscript>\nq = queryExecute(\"s\", /* why */ {a:1});\n</cfscript>\n",
		"<cfscript>\nq = queryExecute(\"s\",\n// why\n{a:1});\n</cfscript>\n",
	} {
		out := formatQueryExpr(t, src, nil)

		// Untouched means the struct keeps its source spacing rather than
		// gaining the padding the formatter would otherwise give it.
		assertContains(t, out, "{a:1}")
		assertReparses(t, out)
	}
}

// TestQueryExecuteFormsThatAreOrdinaryCalls is the other boundary. The grammar
// only produces query_expression for the positional form with a double-quoted
// first argument; a named argument or a single-quoted query is an ordinary
// call_expression and was already being formatted. Neither should regress.
func TestQueryExecuteFormsThatAreOrdinaryCalls(t *testing.T) {
	t.Parallel()

	assertContains(t,
		formatQueryExpr(t, "<cfscript>\nq = queryExecute(sql = \"select 1\", params = {a:1});\n</cfscript>\n", nil),
		"{ a: 1 }")
	assertContains(t,
		formatQueryExpr(t, "<cfscript>\nq = queryExecute('select 1', {a:1});\n</cfscript>\n", nil),
		"{ a: 1 }")
}

// TestQueryExecuteNestedSplitIndentsCorrectly is why the arguments are
// rendered a second time at the deeper level rather than reused from the
// inline attempt. An argument that splits on its own — a nested call with more
// arguments than fit on a line — indents its parts against f.level, and
// rendered at the outer level it lands one level short, with its closing paren
// shallower than the line it belongs to.
func TestQueryExecuteNestedSplitIndentsCorrectly(t *testing.T) {
	t.Parallel()

	src := "<cfscript>\ncomponent {\nfunction f() {\nq = queryExecute(\n\"select *\nfrom t\",\n" +
		"buildParams(aaa, bbb, ccc, ddd, eee)\n);\n}\n}\n</cfscript>\n"

	out := formatQueryExpr(t, src, nil)

	assertContains(t, out, "                buildParams(\n                    aaa,")
	assertContains(t, out, "\n                )\n            );")
}

// TestQueryExecuteIsIdempotent — the rendered form becomes the next pass's
// source, and the multi-line SQL literal inside it is the part most likely to
// be re-read as layout.
func TestQueryExecuteIsIdempotent(t *testing.T) {
	t.Parallel()

	for _, src := range []string{querySrc,
		"<cfscript>\nq = queryExecute(\"select 1\");\n</cfscript>\n",
		"<cfscript>\nq = queryExecute(\"s\",\n// why\n{a:1});\n</cfscript>\n",
	} {
		once := formatQueryExpr(t, src, nil)
		if twice := formatQueryExpr(t, once, nil); twice != once {
			t.Errorf("not idempotent:\n--- pass 1\n%s\n--- pass 2\n%s", once, twice)
		}
	}
}
