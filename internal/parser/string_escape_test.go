package parser

import (
	"slices"
	"strings"
	"testing"
)

// tokenValues returns every token's raw text, so a test can assert where the
// scanner decided a string ends rather than inferring it from a parse.
func tokenValues(src string) []string {
	sc := NewScanner(src)

	var out []string

	for {
		tok := sc.NextSkipComments()
		if tok.Kind == TokEOF {
			return out
		}

		out = append(out, tok.Value)
	}
}

// CFML has no backslash escape. The scanner honoured one — C's rule, not this
// language's — so a string ending in a backslash did not close where it ends: it
// closed at the next quote anywhere in the file and swallowed every call in
// between. `listLast( uri, "/\" )` is an ordinary idiom and one occurrence of it
// cost 333 call sites in a single corpus component.
func TestCFMLHasNoBackslashEscape(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`x = "/\";`, `"/\"`},
		{`x = "C:\";`, `"C:\"`},
		{`x = 'a\';`, `'a\'`},
		{`x = "a\\b";`, `"a\\b"`},
		{`x = "\n";`, `"\n"`},
	} {
		t.Run(c.src, func(t *testing.T) {
			got := tokenValues(c.src)
			if !slices.Contains(got, c.want) {
				t.Errorf("no token %q in %q", c.want, got)
			}
		})
	}

	// What the mis-scan actually cost: the calls after such a string.
	src := "component {\n function go() {\n" +
		"  expect( a ).toBe( listLast( uri, \"/\\\" ) );\n" +
		"  expect( b ).toBeTrue();\n" +
		" }\n}\n"

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	var got []string

	for _, call := range pr.AllCalls() {
		got = append(got, call.FuncName)
	}

	slices.Sort(got)

	want := []string{"expect", "expect", "listLast", "toBe", "toBeTrue"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

// Doubling is how CFML escapes a quote, and it is checked from inside the
// string — so a bare `""` is still the empty string and `""""` is a string
// holding one quote. Without the rule a `""`-escaped string came apart into
// several tokens, which is also what put an interpolated call on the wrong line
// (see the multi-line test below).
func TestADoubledQuoteEscapesAQuote(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`x = "say ""hi""";`, `"say ""hi"""`},
		{`x = 'it''s';`, `'it''s'`},
		{`x = "";`, `""`},
		{`x = """";`, `""""`},
		{`f("", "b");`, `""`},
		{`x = "a"" ( b";`, `"a"" ( b"`},
	} {
		t.Run(c.src, func(t *testing.T) {
			if got := tokenValues(c.src); !slices.Contains(got, c.want) {
				t.Errorf("no token %q in %q", c.want, got)
			}
		})
	}

	// `f("", "b")` must still see two arguments, not one string swallowing the
	// comma — the trap the doubling rule sets if it is applied to the opening
	// quote rather than from inside the string.
	if got := tokenValues(`f("", "b");`); !slices.Contains(got, `"b"`) {
		t.Errorf(`f("", "b") lost its second argument: %q`, got)
	}
}

// Both string scanners must agree, and only one of them is reached by a plain
// tokenisation. `scanString` tries the interpolation-aware `scanQuoted` first
// when the text is known to be CFScript and falls back to `scanQuotedPlain`
// when the nesting does not close — so a `scanQuoted` that still honours a
// backslash escape is *hidden* by the fallback on `"/\"`, and a test that only
// parsed a component passed with that half reverted. Asserting the token text
// under the flag is what reaches it.
func TestBothStringScannersApplyTheSameEscapeRule(t *testing.T) {
	interpTokens := func(src string) []string {
		sc := NewScanner(src)
		sc.interpStrings = true

		var out []string

		for {
			tok := sc.NextSkipComments()
			if tok.Kind == TokEOF {
				return out
			}

			out = append(out, tok.Value)
		}
	}

	for _, c := range []struct{ src, want string }{
		{`x = "say ""hi""" & "#f()#";`, `"say ""hi"""`},
		{`x = 'it''s' & "#f()#";`, `'it''s'`},
		{`x = "a"" #f()# ""b";`, `"a"" #f()# ""b"`},
		{`x = "/" & "#f()#";`, `"/"`},
	} {
		t.Run(c.src, func(t *testing.T) {
			if got := interpTokens(c.src); !slices.Contains(got, c.want) {
				t.Errorf("no token %q in %q", c.want, got)
			}
		})
	}
}

// A `""`-escaped string is how CFML writes markup across several lines, so a
// string token spanning lines is ordinary rather than exotic. The interpolated
// call inside one belongs on its own line, not on the line the string opened:
// before this the whole span was reported at the opening line, which put
// go-to-definition and every `deps` label several lines short.
func TestAMultiLineStringAttributesItsInterpolationToItsOwnLine(t *testing.T) {
	src := "component {\n function go() {\n" +
		"  writeOutput(\n" +
		"   \"\n" +
		"   <img\n" +
		"    class=\"\"#cls#\"\"\n" +
		"    src=\"\"#generateLink( x )#\"\"\n" +
		"   >\"\n" +
		"  );\n" +
		" }\n}\n"

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	before, _, ok := strings.Cut(src, "generateLink")
	if !ok {
		t.Fatal("fixture does not contain generateLink")
	}

	want := uint32(strings.Count(before, "\n"))

	var found bool

	for _, c := range pr.AllCalls() {
		if c.FuncName == "generateLink" {
			found = true

			if c.Line != want {
				t.Errorf("generateLink reported on line %d, want %d", c.Line, want)
			}
		}
	}

	if !found {
		t.Fatal("generateLink was not recorded at all")
	}
}
