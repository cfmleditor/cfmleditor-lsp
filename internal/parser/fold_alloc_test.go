package parser

import (
	"fmt"
	"strings"
	"testing"
)

// foldAllocDoc builds the same component twice over: once with mixed-case
// identifiers and once with those identifiers lowercased. Everything else about
// the two documents — structure, length, token count, the work the parser does
// on them — is identical, so what separates them is only whether the parser's
// case-insensitive switches have anything to fold.
func foldAllocDoc(mixedCase bool) string {
	var b strings.Builder

	b.WriteString("component extends=\"base.AbstractService\" {\n")

	for i := range 40 {
		name := fmt.Sprintf("getThingNumber%d", i)
		result := fmt.Sprintf("resultHolder%d", i)

		if !mixedCase {
			name, result = strings.ToLower(name), strings.ToLower(result)
		}

		fmt.Fprintf(&b, "\tpublic struct function %s(required numeric identValue) {\n"+
			"\t\tvar %s = {};\n"+
			"\t\tvar daoHandle = new model.UserDAO();\n"+
			"\t\t%s.id = arguments.identValue;\n"+
			"\t\treturn %s;\n\t}\n", name, result, result, result)
	}

	b.WriteString("}\n")

	return b.String()
}

// TestFoldingAnIdentifierDoesNotAllocate is a ratio test, for the reason the
// scaling tests in this repo are: what it pins is not a number of allocations,
// which moves with every unrelated change to the parser, but that a capital
// letter in an identifier costs nothing extra to switch on.
//
// strings.ToLower returns its argument untouched when there is nothing to
// lower, so the parser's case-insensitive switches were free on all-lowercase
// source and allocated a throwaway string per identifier on everything else —
// and real CFML is full of getUser, userDAO and ARGUMENTS. A test that parsed
// one document could not see that: the answers were right either way, and the
// count on its own says nothing about which half of it the capitals paid for.
//
// The remaining gap is the scope sets, which are maps keyed by the lowercased
// name and so need a string that outlives the comparison. With the fold on the
// switches reverted the ratio is 1.42; with it, 1.07.
func TestFoldingAnIdentifierDoesNotAllocate(t *testing.T) {
	if testing.Short() {
		t.Skip("measures allocations over many parses")
	}

	const maxRatio = 1.15

	allocs := func(doc string) float64 {
		r := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				ParseWithOptions("file:///fold.cfc", doc, ParseOptions{ExtractCalls: true})
			}
		})

		return float64(r.AllocsPerOp())
	}

	lower := allocs(foldAllocDoc(false))
	mixed := allocs(foldAllocDoc(true))

	if lower == 0 {
		t.Fatal("lowercase parse reported no allocations — the benchmark did not run")
	}

	if ratio := mixed / lower; ratio > maxRatio {
		t.Errorf("mixed-case parse allocates %.0f against %.0f for the same document lowercased "+
			"(ratio %.2f, want <= %.2f) — a case-insensitive switch is building a lowercased string",
			mixed, lower, ratio, maxRatio)
	}
}
