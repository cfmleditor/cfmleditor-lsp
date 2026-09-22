package tsoracle

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
	"go.lsp.dev/uri"
)

// expectedDifferences is every method name the two implementations currently
// disagree about across the repo's fixtures, and why.
//
// A difference outside this map fails the test, and a name in it that no longer
// differs fails too — so a gap cannot be closed unnoticed and the list cannot
// rot into decoration, which is the rule the definition conformance suite
// follows for the same reason.
//
// Distinguishing the two kinds below is the whole job of a differential check.
// The grammar is a second opinion, not ground truth: it has a vocabulary of its
// own, and the parser deliberately models some things differently.
var expectedDifferences = map[string]string{
	"createobject": "deliberate: the parser records a ComponentRef, not a CallSite",
	"entitynew":    "deliberate: an ORM entity factory, recorded as a ComponentRef like createObject",
	"entityload":   "deliberate: an ORM entity factory, recorded as a ComponentRef like createObject",
}

func parserCalls(path string, src []byte) []Call {
	pr := parser.ParseWithOptions(uri.File(path), string(src), parser.ParseOptions{ExtractCalls: true})

	out := make([]Call, 0, 16)
	for _, c := range pr.AllCalls() {
		out = append(out, Call{Line: c.Line, Method: strings.ToLower(c.FuncName)})
	}

	return out
}

func key(c Call) string { return fmt.Sprintf("%d:%s", c.Line, c.Method) }

// diff returns what the grammar saw and the parser did not, and the reverse, as
// multisets keyed on line and method.
func diff(grammar, hand []Call) (missed, invented []string) {
	count := map[string]int{}
	for _, c := range grammar {
		count[key(c)]++
	}

	for _, c := range hand {
		count[key(c)]--
	}

	for k, n := range count {
		for range n {
			missed = append(missed, k)
		}

		for range -n {
			invented = append(invented, k)
		}
	}

	sort.Strings(missed)
	sort.Strings(invented)

	return missed, invented
}

func methodOfKey(k string) string { return k[strings.IndexByte(k, ':')+1:] }

// TestGrammarAndParserAgreeOnCalls walks a corpus and compares what each
// implementation saw.
//
// Without CORPUS it walks the repo's own fixtures and holds them to
// expectedDifferences. With CORPUS — a colon-separated list of directories,
// like the formatter's corpus test — it reports instead of asserting, because a
// real workspace's differences are what you are running it to discover.
func TestGrammarAndParserAgreeOnCalls(t *testing.T) {
	roots, reporting := []string{"../../testdata"}, false
	if c := os.Getenv("CORPUS"); c != "" {
		roots, reporting = strings.Split(c, ":"), true
	}

	var files int

	seen := map[string]int{}

	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
			if walkErr != nil || info.IsDir() {
				return nil //nolint:nilerr // a corpus entry we cannot stat is skipped, not fatal
			}

			l := strings.ToLower(p)
			if !strings.HasSuffix(l, ".cfc") && !strings.HasSuffix(l, ".cfm") {
				return nil
			}

			src, readErr := os.ReadFile(p)
			if readErr != nil {
				return nil //nolint:nilerr // same: an unreadable file is skipped
			}

			files++

			missed, invented := diff(GrammarCalls(src), parserCalls(p, src))
			for _, m := range missed {
				seen[methodOfKey(m)]++
			}

			for _, m := range invented {
				seen["INVENTED "+methodOfKey(m)]++
			}

			if len(missed)+len(invented) > 0 && (reporting || testing.Verbose()) {
				t.Logf("%s\n  grammar-only: %v\n  parser-only:  %v", p, missed, invented)
			}

			return nil
		})
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}

	sort.Strings(names)

	for _, n := range names {
		t.Logf("  %4d  %s", seen[n], n)
	}

	if reporting {
		t.Logf("%d files scanned; the assertions below run only on the repo fixtures", files)

		return
	}

	for _, n := range names {
		if _, ok := expectedDifferences[n]; !ok {
			t.Errorf("new difference on %q (%d sites) — the grammar and the parser disagree about a call\n"+
				"if it is a real gap, fix it; if it is deliberate or an oracle mis-mapping, say so in expectedDifferences",
				n, seen[n])
		}
	}

	for n, why := range expectedDifferences {
		if seen[n] == 0 {
			t.Errorf("%q no longer differs (%s) — remove it from expectedDifferences", n, why)
		}
	}
}

// TestTagSyntaxRecordsCallsInExpressions covers the class this check turned up
// and that is now fixed: in tag syntax a call in a condition, in a <cfreturn>,
// or chained onto an instantiation was recorded nowhere, while the same code in
// script syntax was.
//
// These began as a known-gaps list that failed when a gap started working. They
// all started working, so they are regression tests now — which is the whole
// point of stating a gap as a test rather than a comment.
func TestTagSyntaxRecordsCallsInExpressions(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{`<cfif svc.isValid(x)><cfset y = 1></cfif>`, []string{"isvalid"}},
		{`<cfif NOT svc.check()>x</cfif>`, []string{"check"}},
		{`<cfif len(trim(u)) GT 0>x</cfif>`, []string{"len", "trim"}},
		{`<cfif a><cfelseif svc.other()>x</cfif>`, []string{"other"}},
		{`<cfreturn svc.value()>`, []string{"value"}},
		{`<cfset d = createObject("component","models.Dao").init("ds")>`, []string{"init"}},
		{`<cfset d = new models.Dao().init("ds")>`, []string{"init"}},

		// Controls. A condition with no call, an instantiation with no chain,
		// and a <cfelse> — which the fast skip now admits for <cfelseif>'s sake
		// — must all stay empty, and an ordinary <cfset> call must stay exactly
		// one rather than becoming two.
		{`<cfif a GT b>x</cfif>`, nil},
		{`<cfset d = new models.Dao()>`, nil},
		{`<cfset d = createObject("component","models.Dao")>`, nil},
		{`<cfelse>x`, nil},
		{`<cfset x = svc.y()>`, []string{"y"}},
	}

	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			var got []string
			for _, call := range parserCalls("/t.cfm", []byte(c.src)) {
				got = append(got, call.Method)
			}

			sort.Strings(got)

			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}

	// The script control that made these gaps rather than decisions.
	script := `component { function go() { if (svc.isValid(x)) { y = 1; } } }`
	if got := parserCalls("/t.cfc", []byte(script)); len(got) != 1 {
		t.Errorf("script control: got %v, want one call", got)
	}
}

// TestDeclarationAxisIsWeakerThanTheCallAxis records the measured limit of this
// approach, so nobody builds on an assumption it does not support.
//
// Of the shapes that were declaring phantom variables in this parser, the
// grammar disagrees with the old behaviour on only one. For the rest it agrees
// with the *bug*: `svc.save(force = true)` is an assignment_expression to the
// grammar too, and so is `query name="q"`. Those distinctions are semantic, not
// syntactic, so a syntax oracle cannot arbitrate them — the call axis is where
// this technique pays.
func TestDeclarationAxisIsWeakerThanTheCallAxis(t *testing.T) {
	cases := []struct {
		src            string
		grammarAgrees  bool
		whatItWouldSay string
	}{
		{`component extends="models.Base" { variables.real = 1; }`, false,
			"the grammar reads `extends` as a tag attribute, so it would have caught this"},
		{`component { function go() { svc.save(force = true); } }`, true,
			"the grammar calls `force` an assignment_expression too"},
		{`component { function go() { query name="q" datasource="ds" {} } }`, true,
			"the grammar calls `name` and `datasource` assignments too"},
	}

	caught := 0

	for _, c := range cases {
		declares := false

		for _, d := range grammarDecls([]byte(c.src)) {
			if strings.HasSuffix(d, "extends") || strings.HasSuffix(d, "force") ||
				strings.HasSuffix(d, "name") || strings.HasSuffix(d, "datasource") {
				declares = true
			}
		}

		if declares != c.grammarAgrees {
			t.Errorf("%s\n  grammar agrees with the old parser: got %v want %v (%s)",
				c.src, declares, c.grammarAgrees, c.whatItWouldSay)
		}

		if !declares {
			caught++
		}
	}

	if caught != 1 {
		t.Errorf("the declaration axis caught %d of %d shapes; the note above says one", caught, len(cases))
	}
}

// A <cfset> holds an expression, and the string paths that read one matched a
// few shapes and let the rest through — about 3,100 call sites on the corpus in
// PARSER-GAPS.md. It goes to the script parser now, topping up what those paths
// already recorded rather than replacing it.
func TestSetExpressionsRecordTheirCalls(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{`<cfset arrayAppend(ret, prefix & "." & key) />`, []string{"arrayappend"}},
		{`<cfset var name="test"&createuniqueid()>`, []string{"createuniqueid"}},
		{`<cfset attributes.req.list=cfc.listApplications()>`, []string{"listapplications"}},
		{`<cfset k = evaluate(fileread(v.indexFile)) />`, []string{"evaluate", "fileread"}},
		{`<cfset d = createObject("component","models.Dao").init("ds")>`, []string{"init"}},
		{`<cfset d = new models.Dao().init("ds")>`, []string{"init"}},

		// The shape the string path already matched stays at one call, not two.
		{`<cfset x = svc.y()>`, []string{"y"}},

		// And a genuine repeat stays at two, which is why the top-up counts
		// rather than tests presence.
		{`<cfset x = f() + f()>`, []string{"f", "f"}},

		// Controls.
		{`<cfset x = 1>`, nil},
		{`<cfset d = new models.Dao()>`, nil},
	}

	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			var got []string
			for _, call := range parserCalls("/t.cfm", []byte(c.src)) {
				got = append(got, call.Method)
			}

			sort.Strings(got)

			if !slices.Equal(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

// Two interpolations of the same function on one line are two calls. The
// <cfset> top-up must not reach them — running the shared merge with its tally
// on cost the second one, and only the corpus showed it.
func TestRepeatedInterpolationOnOneLineIsNotDeduped(t *testing.T) {
	for _, src := range []string{
		`<cfoutput>#getColdBoxSetting("a")# #getColdBoxSetting("b")#</cfoutput>`,
		`<li>#f("a")# (#f("b")#)</li>`,
	} {
		t.Run(src, func(t *testing.T) {
			if got := parserCalls("/t.cfm", []byte(src)); len(got) != 2 {
				t.Errorf("got %v, want two calls", got)
			}
		})
	}
}
