package parser

import (
	"slices"
	"testing"
)

// hopNames is a CallSite.Chain as the methods it calls, for tests that
// assert the walk rather than the arguments each hop carries.
func hopNames(chain []string) []string {
	names := make([]string, len(chain))
	for i, hop := range chain {
		names[i] = CallHopName(hop)
	}

	return names
}

// TestAChainHopCarriesItsArguments: a hop between the receiver and the call
// keeps its argument list, which is what an argument-sensitive return is read
// from. The resolver used to recover it by finding the hop's name on the
// line, which answers nothing when the name is called there twice.
func TestAChainHopCarriesItsArguments(t *testing.T) {
	src := `component {
	function run() {
		dao.read( new Product() ).work(); dao.read( 1 ).other();
		load( 2 ).find( "x" ).go();
		x = svc.get( "a" ).make( b ).done();
	}
}`
	pr := ParseWithOptions("file:///t/Page.cfc", src, &ParseOptions{ExtractCalls: true})

	want := map[string][]string{
		"work":  {"$call:read( new Product() )"},
		"other": {"$call:read( 1 )"},
		"find":  {`$call:load( 2 )`},
		"go":    {`$call:load( 2 )`, `$call:find( "x" )`},
		"make":  {`$call:get( "a" )`},
		"done":  {`$call:get( "a" )`, `$call:make( b )`},
	}

	got := map[string][]string{}

	for _, c := range pr.AllCalls() {
		if _, ok := want[c.FuncName]; ok {
			got[c.FuncName] = c.Chain
		}
	}

	for name, w := range want {
		if !slices.Equal(got[name], w) {
			t.Errorf("%s: Chain %q, want %q", name, got[name], w)
		}
	}
}
