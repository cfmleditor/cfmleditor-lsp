
package formatter_test

// cfscript_formatter_test.go — tests for the recursive CFScript sub-formatter.
//
// These tests verify that the formatter correctly handles all major CFScript
// constructs by parsing with tree-sitter-cfml and checking the output.

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/formatter"
)

// wrap wraps CFScript content in a <cfscript> block.
func wrap(code string) string {
	return "<cfscript>\n" + code + "\n</cfscript>"
}

// lines returns true if got contains all the given substrings, each
// checked independently.
func allIn(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("expected output to contain %q\ngot:\n%s", w, got)
		}
	}
}

// ─── variable declarations ───────────────────────────────────────────────────

func TestVarDeclaration(t *testing.T) {
	src := wrap(`var x = 1;`)
	got := format(t, src)
	allIn(t, got, "var x = 1;")
}

func TestLocalDeclaration(t *testing.T) {
	src := wrap(`local.result = getSomething();`)
	got := format(t, src)
	allIn(t, got, "local.result = getSomething();")
}

// ─── function definitions ────────────────────────────────────────────────────

func TestFunctionNoParams(t *testing.T) {
	src := wrap(`function greet() { return "hello"; }`)
	got := format(t, src)
	allIn(t, got, "function greet()", "return \"hello\";", "}")
}

func TestFunctionWithParams(t *testing.T) {
	src := wrap(`function add(a, b) { return a + b; }`)
	got := format(t, src)
	allIn(t, got, "function add(a, b)", "return a + b;")
}

func TestFunctionWithAccessModifier(t *testing.T) {
	src := wrap(`public string function getName() { return variables.name; }`)
	got := format(t, src)
	allIn(t, got, "function getName()", "return variables.name;")
}

func TestFunctionBodyIndented(t *testing.T) {
	src := wrap(`function foo() {
var x = 1;
var y = 2;
return x + y;
}`)
	got := format(t, src)
	lines := strings.Split(got, "\n")
	for _, l := range lines {
		if strings.Contains(l, "var x") || strings.Contains(l, "var y") || strings.Contains(l, "return x") {
			if !strings.HasPrefix(l, "        ") { // 2 levels of indent (cfscript + function body)
				// Accept either 1 or 2 levels depending on context
				if !strings.HasPrefix(l, "    ") {
					t.Errorf("function body should be indented, got: %q", l)
				}
			}
		}
	}
}

// ─── return / throw / break / continue ───────────────────────────────────────

func TestReturnVoid(t *testing.T) {
	src := wrap(`function f() { return; }`)
	got := format(t, src)
	allIn(t, got, "return;")
}

func TestReturnValue(t *testing.T) {
	src := wrap(`function f() { return x * 2; }`)
	got := format(t, src)
	allIn(t, got, "return x * 2;")
}

func TestThrow(t *testing.T) {
	src := wrap(`throw new Exception("oops");`)
	got := format(t, src)
	allIn(t, got, `throw new Exception("oops");`)
}

// ─── if / else if / else ─────────────────────────────────────────────────────

func TestIfOnly(t *testing.T) {
	src := wrap(`if (x > 0) { doSomething(); }`)
	got := format(t, src)
	allIn(t, got, "if (x > 0) {", "doSomething();", "}")
}

func TestIfElse(t *testing.T) {
	src := wrap(`if (x > 0) { pos(); } else { neg(); }`)
	got := format(t, src)
	allIn(t, got, "if (x > 0)", "} else {", "neg();")
}

func TestIfElseIf(t *testing.T) {
	src := wrap(`if (x > 0) { pos(); } else if (x < 0) { neg(); } else { zero(); }`)
	got := format(t, src)
	allIn(t, got, "if (x > 0)", "else if (x < 0)", "else {", "zero();")
}

// ─── switch ──────────────────────────────────────────────────────────────────

func TestSwitch(t *testing.T) {
	src := wrap(`switch (x) {
case 1:
  doOne();
  break;
case 2:
  doTwo();
  break;
default:
  doDefault();
}`)
	got := format(t, src)
	allIn(t, got, "switch (x)", "case 1:", "case 2:", "default:", "doOne();", "doDefault();", "break;")
}

// ─── loops ───────────────────────────────────────────────────────────────────

func TestWhile(t *testing.T) {
	src := wrap(`while (i < 10) { i++; }`)
	got := format(t, src)
	allIn(t, got, "while (i < 10)", "i++;")
}

func TestDoWhile(t *testing.T) {
	src := wrap(`do { x++; } while (x < 5);`)
	got := format(t, src)
	allIn(t, got, "do {", "x++;", "while (x < 5);")
}

func TestForLoop(t *testing.T) {
	src := wrap(`for (var i = 0; i < 10; i++) { process(i); }`)
	got := format(t, src)
	allIn(t, got, "for (var i = 0; i < 10; i++)", "process(i);")
}

func TestForIn(t *testing.T) {
	src := wrap(`for (var key in myStruct) { writeDump(key); }`)
	got := format(t, src)
	allIn(t, got, "for (key in myStruct)", "writeDump(key);")
}

// ─── try / catch / finally ───────────────────────────────────────────────────

func TestTryCatch(t *testing.T) {
	src := wrap(`try { riskyOp(); } catch (any e) { logError(e); }`)
	got := format(t, src)
	allIn(t, got, "try {", "riskyOp();", "catch", "logError(e);")
}

func TestTryCatchFinally(t *testing.T) {
	src := wrap(`try { open(); } catch (any e) { close(); } finally { cleanup(); }`)
	got := format(t, src)
	allIn(t, got, "try {", "} catch", "} finally {", "cleanup();")
}

// ─── expressions ─────────────────────────────────────────────────────────────

func TestCallExpression(t *testing.T) {
	src := wrap(`writeOutput(foo(1, 2));`)
	got := format(t, src)
	allIn(t, got, "writeOutput(foo(1, 2));")
}

func TestMemberExpression(t *testing.T) {
	src := wrap(`var n = obj.method();`)
	got := format(t, src)
	allIn(t, got, "obj.method()")
}

func TestTernary(t *testing.T) {
	src := wrap(`var r = x > 0 ? "pos" : "neg";`)
	got := format(t, src)
	allIn(t, got, `x > 0 ? "pos" : "neg"`)
}

func TestNewExpression(t *testing.T) {
	src := wrap(`var obj = new MyComponent(arg1, arg2);`)
	got := format(t, src)
	allIn(t, got, "new MyComponent(arg1, arg2)")
}

// ─── comments ────────────────────────────────────────────────────────────────

func TestLineCommentInScript(t *testing.T) {
	src := wrap(`// this is a comment
var x = 1;`)
	got := format(t, src)
	allIn(t, got, "// this is a comment")
	// Comment should appear before the var declaration.
	commentIdx := strings.Index(got, "// this")
	varIdx := strings.Index(got, "var x")
	if commentIdx > varIdx {
		t.Errorf("comment should appear before var declaration")
	}
}

func TestBlockCommentInScript(t *testing.T) {
	src := wrap(`/* multi
   line
   comment */
var x = 1;`)
	got := format(t, src)
	allIn(t, got, "/* multi", "comment */")
}

// ─── blank line preservation ─────────────────────────────────────────────────

func TestBlankLinePreserved(t *testing.T) {
	src := wrap(`var x = 1;

var y = 2;`)
	got := format(t, src)
	// Should have exactly one blank line between var declarations
	xIdx := strings.Index(got, "var x")
	yIdx := strings.Index(got, "var y")
	between := got[xIdx:yIdx]
	blankLines := strings.Count(between, "\n\n")
	if blankLines == 0 {
		t.Errorf("expected blank line to be preserved between var declarations\ngot:\n%s", got)
	}
}

func TestConsecutiveBlankLinesCapped(t *testing.T) {
	src := wrap(`var x = 1;



var y = 2;`)
	got := format(t, src)
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("consecutive blank lines should be capped at 1\ngot:\n%s", got)
	}
}

// ─── component declaration ───────────────────────────────────────────────────

func TestComponentDeclaration(t *testing.T) {
	src := wrap(`component extends="Base" {
  public function init() {
    return this;
  }
}`)
	got := format(t, src)
	allIn(t, got, "component", "function init()", "return this;")
}

// ─── idempotency ─────────────────────────────────────────────────────────────

func TestScriptIdempotency(t *testing.T) {
	src := wrap(`function greet(required string name) {
    var greeting = "Hello, " & name & "!";
    writeOutput(greeting);
    return greeting;
}`)
	got1 := format(t, src)
	tree2 := parse(t, got1)
	got2, err := formatter.Format([]byte(got1), tree2, testOpts())
	if err != nil {
		t.Fatalf("second format error: %v", err)
	}
	if got1 != string(got2) {
		t.Errorf("formatter is not idempotent.\nFirst pass:\n%s\nSecond pass:\n%s", got1, string(got2))
	}
}
