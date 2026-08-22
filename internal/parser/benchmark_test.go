package parser

import (
	"strings"
	"testing"
	"time"
)

// Realistic script-based CFC with multiple functions, arguments, and component refs.
var benchScriptCFC = `component extends="base.AbstractService" {

	property name="userDAO" inject="model.UserDAO";
	property name="logger" inject="coldbox:logger";

	this.name = "UserService";
	variables.instance = {};

` + strings.Repeat(`
	public struct function getUser(required numeric userID, boolean includeRoles=true) {
		var result = {};
		var qry = "";
		var userDAO = new model.UserDAO();
		var cache = createObject("component", "utils.CacheManager");
		result.id = arguments.userID;
		result.roles = includeRoles ? getRoles(userID) : [];
		return result;
	}
`, 20) + `}`

// Realistic tag-based CFC.
var benchTagCFC = `<cfcomponent extends="base.AbstractService" output="false">
	<cfproperty name="dsn" type="string" />
` + strings.Repeat(`
	<cffunction name="getData" returntype="query" access="public" output="false">
		<cfargument name="id" type="numeric" required="true" />
		<cfargument name="includeArchived" type="boolean" required="false" default="false" />
		<cfset var qry = "" />
		<cfset var result = createObject("component", "models.Result").init() />
		<cfquery name="qry" datasource="#variables.dsn#">
			SELECT id, name, email, created
			FROM users
			WHERE id = <cfqueryparam cfsqltype="CF_SQL_INTEGER" value="#arguments.id#">
			<cfif arguments.includeArchived>
				AND archived = 0
			</cfif>
		</cfquery>
		<cfreturn qry />
	</cffunction>
`, 20) + `</cfcomponent>`

func BenchmarkParse_ScriptCFC(b *testing.B) {
	for b.Loop() {
		Parse("file:///bench.cfc", benchScriptCFC)
	}
}

func BenchmarkParse_TagCFC(b *testing.B) {
	for b.Loop() {
		Parse("file:///bench.cfc", benchTagCFC)
	}
}

func BenchmarkParse_ScriptCFC_ExtractCalls(b *testing.B) {
	for b.Loop() {
		ParseWithOptions("file:///bench.cfc", benchScriptCFC, ParseOptions{ExtractCalls: true})
	}
}

func BenchmarkParse_TagCFC_ExtractCalls(b *testing.B) {
	for b.Loop() {
		ParseWithOptions("file:///bench.cfc", benchTagCFC, ParseOptions{ExtractCalls: true})
	}
}

func BenchmarkApplyEdit_InFunc(b *testing.B) {
	for b.Loop() {
		b.StopTimer()

		pr := Parse("file:///bench.cfc", benchScriptCFC)

		b.StartTimer()
		pr.ApplyEdit(4, 0, 4, 0, "\t\tvar z = 1;\n")
	}
}

func BenchmarkApplyEdit_Global(b *testing.B) {
	for b.Loop() {
		b.StopTimer()

		pr := Parse("file:///bench.cfc", benchScriptCFC)

		b.StartTimer()
		pr.ApplyEdit(0, 0, 0, 0, "// comment\n")
	}
}

func BenchmarkFuncVars(b *testing.B) {
	pr := Parse("file:///bench.cfc", benchScriptCFC)
	if len(pr.Scopes) == 0 {
		b.Fatal("no scopes")
	}

	s := pr.Scopes[0]
	for b.Loop() {
		pr.InvalidateFunc(s.Start, s.End)
		pr.FuncVars(s.Start, s.End)
	}
}

func BenchmarkGlobalVars(b *testing.B) {
	pr := Parse("file:///bench.cfc", benchScriptCFC)
	for b.Loop() {
		pr.mu.Lock()
		pr.globalDone = false
		pr.mu.Unlock()
		pr.GlobalVars()
	}
}

// TestParsePerformance ensures parsing stays within acceptable time budgets.
// Thresholds are set at ~3x the baseline to allow for CI variance.
func TestParsePerformance(t *testing.T) {
	// Wall-clock thresholds are meaningless on a shared or loaded machine, and
	// these are tight enough (hundreds of microseconds) that CI runners trip
	// them routinely. Skipped under -short so automated runs stay meaningful;
	// run the suite without -short to actually measure.
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}

	const iterations = 100

	tests := []struct {
		name     string
		maxPerOp time.Duration
		fn       func()
	}{
		{"Parse_ScriptCFC", 200 * time.Microsecond, func() {
			Parse("file:///bench.cfc", benchScriptCFC)
		}},
		{"Parse_TagCFC", 500 * time.Microsecond, func() {
			Parse("file:///bench.cfc", benchTagCFC)
		}},
		{"ApplyEdit_InFunc", 500 * time.Microsecond, func() {
			pr := Parse("file:///bench.cfc", benchScriptCFC)
			pr.ApplyEdit(4, 0, 4, 0, "\t\tvar z = 1;\n")
		}},
		{"FuncVars", 200 * time.Microsecond, func() {
			pr := Parse("file:///bench.cfc", benchScriptCFC)
			s := pr.Scopes[0]
			pr.FuncVars(s.Start, s.End)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Warm up
			for range 10 {
				tt.fn()
			}

			start := time.Now()

			for range iterations {
				tt.fn()
			}

			elapsed := time.Since(start)

			perOp := elapsed / iterations
			if perOp > tt.maxPerOp {
				t.Errorf("performance regression: %v/op exceeds threshold %v/op", perOp, tt.maxPerOp)
			} else {
				t.Logf("%v/op (threshold %v)", perOp, tt.maxPerOp)
			}
		})
	}
}

// ResolverSet.Resolve is on the completion and hover path, with a realistic
// config of a few dozen resolvers. It is also where the first-byte index has to
// narrow the candidates without reordering them, so it is worth watching: the
// sort that restores configuration order also collapses the duplicates a
// pipe-delimited prefix used to contribute, which is why it is not a slowdown.
func BenchmarkResolverSetResolve(b *testing.B) {
	resolvers := []Resolver{
		{Prefix: "getDirectContent", Match: "getDirectContent()", Resolve: "pdf.passthrough"},
		{Prefix: "get", Match: "get$1()", Resolve: "packages.tass.${1:lower}"},
		{Prefix: "kernel", Match: `kernel\.get([A-Za-z0-9_]+)\(\)`, Resolve: "packages.$1"},
	}

	for i := range 37 {
		name := string(rune('a'+i%26)) + "svc"
		resolvers = append(resolvers, Resolver{Prefix: name, Match: name + ".$1", Resolve: "app.svc"})
	}

	rs := BuildResolverSet(resolvers)

	b.ResetTimer()

	for range b.N {
		_ = rs.Resolve("VARIABLES._document.getDirectContent()")
		_ = rs.Resolve("kernel.getFoo()")
		_ = rs.Resolve("nothing.matches.here()")
	}
}
