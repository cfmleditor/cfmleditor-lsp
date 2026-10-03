package parser

import (
	"testing"
)

// A return reads what every assignment that may reach it says. One in a block
// the return is outside of (a branch, a case, a loop body, a try or a catch)
// may or may not have run, so it joins what reached the block rather than
// replacing it. When those disagree the function has no return type; when
// one is dynamic, the return is. An assignment in a block holding the return
// always runs before it, and replaces what came before.
func TestReturnTypeComparesTheBranchesReachingIt(t *testing.T) {
	cases := []struct {
		name, want, content string
	}{
		{"if else disagree", "", `component {
function f( c ) {
	if ( c ) {
		var x = new models.A();
	} else {
		x = new models.B();
	}
	return x;
}
}`},
		{"if without else", "", `component {
function f( c ) {
	var x = new models.A();
	if ( c ) {
		x = new models.B();
	}
	return x;
}
}`},
		{"if else agree", "models.B", `component {
function f( c ) {
	if ( c ) {
		var x = new models.B();
	} else {
		x = new models.B();
	}
	return x;
}
}`},
		{"try catch", "", `component {
function f() {
	try {
		var x = new models.A();
	} catch ( any e ) {
		x = new models.B();
	}
	return x;
}
}`},
		{"one branch dynamic", "$any", `component {
function f( c, t ) {
	if ( c ) {
		var x = new models.A();
	} else {
		x = createObject( "component", "objs.#t#" );
	}
	return x;
}
}`},
		{"branch then overwritten", "models.B", `component {
function f( c ) {
	if ( c ) {
		var x = new models.A();
	}
	x = new models.B();
	return x;
}
}`},
		{"return inside the branch", "models.A", `component {
function f( c ) {
	var x = new models.B();
	if ( c ) {
		x = new models.A();
		return x;
	}
}
}`},
		{"switch cases", "", `component {
function f( k ) {
	switch ( k ) {
		case 1:
			var x = new models.A();
			break;
		default:
			x = new models.B();
	}
	return x;
}
}`},
		{"loop body", "", `component {
function f( items ) {
	var x = new models.A();
	for ( var i in items ) {
		x = new models.B();
	}
	return x;
}
}`},
		{"branches typed by calls", "", `component {
function makeA() { return new models.A(); }
function makeB() { return new models.B(); }
function f( c ) {
	if ( c ) {
		var x = makeA();
	} else {
		x = makeB();
	}
	return x;
}
}`},
		{"tag cfif cfelse disagree", "", `<cfcomponent>
<cffunction name="f">
	<cfargument name="c">
	<cfif arguments.c>
		<cfset var x = createObject("component", "models.A")>
	<cfelse>
		<cfset x = createObject("component", "models.B")>
	</cfif>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
		{"tag cfif cfelse agree", "models.B", `<cfcomponent>
<cffunction name="f">
	<cfargument name="c">
	<cfif arguments.c>
		<cfset var x = createObject("component", "models.B")>
	<cfelseif true>
		<cfset x = createObject("component", "models.B")>
	</cfif>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
		{"tag cftry cfcatch", "", `<cfcomponent>
<cffunction name="f">
	<cftry>
		<cfset var x = createObject("component", "models.A")>
		<cfcatch type="any">
			<cfset x = createObject("component", "models.B")>
		</cfcatch>
	</cftry>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
		{"tag cfloop", "", `<cfcomponent>
<cffunction name="f">
	<cfset var x = createObject("component", "models.A")>
	<cfloop from="1" to="3" index="i">
		<cfset x = createObject("component", "models.B")>
	</cfloop>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
		{"tag branch then overwritten", "models.B", `<cfcomponent>
<cffunction name="f">
	<cfargument name="c">
	<cfif arguments.c>
		<cfset var x = createObject("component", "models.A")>
	</cfif>
	<cfset x = createObject("component", "models.B")>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
		{"tag cflock runs its body", "models.B", `<cfcomponent>
<cffunction name="f">
	<cfset var x = createObject("component", "models.A")>
	<cflock name="l" timeout="1">
		<cfset x = createObject("component", "models.B")>
	</cflock>
	<cfreturn x>
</cffunction>
</cfcomponent>`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr := Parse(testURI, tc.content)

			for i := range pr.Funcs {
				if f := &pr.Funcs[i]; f.Name == "f" && f.ReturnComponent != tc.want {
					t.Errorf("f returns %q, want %q", f.ReturnComponent, tc.want)
				}
			}
		})
	}
}

// TestBracelessBodiesAreBlocks: a body written without braces
// (`if ( c ) x = new B();`, and the same after else, for and while) is a block
// of its own, so its assignment joins what came before rather than replacing
// it. Only a statement that is one line ending in a semicolon is bounded; any
// other braceless body is read as before, as one that always runs, which the
// last two cases pin so that a change to the rule shows.
func TestBracelessBodiesAreBlocks(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"if", "var x = new models.A();\n\tif ( c ) x = new models.B();", ""},
		{"if else", "if ( c ) var x = new models.A();\n\telse x = new models.B();", ""},
		{"braced if, braceless else", "var x = new models.A();\n\tif ( c ) {\n\t\tx = new models.B();\n\t} else x = new models.C();", ""},
		{"for", "var x = new models.A();\n\tfor ( var i in c ) x = new models.B();", ""},
		{"for with semicolons in its head", "var x = new models.A();\n\tfor ( var i = 0; i < 3; i++ ) x = new models.B();", ""},
		{"while", "var x = new models.A();\n\twhile ( c ) x = new models.B();", ""},
		{"else if chain, as a braced one: no exhaustiveness", "var x = new models.A();\n\tif ( c ) x = new models.B();\n\telse if ( d ) x = new models.B();\n\telse x = new models.B();", ""},
		{"nested", "var x = new models.A();\n\tif ( c ) if ( d ) x = new models.B();", ""},
		{"agreeing", "var x = new models.B();\n\tif ( c ) x = new models.B();", "models.B"},
		{"overwritten after", "var x = new models.A();\n\tif ( c ) x = new models.B(); x = new models.C();", "models.C"},
		{"overwritten on the next statement", "var x = new models.A();\n\tif ( c ) x = new models.B();\n\tx = new models.C();", "models.C"},
		// Not bounded: the statement runs over two lines, or has no semicolon.
		{"unbounded: two lines", "var x = new models.A();\n\tif ( c ) x =\n\t\tnew models.B();", "models.B"},
		{"unbounded: no semicolon", "var x = new models.A();\n\tif ( c ) x = new models.B()", "models.B"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr := Parse(testURI, "component {\nfunction f( c, d ) {\n\t"+tc.body+"\n\treturn x;\n}\n}")
			if len(pr.Funcs) != 1 {
				t.Fatalf("got %d functions, want 1", len(pr.Funcs))
			}

			if got := pr.Funcs[0].ReturnComponent; got != tc.want {
				t.Errorf("f returns %q, want %q", got, tc.want)
			}
		})
	}
}
