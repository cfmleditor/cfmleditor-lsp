package formatter

// cfscript_formatter_test.go — tests for the recursive CFScript sub-formatter.
//
// These tests verify that the formatter correctly handles all major CFScript
// constructs by parsing with tree-sitter-cfml and checking the output.

import (
	"strings"
	"testing"
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
	allIn(t, got, "function add(\n", "a,\n", "b\n", "return a + b;")
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

	lines := strings.SplitSeq(got, "\n")
	for l := range lines {
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
	allIn(t, got, "if ( x > 0 ) {", "doSomething();", "}")
}

func TestIfElse(t *testing.T) {
	src := wrap(`if (x > 0) { pos(); } else { neg(); }`)
	got := format(t, src)
	allIn(t, got, "if ( x > 0 )", "} else {", "neg();")
}

func TestIfElseIf(t *testing.T) {
	src := wrap(`if (x > 0) { pos(); } else if (x < 0) { neg(); } else { zero(); }`)
	got := format(t, src)
	allIn(t, got, "if ( x > 0 )", "else if ( x < 0 )", "else {", "zero();")
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
	allIn(t, got, "switch ( x )", "case 1:", "case 2:", "default:", "doOne();", "doDefault();", "break;")
}

// ─── loops ───────────────────────────────────────────────────────────────────

func TestWhile(t *testing.T) {
	src := wrap(`while (i < 10) { i++; }`)
	got := format(t, src)
	allIn(t, got, "while ( i < 10 )", "i++;")
}

func TestDoWhile(t *testing.T) {
	src := wrap(`do { x++; } while (x < 5);`)
	got := format(t, src)
	allIn(t, got, "do {", "x++;", "while ( x < 5 );")
}

func TestForLoop(t *testing.T) {
	src := wrap(`for (var i = 0; i < 10; i++) { process(i); }`)
	got := format(t, src)
	allIn(t, got, "for ( var i = 0; i < 10; i++ )", "process(i);")
}

func TestForIn(t *testing.T) {
	src := wrap(`for (var key in myStruct) { writeDump(key); }`)
	got := format(t, src)
	allIn(t, got, "for ( var key in myStruct )", "writeDump(key);")
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

// The grammar wraps "?." in a named optional_chain node rather than an
// anonymous token, the same way it wraps "::" in a named static_chain node.
// memberOperator's fallback loop skips every named child, so without an
// explicit optional_chain check it silently dropped the "?" and turned a
// null-safe chain into one that throws on a nil receiver.
func TestOptionalChainExpression(t *testing.T) {
	src := wrap(`var z = a?.b?.c?.d;`)
	got := format(t, src)
	allIn(t, got, "a?.b?.c?.d")
}

func TestOptionalChainWithCalls(t *testing.T) {
	src := wrap(`var z = a()?.b?.c()?.d();`)
	got := format(t, src)
	allIn(t, got, "a()?.b?.c()?.d()")
}

func TestStaticChainExpression(t *testing.T) {
	src := wrap(`var w = Widget::getData();`)
	got := format(t, src)
	allIn(t, got, "Widget::getData()")
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

	got2, err := Format([]byte(got1), tree2, testOpts())
	if err != nil {
		t.Fatalf("second format error: %v", err)
	}

	if got1 != string(got2) {
		t.Errorf("formatter is not idempotent.\nFirst pass:\n%s\nSecond pass:\n%s", got1, string(got2))
	}
}

// ─── multi-line arguments ────────────────────────────────────────────────────

func TestMultiLineArgsIndented(t *testing.T) {
	src := wrap(`myFunction(arg1, arg2, arg3, arg4);`)
	got := format(t, src)
	// >3 arguments should be broken onto separate lines, each indented
	allIn(t, got, "\n        arg1,\n        arg2,\n        arg3,\n        arg4\n")
}

func TestSingleLineArgsUnchanged(t *testing.T) {
	src := wrap(`myFunction(arg1, arg2, arg3);`)
	got := format(t, src)
	allIn(t, got, "myFunction(arg1, arg2, arg3);")
}

// ─── tree-sitter-cfml v0.26.32/33 constructs ─────────────────────────────────
//
// Each of these parsed as an ERROR before the grammar bump, so the formatter
// had never seen the nodes and fell through to a default that quietly rewrote
// them. All three are semantic, not cosmetic: `->` and `=>` differ in scope
// capture, and the `new` type prefix picks the type system.

func TestThinArrowLambdaPreserved(t *testing.T) {
	src := wrap(`var a = t -> t.b();`)
	got := format(t, src)
	allIn(t, got, "t -> t.b()")
	assertNotContains(t, got, "=>")
}

func TestFatArrowClosureStillPreserved(t *testing.T) {
	src := wrap(`var a = t => t.b();`)
	got := format(t, src)
	allIn(t, got, "t => t.b()")
	assertNotContains(t, got, "->")
}

func TestThinArrowLambdaWithBlockBody(t *testing.T) {
	src := wrap(`var c = t -> { return t.b(); };`)
	got := format(t, src)
	allIn(t, got, "t -> {")
}

func TestNewJavaTypePrefixPreserved(t *testing.T) {
	src := wrap(`var d = new java:java.io.File(path);`)
	got := format(t, src)
	allIn(t, got, "new java:java.io.File(path)")
}

func TestNewCfmlTypePrefixPreserved(t *testing.T) {
	src := wrap(`var e = new cfml:models.Base();`)
	got := format(t, src)
	allIn(t, got, "new cfml:models.Base()")
}

func TestNewWithoutTypePrefixUnchanged(t *testing.T) {
	src := wrap(`var a = new java.util.Properties();`)
	got := format(t, src)
	allIn(t, got, "new java.util.Properties()")
	assertNotContains(t, got, "java:")
}

func TestArrayTypeInParameterPosition(t *testing.T) {
	src := wrap(`function f( string[] v, numeric[][] w ) { return v; }`)
	got := format(t, src)
	allIn(t, got, "string[] v", "numeric[][] w")
	assertNotContains(t, got, "string []")
}

func TestArrayReturnSuffixOnDeclaration(t *testing.T) {
	src := wrap(`User[] function getUsers() { return []; }`)
	got := format(t, src)
	allIn(t, got, "User[] function getUsers()")
	assertNotContains(t, got, "User []")
}

// The LSP runs with whitespaceOnly on by default, so before these fixes it did
// not merely reformat these files — it refused to format them at all.
func TestNewGrammarConstructsPassWhitespaceOnlyGuard(t *testing.T) {
	src := wrap(`var a = t -> t.b();
var d = new java:java.io.File(path);
var u = getUsers();
function f( string[] v ) { return v; }
User[] function getUsers() { return []; }`)

	opts := testOpts()
	opts.WhitespaceOnly = true

	if _, err := Format([]byte(src), parse(t, src), opts); err != nil {
		t.Errorf("whitespaceOnly guard rejected the format: %v", err)
	}
}
