package parser

import (
	"strings"
	"testing"
)

func TestReviewMemberSnapshotOwnedAndReused(t *testing.T) {
	source := `component {
 variables.bucket.field=new pkg.Shared();
 function first(rc){rc.service=new pkg.Service();}
 function second(rc){rc.other=new pkg.Other();}
 }`
	pr := ParseWithOptions(testURI, source, &ParseOptions{Shallow: true})
	pr.funcRefsMap = nil
	backing := make([]ComponentRef, len(pr.ComponentRefs)+1)
	copy(backing, pr.ComponentRefs)
	backing[len(backing)-1] = ComponentRef{Variable: "canary"}
	pr.ComponentRefs = backing[:len(backing)-1]

	var snapshot *ParseResult

	for _, scope := range pr.Scopes {
		refs, _ := pr.FuncRefs(scope.Start, scope.End)
		if len(refs) == 0 {
			t.Fatal("member refs lost")
		}

		if snapshot == nil {
			snapshot = pr.memberSnapshot
		} else if snapshot != pr.memberSnapshot {
			t.Fatal("whole-file member snapshot recomputed per function")
		}
	}

	if snapshot == nil {
		t.Fatal("member snapshot not built")
	}

	if backing[len(backing)-1].Variable != "canary" {
		t.Fatal("member analysis wrote into shared slice capacity")
	}

	pr.ApplyFullReplace(strings.ReplaceAll(source, "pkg.Service", "pkg.Replacement"))
	scope := pr.Scopes[0]
	refs, _ := pr.FuncRefs(scope.Start, scope.End)
	found := false

	for _, ref := range refs {
		if ref.Variable == "rc.service" && ref.Component == "pkg.Replacement" {
			found = true
		}
	}

	if !found {
		t.Fatalf("stale member snapshot: %+v", refs)
	}
}

func TestReviewManagedMetadataReadsOnlyDeclarations(t *testing.T) {
	for _, content := range []string{`component accessors=true {function setService(service){variables.service=arguments.service;} property name="service" inject=false;}`, `<cfcomponent accessors="true"><cffunction name="setService"><cfargument name="service"><cfset variables.service=arguments.service></cffunction><cfproperty name="service" inject="false"></cfcomponent>`} {
		pr := ParseWithOptions(testURI, content, &ParseOptions{SetterLookup: func(string) string { return "pkg.Service" }, PropertyBeanLookup: func(string, map[string]string) string { return "" }})

		metadata := pr.managedPropertyMetadata()
		if !metadata.Accessors || len(metadata.Properties) != 1 || len(metadata.Funcs) != 0 || len(metadata.ComponentRefs) != 0 {
			t.Fatalf("metadata includes body parse: %+v", metadata)
		}

		for _, fn := range pr.Funcs {
			if strings.EqualFold(fn.Name, "setService") && len(fn.Arguments) > 0 && argumentComponentType(&fn.Arguments[0]) != "" {
				t.Fatal("late ignored property permitted setter inference")
			}
		}
	}
}

func BenchmarkReviewManagedParse(b *testing.B) {
	content := `component accessors=true {property name="service" inject=false;` + strings.Repeat("function work(required any value){var x=new pkg.Service();return x;}\n", 300) + `}`
	opts := &ParseOptions{Shallow: true, SetterLookup: func(string) string { return "pkg.Service" }, PropertyBeanLookup: func(string, map[string]string) string { return "" }}

	b.ReportAllocs()

	for b.Loop() {
		ParseWithOptions(testURI, content, opts)
	}
}
