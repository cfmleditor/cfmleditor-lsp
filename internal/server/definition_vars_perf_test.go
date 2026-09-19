package server

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// varCorpus is a document with declarations spread across every scope, inside
// functions and outside them, with names that repeat.
func varCorpus(t testing.TB, n int) (string, []parser.VarDef) {
	t.Helper()

	var b strings.Builder

	scopes := []string{"variables", "url", "form", "application", "request", "session", "server", "cgi", "client"}
	names := []string{"total", "count", "name", "id", "result", "cfg"}

	rng := rand.New(rand.NewSource(9))

	b.WriteString("<cfcomponent>\n")

	for i := range n {
		if i%7 == 0 {
			fmt.Fprintf(&b, "\t<cffunction name=\"f%d\">\n\t\t<cfargument name=\"%s\">\n\t\t<cfset var %s = 1>\n\t</cffunction>\n",
				i, names[rng.Intn(len(names))], names[rng.Intn(len(names))])

			continue
		}

		fmt.Fprintf(&b, "\t<cfset %s.%s = %d>\n", scopes[rng.Intn(len(scopes))], names[rng.Intn(len(names))], i)
	}

	b.WriteString("</cfcomponent>\n")

	content := b.String()

	return content, parser.ParseVars(content)
}

// bestVarDeclPerScope answers for nine scopes in one pass; bestVarDecl answers
// for one at a time. They must agree, or the faster one is quietly returning a
// different declaration — which looks like go-to-definition landing on the wrong
// assignment, not like an error.
func TestOnePassAgreesWithPerScopeSearch(t *testing.T) {
	content, vars := varCorpus(t, 3000)
	lines := strings.Count(content, "\n")

	rng := rand.New(rand.NewSource(4))

	for range 400 {
		name := []string{"total", "count", "name", "id", "result", "cfg", "absent"}[rng.Intn(7)]
		line := rng.Intn(lines)

		got := bestVarDeclPerScope(vars, name, unscopedSearchOrder, line)

		for i, sc := range unscopedSearchOrder {
			want := bestVarDecl(vars, name, sc, line)

			switch {
			case want == nil && got[i] == nil:
			case want == nil || got[i] == nil:
				t.Fatalf("%s at line %d scope %s: one pass gave %v, per-scope gave %v",
					name, line, parser.ScopeName(sc), got[i], want)
			case *want != *got[i]:
				t.Fatalf("%s at line %d scope %s: one pass gave %+v, per-scope gave %+v",
					name, line, parser.ScopeName(sc), *got[i], *want)
			}
		}
	}
}

// A lookup must not allocate a copy of the document.
//
// varLocation split the whole file into lines to read the one line a
// declaration sits on: a megabyte per lookup on a large component, for a single
// row of it. And an unscoped name walked every declaration once per scope, nine
// times over, when the answer is usually in the first scope or nowhere.
func TestVariableLookupDoesNotCopyTheDocument(t *testing.T) {
	content, vars := varCorpus(t, 4000)
	docURI := uri.URI("file://" + testdataDir() + "/VarPerf.cfc")

	r := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			matchVarInFile(vars, "total", len(content)/40, false, parser.ScopeVariables, docURI, content)
		}
	})

	perOp := r.AllocedBytesPerOp()
	t.Logf("document %d bytes, %d declarations, lookup allocates %d bytes/op", len(content), len(vars), perOp)

	// Generous, and still three orders of magnitude below a copy of the file.
	if perOp > 4096 {
		t.Errorf("an unscoped lookup allocates %d bytes over a %d byte document; it is copying the document rather than reading it",
			perOp, len(content))
	}
}

// One pass over the declarations must beat one pass per scope.
//
// The alternative is what this replaced: ask each scope in turn, walking every
// declaration in the file again each time. The comparison has to be against
// that, not against a single scope that happens to answer first — which is what
// the first version of this test measured, making a 2.7x improvement look like a
// 10x regression.
func TestOnePassBeatsOneScanPerScope(t *testing.T) {
	content, vars := varCorpus(t, 4000)
	line := len(content) / 40

	onePass := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			bestVarDeclPerScope(vars, "total", unscopedSearchOrder, line)
		}
	})

	perScope := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			for _, sc := range unscopedSearchOrder {
				if bestVarDecl(vars, "total", sc, line) != nil {
					break
				}
			}
		}
	})

	ratio := float64(onePass.NsPerOp()) / float64(perScope.NsPerOp())
	t.Logf("one pass %dns vs per-scope %dns (%.2fx)", onePass.NsPerOp(), perScope.NsPerOp(), ratio)

	if ratio > 1 {
		t.Errorf("one pass over %d declarations cost %.2fx a scan per scope (%dns vs %dns); "+
			"it is not earning the bookkeeping it carries",
			len(vars), ratio, onePass.NsPerOp(), perScope.NsPerOp())
	}
}
