package server

import (
	"strings"
	"testing"

	"github.com/cfmleditor/cfmleditor-lsp/internal/config"
	"github.com/cfmleditor/cfmleditor-lsp/internal/route"
	"go.lsp.dev/uri"
)

// routeServer is a server with a routing convention configured, which is what
// makes any of the route work happen at all.
func routeServer(t *testing.T) *Server {
	t.Helper()

	srv := newTestServer()
	srv.WorkspaceFolders = []string{testdataDir()}
	srv.Features = config.ResolveFeatures(nil)
	srv.Routes = route.Config{
		Attributes:  []string{"data-view", "data-read"},
		QueryParams: []string{"do"},
		Functions:   []string{"redirect"},
		// Enabled() needs a source *and* somewhere to resolve to. Without a
		// controller or view template it returns false and every route path
		// below short-circuits — which is a test that passes whatever the code
		// does, the failure mode these tests exist to catch.
		Controllers: []route.ControllerRule{{Component: "models.${1}", Method: "${2}"}},
	}

	if !srv.Routes.Enabled() {
		t.Fatal("route config is not enabled; these tests would pass vacuously")
	}

	return srv
}

// A route lookup at the cursor must cost the same however long the document is.
//
// It did not: the scan covered the whole file and then kept only references on
// the cursor's line, so every other line was work thrown away and the file size
// decided how much. On a 64,000-line component that was three seconds for every
// go-to-definition, and the answer was usually nothing — which is why no test
// that checked the answer ever noticed.
//
// The same cursor, in the same first few lines, with a growing tail after it.
func TestRouteLookupDoesNotScaleWithDocumentSize(t *testing.T) {
	srv := routeServer(t)

	// The head is comfortably longer than the window on both sides of the
	// cursor, so both documents present a full window and the only difference
	// between them is content the window should never reach. A short head would
	// compare a small window against a full one and fail for that reason
	// instead, which is what the first version of this test did.
	pad := strings.Repeat("<cfset pad = \"a & b? c\">\n", 4*routeWindow)
	cursorLine := 4 * routeWindow
	head := pad + `<a data-view="admin.report.view">report</a>` + "\n" + pad
	tail := strings.Repeat("<cfset y = \"a & b? c\">\n", 20000)

	cost := func(content string) int64 {
		r := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				srv.routeAtPosition(content, cursorLine, 16)
			}
		})

		return r.NsPerOp()
	}

	srv.routeAtPosition(head, 1, 16) // warm the memoised resolver

	small, large := cost(head), cost(head+tail)

	t.Logf("document grew by %d bytes: %dns -> %dns (%.1fx)", len(tail), small, large, float64(large)/float64(small))

	// The window is a fixed number of lines, so the tail should not be read at
	// all. Doubling allows for the cost of finding where the window ends.
	if float64(large) > float64(small)*2 {
		t.Errorf("appending %d bytes after the cursor took the lookup from %dns to %dns (%.1fx); "+
			"the scan is reading the whole document rather than the cursor's neighbourhood",
			len(tail), small, large, float64(large)/float64(small))
	}
}

// Document links need every reference in the file, so they cannot use the
// cursor window — they must not repeat the scan instead.
//
// An editor asks for links again on a document it has not changed: on open, on
// focus, after anything that refreshes them. Without the memo that was a full
// scan every time.
func TestDocumentLinkScanIsReusedForUnchangedContent(t *testing.T) {
	srv := routeServer(t)
	content := `<a data-view="admin.report.view">x</a>` + "\n" + strings.Repeat("<cfset z = 1>\n", 500)

	first := srv.routeLinks(content)
	key := srv.routeScanKey

	if key == "" {
		t.Fatal("expected the scan to be memoised after the first call")
	}

	srv.routeLinks(content)

	if srv.routeScanKey != key {
		t.Error("unchanged content rescanned; the memo is keyed on something that is not the content")
	}

	// Changed content must not serve the old answer.
	srv.routeLinks(content + `<a data-read="admin.thing.read">y</a>`)

	if srv.routeScanKey == key {
		t.Error("changed content served the previous scan")
	}

	_ = first
}

// The switch has to stop the work, not merely the answer.
func TestRoutesFeatureSwitchStopsTheScan(t *testing.T) {
	off := false
	srv := routeServer(t)
	srv.Features = config.ResolveFeatures(&config.Features{Routes: &off})

	if r := srv.routeResolver(); r != nil {
		t.Fatal("with features.routes off, expected no route resolver")
	}

	content := `<a data-view="admin.report.view">x</a>`
	if links := srv.routeLinks(content); links != nil {
		t.Errorf("expected no route links with the feature off, got %d", len(links))
	}

	if _, ok := srv.routeAtPosition(content, 0, 16); ok {
		t.Error("expected no route at position with the feature off")
	}

	docURI := uri.URI("file://" + testdataDir() + "/FeatureOff.cfm")
	srv.setDocument(docURI, content)

	if got := definitionAt(t, srv, docURI, 0, 16); got != nil {
		t.Errorf("expected no definition through routes with the feature off, got %v", got)
	}
}
