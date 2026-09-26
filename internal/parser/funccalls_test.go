package parser

import (
	"slices"
	"testing"
)

// A function that calls nothing has no entry in the call map, and that is not
// the same question as "is this a function scope at all". Answering both by
// falling back to every call in the file handed a leaf method its siblings'
// calls, so `deps` drew an edge out of an empty function — labelled, since the
// CallSite came from elsewhere, with another function's line number.
func TestFuncCallsDoesNotLendOneFunctionAnothersCalls(t *testing.T) {
	src := `component {
	function doesCall() {
		return svc.work();
	}

	function callsNothing() {
		return 1;
	}
}
`

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	if len(pr.Scopes) != 2 {
		t.Fatalf("expected two scopes, got %+v", pr.Scopes)
	}

	for _, sc := range pr.Scopes {
		var got []string
		for _, c := range pr.FuncCalls(sc.Start, sc.End) {
			got = append(got, c.Variable+"."+c.FuncName)
		}

		want := []string(nil)
		if sc.Name == "doesCall" {
			want = []string{"svc.work"}
		}

		if !slices.Equal(got, want) {
			t.Errorf("%s: got %v want %v", sc.Name, got, want)
		}
	}
}

// AllCalls is what a whole-file scan wants, and it is now asked for by name
// rather than by handing FuncCalls a range it will fail to match.
func TestAllCallsCoversEveryCallSite(t *testing.T) {
	src := `component {
	variables.svc = new models.Svc();
	variables.svc.atFileLevel();

	function a() {
		return one.x();
	}

	function b() {
		return two.y();
	}
}
`

	pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

	var got []string
	for _, c := range pr.AllCalls() {
		got = append(got, c.Variable+"."+c.FuncName)
	}

	slices.Sort(got)

	want := []string{"one.x", "two.y", "variables.svc.atFileLevel"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

// Without ExtractCalls the answer is parsed on demand per function, and that
// path was always per-function. It must still be.
func TestFuncCallsWithoutExtractCallsIsAlsoPerFunction(t *testing.T) {
	src := `component {
	function doesCall() {
		return svc.work();
	}

	function callsNothing() {
		return 1;
	}
}
`

	pr := Parse(testURI, src)

	for _, sc := range pr.Scopes {
		n := len(pr.FuncCalls(sc.Start, sc.End))
		if (sc.Name == "doesCall") != (n > 0) {
			t.Errorf("%s: got %d calls", sc.Name, n)
		}
	}
}

// The buckets come out of a map, so the order changed between runs of one
// build — and `unresolved`, `explain` and the code map each write a report a
// reader is meant to diff against an earlier one.
func TestAllCallsIsOrderedByLine(t *testing.T) {
	src := `component {
	function a() {
		return one.x();
	}

	function b() {
		return two.y();
	}

	function c() {
		return three.z();
	}
}
`

	want := []string{"one.x", "two.y", "three.z"}

	for range 20 {
		pr := ParseWithOptions(testURI, src, &ParseOptions{ExtractCalls: true})

		var got []string
		for _, c := range pr.AllCalls() {
			got = append(got, c.Variable+"."+c.FuncName)
		}

		if !slices.Equal(got, want) {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
