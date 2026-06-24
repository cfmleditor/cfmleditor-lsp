package parser_test

import (
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/docs"
	"github.com/cfmleditor/cfmleditor-lsp/internal/parser"
)

func TestGetCloudServiceBuiltin(t *testing.T) {
	src := `<cfcomponent>
<cffunction name="test">
<cfset s3Cred = {} />
<cfset s3Conf = { "serviceName": "S3" } />
<cfset s3Service = getCloudService(s3Cred, s3Conf) />
<cfset s3Bucket = s3Service.root("mybucket") />
<cfset s3Result = s3Bucket.downloadObject({ "key": "test" }) />
</cffunction>
</cfcomponent>`

	pr := parser.ParseWithOptions("file:///test.cfc", src, parser.ParseOptions{
		BuiltinReturnLookup: docs.LookupBuiltinReturnComponent,
		ExtractCalls:        true,
	})

	// s3Service should get a global $builtin ref
	found := false

	for _, ref := range pr.ComponentRefs {
		if ref.Variable == "s3Service" && ref.Component == "$builtin.getcloudservice" {
			found = true
		}
	}

	if !found {
		t.Error("expected global component ref for s3Service -> $builtin.getcloudservice")
	}

	// s3Bucket and s3Result should get function-scoped refs propagated from s3Service
	for _, scope := range pr.Scopes {
		refs, _ := pr.FuncRefs(scope.Start, scope.End)
		for _, ref := range refs {
			if ref.Variable == "s3Bucket" && ref.Component == "$builtin.getcloudservice" {
				return // success
			}
		}
	}

	t.Error("expected function-scoped component ref for s3Bucket (propagated from s3Service)")
}
