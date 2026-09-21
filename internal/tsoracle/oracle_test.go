package tsoracle

import (
	"fmt"
	"os"
	"path/filepath"
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

	"init":      "gap: `.init()` chained after createObject(...) in a <cfset> is not recorded",
	"len":       "gap: a call in a <cfif>/<cfelseif> condition is not recorded",
	"trim":      "gap: same, nested inside that condition",
	"isdefined": "gap: same, in a <cfelseif>",
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

// TestKnownTagSyntaxGaps states the shapes the oracle turned up, so each fails
// when it starts working.
//
// They are one class: in tag syntax a call in a condition, in a <cfreturn>, or
// chained onto an instantiation is not recorded, while the same code in script
// syntax is.
func TestKnownTagSyntaxGaps(t *testing.T) {
	for _, src := range []string{
		`<cfif svc.isValid(x)><cfset y = 1></cfif>`,
		`<cfif NOT svc.check()>x</cfif>`,
		`<cfreturn svc.value()>`,
		`<cfset d = createObject("component","models.Dao").init("ds")>`,
		`<cfset d = new models.Dao().init("ds")>`,
	} {
		t.Run(src, func(t *testing.T) {
			if got := parserCalls("/t.cfm", []byte(src)); len(got) != 0 {
				t.Errorf("this gap is closed — record it and delete the case: got %v", got)
			}

			if len(GrammarCalls([]byte(src))) == 0 {
				t.Errorf("the grammar sees nothing either, so this case proves nothing")
			}
		})
	}

	// The control: the same shape in script syntax works, which is what makes
	// the above a gap in the tag parser rather than a decision.
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
