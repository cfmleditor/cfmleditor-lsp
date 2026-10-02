package resolve

import "testing"

// TestProducerPlansReadTheSourceAsWritten pins how a function's source is read
// into a plan and evaluated, each case a shape ColdBox's BaseTestCase uses.
func TestProducerPlansReadTheSourceAsWritten(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"models/A.cfc": `component {function onlyA(){}}`,
		"models/B.cfc": `component {function onlyB(){}}`,
		"Base.cfc":     `component {function set(){ return this; }}`,
		"Maker.cfc": `component extends="Base" {
// this.f() and variables.f() call the component's own f().
function viaThis(){ return this.make(); }
function viaVariables(){ return variables.make(); }
function make(){ return new models.A(); }

/**
 * A documented return still applies when the body only derives its value
 * from calls it cannot prove.
 * @return models.A
 */
function documented(){
	var t = fromScope()
	if ( !isNull( t ) ) {
		return t
	}
	lock name="x" timeout="1" {
		return fromScope()
	}
}
function fromScope(){ return request.cached; }

// A value given members its component does not declare is not that component.
function augmented(){ var x = new models.A(); x.extra = variables.make; return x; }
function onlySub(){}

// The assignment reaching the return is what is returned: after a second
// catch clause, and with no semicolons.
function catches(){ var x = new models.A(); try { x = new models.A(); } catch ( Foo e ) { x = new models.A(); } catch ( any e ) { } x = new models.B(); return x; }
function noSemicolons(){
	var x = new models.A()
	x = new models.B()
	return x
}
function run(){
	viaThis().onlyA();
	viaVariables().onlyA();
	documented().onlyA();
	augmented().extra();
	set().onlySub();
	catches().onlyB();
	noSemicolons().onlyB();
}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Maker.cfc"), map[string]string{
		"viaThis.onlyA":      "",
		"viaVariables.onlyA": "",
		"documented.onlyA":   "",
		"augmented.extra":    "method 'augmented' has no component return type (chain to 'extra')",
		"set.onlySub":        "",
		"catches.onlyB":      "",
		"noSemicolons.onlyB": "",
	})
}

// TestAPlanKeepsEveryStatement: a second catch clause is a handler, and a line
// break can end a statement. Read otherwise, the statement after the catches
// and every statement after the first line were swallowed into one, and the
// interpreter kept a stale value or lost the return.
func TestAPlanKeepsEveryStatement(t *testing.T) {
	methods := producerMethods(`component {
function catches(){ var x = a(); try { x = a(); } catch ( Foo e ) { x = a(); } catch ( any e ) { } x = b(); return x; }
function noSemicolons(){
	var x = a()
	x = b()
		.c()
	return x
}
function keyword(){
	var x = new
		b()
	return x
}
}`)

	kinds := func(name string) (out []string) {
		if methods[name] == nil {
			return []string{"no plan"}
		}

		for _, n := range methods[name].body {
			out = append(out, n.kind+" "+n.target+" "+n.expression)
		}

		return out
	}

	for name, want := range map[string][]string{
		"catches":      {"set local.x a ( )", "try  ", "set x b ( )", "return  x"},
		"nosemicolons": {"set local.x a ( )", "set x b ( ) . c ( )", "return  x"},
		"keyword":      {"set local.x new b ( )", "return  x"},
	} {
		got := kinds(name)
		if len(got) != len(want) {
			t.Errorf("%s: %q, want %q", name, got, want)

			continue
		}

		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s node %d: %q, want %q", name, i, got[i], want[i])
			}
		}
	}

	if methods["catches"] == nil || len(methods["catches"].body) < 2 {
		return
	}

	if alt := methods["catches"].body[1].alternative; len(alt) != 1 || alt[0].kind != "if" || len(alt[0].alternative) != 1 {
		t.Errorf("catches: the two handlers are not one alternative: %+v", alt)
	}
}
