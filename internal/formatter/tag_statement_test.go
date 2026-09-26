package formatter

import "testing"

// TestTagStatementBodyFormatted covers a script-syntax CF tag with a body —
// `lock`, `transaction`, `query`, `loop`, `savecontent`, `thread` and the call
// form `cfhttp(…) { … }`. With no renderer, the node went to scriptRaw, which
// trims every line and writes them all at the statement's level: the body came
// out flat, nested blocks included, and none of it was formatted. 811 corpus
// components have one.
func TestTagStatementBodyFormatted(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "attribute form with nested blocks",
			src:  "lock name=\"x\" type=\"exclusive\" {\nif (a) {\nb();\n}\n}",
			want: "    lock name=\"x\" type=\"exclusive\" {\n\n" +
				"        if ( a ) {\n\n" +
				"            b();\n\n" +
				"        }\n\n" +
				"    }",
		},
		{
			name: "no attributes",
			src:  "transaction {\nx();\n}",
			want: "    transaction {\n\n        x();\n\n    }",
		},
		{
			name: "query tag, brace against the name",
			src:  "query{\necho(\"SELECT 1\");\n}",
			want: "    query {\n\n        echo(\"SELECT 1\");\n\n    }",
		},
		{
			name: "call form",
			src:  "cfhttp(url=\"u\", method=\"get\") {\ncfhttpparam(name=\"a\", value=\"b\");\n}",
			want: "    cfhttp(url=\"u\", method=\"get\") {\n\n        cfhttpparam(name = \"a\", value = \"b\");\n\n    }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, formatStable(t, wrap(tt.src)), tt.want)
		})
	}
}

// TestTagStatementKeptAsWritten pins the two shapes scriptTagStatement leaves to
// scriptRaw: a body the author wrote on one line, and a header holding a `//`
// comment, where the opening brace written after it would land in the comment.
func TestTagStatementKeptAsWritten(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"one-line body", "query name=\"q\" { echo(\"SELECT 1\"); }", "query name=\"q\" { echo(\"SELECT 1\"); }"},
		{"line comment in the header", "lock name=\"y\" // why\ntype=\"readonly\" {\nz();\n}", "lock name=\"y\" // why\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertContains(t, formatStable(t, wrap(tt.src)), tt.want)
		})
	}
}

// TestMixedArgSeparatorsKept covers a tag call whose attributes are separated
// partly by commas and partly by spaces, as in Lucee's LDEV4128 test:
// `cflog(file="#name#" text="load test", type="error")`. Joining with commas put
// one where the source had a space, which the guard rejected. It surfaced once
// tag-statement bodies were formatted, since that is where the call sat.
func TestMixedArgSeparatorsKept(t *testing.T) {
	call := "cflog(file=\"#n#\" text=\"load test\", type=\"error\");"

	for name, src := range map[string]string{
		"statement":     wrap(call),
		"in a tag body": wrap("loop times=3 {\n" + call + "\n}"),
	} {
		t.Run(name, func(t *testing.T) {
			assertContains(t, formatStable(t, src), call)
		})
	}
}
