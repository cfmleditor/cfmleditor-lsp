package index

import (
	"testing"

	"go.lsp.dev/uri"
)

// TestDelegatesAreForgottenWithTheExtends: a file's WireBox delegations are
// recorded by IndexFile and dropped by any re-index, as its extends is, so an
// edit that removes a delegation is seen by the next lookup rather than kept
// from the version first indexed.
func TestDelegatesAreForgottenWithTheExtends(t *testing.T) {
	idx := New()
	u := uri.URI("file:///w/Computer.cfc")

	idx.IndexFile(u, `component { property name="memory" inject delegate; }`)

	if ds, ok := idx.DelegatesForFile(u); !ok || len(ds) != 1 || ds[0].Target != "memory" {
		t.Fatalf("after IndexFile: %+v, %v", ds, ok)
	}

	idx.IndexFileFromResult(u, nil, nil)

	if ds, ok := idx.DelegatesForFile(u); ok {
		t.Errorf("after a re-index: %+v still held", ds)
	}

	idx.SetDelegates(u, nil)
	idx.RemoveFile(u)

	if _, ok := idx.DelegatesForFile(u); ok {
		t.Error("after RemoveFile: still held")
	}
}

// TestAFileWithNoFunctionsIsFoundInItsOwnCase: the file-name search
// recovered a file's real path from one of its definitions, and a component
// with none — Lucee's `component extends="…TestCase" {}` — came back
// lowercased, a path a case-sensitive filesystem does not have. The extends
// walk then stopped there, and 80 of Lucee's $assert calls lost their type.
func TestAFileWithNoFunctionsIsFoundInItsOwnCase(t *testing.T) {
	idx := New()
	u := uri.URI("file:///w/Org/LuceeTestCase.cfc")

	idx.IndexFile(u, `component extends="testbox.system.compat.framework.TestCase" {}`)

	if got := idx.FindFilesByBasename("LuceeTestCase"); len(got) != 1 || got[0] != "/w/Org/LuceeTestCase.cfc" {
		t.Errorf("IndexFile: %v", got)
	}

	idx.IndexFileFromResult(u, nil, nil)

	if got := idx.FindFilesByBasename("org/luceetestcase"); len(got) != 1 || got[0] != "/w/Org/LuceeTestCase.cfc" {
		t.Errorf("IndexFileFromResult: %v", got)
	}

	idx.RemoveFile(u)

	if got := idx.FindFilesByBasename("LuceeTestCase"); len(got) != 0 {
		t.Errorf("after RemoveFile: %v", got)
	}
}
