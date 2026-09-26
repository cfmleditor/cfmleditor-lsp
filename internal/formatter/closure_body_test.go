package formatter

import (
	"strings"
	"testing"
	"time"
)

// formatStable formats src with the guard on, checks the result still parses,
// and checks a second pass leaves it alone.
func formatStable(t *testing.T, src string) string {
	t.Helper()

	out := formatGuarded(t, src)
	assertReparses(t, out)

	if again := formatGuarded(t, out); again != out {
		t.Errorf("not idempotent:\nfirst:\n%s\nsecond:\n%s", out, again)
	}

	return out
}

// TestClosureBodyFormatted covers the body of a function expression or arrow
// function written across lines. It was copied verbatim, so a callback kept
// whatever indentation it was typed with, and a closure argument long enough to
// push its call past the line width broke the call onto one argument per line:
// `describe(`, `"x",`, `function() {`, each on its own line, around a body left
// as written. 2,929 corpus components hold one; 2,250 of them are TestBox specs.
func TestClosureBodyFormatted(t *testing.T) {
	src := wrap(`describe("suite", function(){
it("works", function(){
expect(1).toBe(1);
      expect(2).toBe(2);
});
});`)

	out := formatStable(t, src)
	assertContains(t, out, "describe(\"suite\", function() {\n\n"+
		"        it(\"works\", function() {\n\n"+
		"            expect(1).toBe(1);\n"+
		"            expect(2).toBe(2);\n\n"+
		"        });\n\n"+
		"    });")
}

// TestClosureHugsItsCall pins the argument-list rule: a call whose multi-line
// arguments are all closures stays on the call's line when the lines the call
// owns fit, whether the closure is passed by position or by name.
func TestClosureHugsItsCall(t *testing.T) {
	long := strings.Repeat("x", 40)

	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "positional",
			src:  "arrayEach(arr, function(el){\n" + long + "(el);\n" + long + "(el);\n" + long + "(el);\n});",
			want: "arrayEach(arr, function(el) {\n",
		},
		{
			name: "named argument",
			src:  "it(title = \"t\", body = function(){\n" + long + "();\n" + long + "();\n" + long + "();\n});",
			want: "it(title = \"t\", body = function() {\n",
		},
		{
			name: "arrow function",
			src:  "arr.sort((a, b) => {\n" + long + "(a);\n" + long + "(b);\n" + long + "(a);\n});",
			want: "arr.sort((a, b) => {\n",
		},
		{
			name: "closure before another argument",
			src:  "reduce(arr, function(acc, x){\n" + long + "(x);\n" + long + "(x);\n" + long + "(x);\n}, 0);",
			want: "    }, 0);",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, formatStable(t, wrap(tt.src)), tt.want)
		})
	}
}

// TestClosureInChainIndent checks that a closure later in a chain lines up with
// the one before it. The call renderer indented a call's arguments a level
// deeper whenever its callee held a newline, meaning a chain broken before the
// accessor; a closure earlier in the chain supplies newlines without breaking
// it, and `.catch`'s callback came out a level deeper than `.then`'s.
func TestClosureInChainIndent(t *testing.T) {
	src := wrap("p.then(function(r){\nlog(r);\n}).catch(function(e){\nlog(e);\n});")

	out := formatStable(t, src)
	assertContains(t, out, "    p.then(function(r) {\n\n        log(r);\n\n    }).catch(function(e) {\n\n        log(e);\n\n    });")
}

// TestClosureKeptAsWritten pins the two bodies closureBody leaves alone: one
// written on one line, and an empty one.
func TestClosureKeptAsWritten(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"one line", "x = arr.map(function(x){ return x * 2; });", "x = arr.map(function(x) { return x * 2; });"},
		{"empty across lines", "noop = function(){\n};", "noop = function() {};"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, formatStable(t, wrap(tt.src)), tt.want)
		})
	}
}

// TestLiteralHoldingClosureBreaks checks a struct whose entry is a multi-line
// closure is written one entry per line. Kept inline because it was short, it
// came out as `{ a = function() {`, the body, then `}, b = 1 }`.
func TestLiteralHoldingClosureBreaks(t *testing.T) {
	out := formatStable(t, "<cfset l = {a=function(x){\nreturn x;\n}, b=1}>\n")

	assertContains(t, out, "<cfset l = {\n    a = function(x) {\n\n        return x;\n\n    },\n    b = 1\n} />")
}

// TestWordOperatorLineCommentKept covers the comment defect closure formatting
// surfaced in WireBox's Binder.cfc: a `//` comment on its own line between `OR`
// and the next clause. The word operator is lifted from the source with the gap
// around it, trimmed, and the right operand joined on after it — so the
// comment swallowed the clause. The statement renderer had the defect all
// along; the file's only instance was in a closure body, copied verbatim until
// now.
func TestWordOperatorLineCommentKept(t *testing.T) {
	src := wrap("return (\n\t( a )\n\tOR\n\t// the other case\n\t( b )\n);")

	out := formatStable(t, src)
	assertNotContains(t, out, "// the other case (")
}

// TestNestedClosuresLinear guards the closureBody cache. A literal or argument
// list going multi-line renders its entries twice, so each level of callback
// nested inside one doubled the work: fourteen levels of `it(…, body =
// function() { var c = { h = function() {` took seconds uncached and never
// finished at twenty.
func TestNestedClosuresLinear(t *testing.T) {
	body := "x();\n"
	for i := range 20 {
		body = "it(title = \"t" + strings.Repeat("i", i) + "\", skip = false, data = {}, body = function() {\n" +
			"var c = { h = function() {\n" + body + "} };\n});\n"
	}

	start := time.Now()

	formatGuarded(t, wrap(body))

	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("20 nested closures took %v", d)
	}
}
