package parser

import (
	"go.lsp.dev/uri"
	"os"
	"testing"
)

func TestHintDebugReal(t *testing.T) {
	content, err := os.ReadFile("/Users/garethedwards/tassdev/tassweb/packages/tass/orbitstaffapi.cfc")
	if err != nil {
		t.Skip("file not found")
	}

	fileURI := uri.URI("file:///orbitstaffapi.cfc")
	pr := ParseWithOptions(fileURI, string(content), ParseOptions{ExtractCalls: true})

	var docComp string

	for _, r := range pr.ComponentRefs {
		if r.Variable == "document" {
			docComp = r.Component

			break
		}
	}

	t.Logf("document ComponentRef component: %q", docComp)

	// Check func-scoped refs around line 6413 (0-indexed: 6412)
	for _, s := range pr.Scopes {
		if s.Start <= 6412 && s.End >= 6412 {
			t.Logf("scope %d-%d (%s)", s.Start, s.End, s.Name)

			for _, r := range pr.FuncComponentRefs(s.Start, s.End) {
				if r.Variable == "document" {
					t.Logf("  funcscoped document -> %s", r.Component)
				}
			}
		}
	}
}
