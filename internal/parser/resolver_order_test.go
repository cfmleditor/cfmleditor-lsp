package parser

import "testing"

// Two code paths resolve an expression against componentResolvers:
// ResolveFromCallMatch walks the slice, and ResolverSet.Resolve narrows by a
// first-byte index first. CLAUDE.md documents one contract for both — resolvers
// are tried in array order, so listing a specific entry before a broad
// catch-all is how you override it — and CanResolveCall reaches the first path
// while completion and hover reach the second. They have to agree.
func TestResolverSetMatchesArrayOrder(t *testing.T) {
	tests := []struct {
		name      string
		resolvers []Resolver
		expr      string
		want      string
	}{
		{
			// The regression: "svc" is listed first but its prefix byte appears
			// at index 3, while "app" appears at 0. The byte index collected the
			// second resolver first and it won.
			name: "earlier resolver whose prefix appears later in the expression",
			resolvers: []Resolver{
				{Prefix: "svc", Match: "svc.$1", Resolve: "app.specific"},
				{Prefix: "app", Match: "app$1", Resolve: "app.generic"},
			},
			expr: "appsvc.foo()",
			want: "app.specific",
		},
		{
			name: "reversing the config reverses the winner",
			resolvers: []Resolver{
				{Prefix: "app", Match: "app$1", Resolve: "app.generic"},
				{Prefix: "svc", Match: "svc.$1", Resolve: "app.specific"},
			},
			expr: "appsvc.foo()",
			want: "app.generic",
		},
		{
			// Prefixes sharing a first byte land in one bucket, which was
			// already in array order — this case worked before and must stay.
			name: "specific entry ahead of a catch-all sharing its first byte",
			resolvers: []Resolver{
				{Prefix: "getDirectContent", Match: "getDirectContent()", Resolve: "pdf.passthrough"},
				{Prefix: "get", Match: "get$1()", Resolve: "packages.${1:lower}"},
			},
			expr: "getDirectContent()",
			want: "pdf.passthrough",
		},
		{
			name: "catch-all still fires for names it is meant to cover",
			resolvers: []Resolver{
				{Prefix: "getDirectContent", Match: "getDirectContent()", Resolve: "pdf.passthrough"},
				{Prefix: "get", Match: "get$1()", Resolve: "packages.${1:lower}"},
			},
			expr: "getPageTools()",
			want: "packages.pagetools",
		},
		{
			// A pipe-delimited prefix registers the resolver under one bucket
			// per distinct first byte, so an expression containing both bytes
			// collected the same resolver twice and matched it twice. The match
			// is anchored here so it succeeds from the position the first
			// alternative found, which is what makes the duplicate observable.
			name: "pipe-delimited prefix with both alternatives present",
			resolvers: []Resolver{
				{
					Prefix:  "createModel|buildModel",
					Match:   `^createModel\.buildModel\((\w+)\)$`,
					Resolve: "app.models.$1",
				},
			},
			expr: "createModel.buildModel(user)",
			want: "app.models.user",
		},
		{
			// The documented gotcha, pinned so both paths keep agreeing on it:
			// unanchored, findPrefixPos returns the position of whichever
			// alternative it finds first in the order written, and that slice
			// position is what match sees. "createModel" wins the position, so
			// a match written for "buildModel(" never gets a chance.
			name: "shorter alternative found first fixes the slice position",
			resolvers: []Resolver{
				{Prefix: "createModel|buildModel", Match: "buildModel($1)", Resolve: "app.models"},
			},
			expr: "createModel, buildModel(x)",
			want: "",
		},
		{
			name: "no resolver matches",
			resolvers: []Resolver{
				{Prefix: "svc", Match: "svc.$1", Resolve: "app.specific"},
			},
			expr: "unrelated.call()",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viaArray, _ := ResolveFromCallFull(tt.expr, tt.resolvers)
			viaSet := BuildResolverSet(tt.resolvers).Resolve(tt.expr)

			if viaArray != tt.want {
				t.Errorf("ResolveFromCallFull = %q, want %q", viaArray, tt.want)
			}

			if viaSet != tt.want {
				t.Errorf("ResolverSet.Resolve = %q, want %q", viaSet, tt.want)
			}

			if viaArray != viaSet {
				t.Errorf("paths disagree on %q: array=%q set=%q", tt.expr, viaArray, viaSet)
			}
		})
	}
}
