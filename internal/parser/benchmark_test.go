package parser

import (
	"slices"
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

// perfTimer measures only the parts of an iteration a case asks it to, the way
// a testing.B does with StopTimer/StartTimer. A case that needs per-iteration
// setup pauses it while building that setup, so what the case costs is not
// swamped by what it takes to get there.
type perfTimer struct {
	total   time.Duration
	started time.Time
}

func (p *perfTimer) pause() {
	p.total += time.Since(p.started)
}

func (p *perfTimer) resume() {
	p.started = time.Now()
}

const (
	perfRounds     = 5
	perfIterations = 100
)

// bestPerOp times fn over several rounds and reports the fastest round's
// per-operation cost along with the slowest.
//
// The fastest round is the verdict because scheduler noise is one-sided: a
// round can be slowed by whatever else the machine is doing, never sped up by
// it. So the minimum is the closest thing to the true cost that wall-clock
// timing can report, while a mean is dragged around by a single unlucky
// preemption. The slowest is kept only to put a number on how noisy the run
// was when a failure has to be explained.
func bestPerOp(fn func(*perfTimer)) (best, worst time.Duration) {
	rounds := make([]time.Duration, 0, perfRounds)

	for range perfRounds {
		tm := &perfTimer{}
		tm.resume()

		for range perfIterations {
			fn(tm)
		}

		tm.pause()

		rounds = append(rounds, tm.total/perfIterations)
	}

	return slices.Min(rounds), slices.Max(rounds)
}

// TestParsePerformance guards the hot parse paths against an order-of-magnitude
// regression. That is the most a wall-clock assertion can honestly claim, and
// two things had to change before this one could claim even that.
//
// The thresholds are 5x the cost measured on a four-core container, with each
// case's baseline recorded beside it. This comment used to claim "~3x the
// baseline" while the numbers underneath it left between 3% and 8% headroom, so
// three of the four cases failed on an idle machine, every run — and a test
// that is always red teaches people to stop reading it.
//
// 5x rather than the 3x claimed, because 3x costs sensitivity nobody has and
// buys flakiness everyone pays for. The regressions a test like this can catch
// at all are algorithmic — a lost memo, a regex recompiled per call, an
// accidental O(n^2) — and those are an order of magnitude, not 20%; for 20%
// there are the benchmarks beside it, which the perf job publishes every run.
// Meanwhile 3x has only 1.5x left once a build is running in another terminal,
// which is the state the machine is usually in when someone runs the suite.
//
// The other change is what gets measured. Three cases built a fresh
// ParseResult inside the timed region, so what they timed was Parse plus a
// rounding error: FuncVars costs ~8µs beside the ~185µs Parse it was being
// timed with, which left it unable to see a 20x regression in itself but able
// to fail on a 5% slowdown in Parse. Setup is now paused out, and the verdict
// is the fastest of several rounds rather than the mean of one (see bestPerOp).
//
// Still skipped under -short, because none of this makes a microsecond budget
// safe on a shared runner. CI's build-test and race jobs pass -short; the
// informational perf job does not, and publishes these numbers.
func TestParsePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}

	// Two cases index Scopes[0]. Checked once here so a parser that stops
	// finding functions fails with this rather than with an index panic from
	// inside the timing loop.
	if len(Parse("file:///bench.cfc", benchScriptCFC).Scopes) == 0 {
		t.Fatal("no scopes parsed from benchScriptCFC")
	}

	tests := []struct {
		name     string
		maxPerOp time.Duration
		fn       func(*perfTimer)
	}{
		// Baseline ~185µs.
		{"Parse_ScriptCFC", 900 * time.Microsecond, func(*perfTimer) {
			Parse("file:///bench.cfc", benchScriptCFC)
		}},
		// Baseline ~320µs.
		{"Parse_TagCFC", 1600 * time.Microsecond, func(*perfTimer) {
			Parse("file:///bench.cfc", benchTagCFC)
		}},
		// Baseline ~195µs. ApplyEdit consumes the result it edits — it rewrites
		// Content and shifts every scope after the edit — so each iteration
		// needs a fresh one, and building it is setup rather than the thing
		// being measured.
		{"ApplyEdit_InFunc", time.Millisecond, func(tm *perfTimer) {
			tm.pause()

			pr := Parse("file:///bench.cfc", benchScriptCFC)

			tm.resume()

			pr.ApplyEdit(4, 0, 4, 0, "\t\tvar z = 1;\n")
		}},
		// Baseline ~8µs. FuncVars memoizes per function, so the invalidation is
		// part of the operation: without it every iteration after the first is
		// a map lookup and the case measures nothing.
		{"FuncVars", 40 * time.Microsecond, func(tm *perfTimer) {
			tm.pause()

			pr := Parse("file:///bench.cfc", benchScriptCFC)
			s := pr.Scopes[0]

			tm.resume()

			pr.InvalidateFunc(s.Start, s.End)
			pr.FuncVars(s.Start, s.End)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Warm up. The timer is only here to satisfy the signature; what
			// these iterations record is thrown away with it.
			warm := &perfTimer{}
			warm.resume()

			for range 10 {
				tt.fn(warm)
			}

			best, worst := bestPerOp(tt.fn)

			if best > tt.maxPerOp {
				t.Errorf("performance regression: %v/op (best of %d rounds; slowest round %v) exceeds threshold %v/op",
					best, perfRounds, worst, tt.maxPerOp)
			} else {
				t.Logf("%v/op (slowest round %v, threshold %v)", best, worst, tt.maxPerOp)
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
