package codemap

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// HTMLOptions configures a generated report.
type HTMLOptions struct {
	// Title names the report.
	Title string

	// HideUtility opens the report with utility code switched off.
	//
	// It sets the toggle's starting position, not the contents: the nodes are
	// still in the page and the counts still include them, so a reader can switch
	// them back on. A report that omitted them would be a different map, and one
	// whose reader cannot tell it was narrowed.
	HideUtility bool
}

//go:embed assets/viewer.html
var viewerHTML string

// d3Bundle is the viewer's only JavaScript dependency, built from the module list
// in assets/vendor/entry.js and committed so that neither this build nor a
// generated report needs a network. See assets/vendor/README.md.
//
//go:embed assets/vendor/d3.bundle.js
var d3Bundle string

// WriteHTML writes a self-contained interactive viewer with the map inlined.
//
// Four views, because no single picture works at every size. Hierarchical edge
// bundling groups the ring by island and then directory, so a detached piece
// occupies its own arc instead of being interleaved with live code. The force view
// gives every island its own centre — without that, the repulsion piles every
// detached island against the border, which is exactly how unreachable code
// becomes invisible. The matrix has no occlusion at any size, so it is the one that
// still works when the others are a hairball. The islands view is the disconnected
// structure on its own terms.
//
// The page is genuinely self-contained: D3 is inlined from a vendored bundle, not
// fetched. A report is a single file you can mail, attach to a ticket or open on a
// machine with no network, and it renders identically years later — a CDN reference
// would make all three depend on someone else's uptime and versioning. The bundle
// carries only the eight D3 modules these views use, 76KB rather than 280KB.
func (m *Map) WriteHTML(w io.Writer, opts HTMLOptions) error {
	title := opts.Title
	if title == "" {
		title = "Code map"
	}

	payload := m.compact(title)
	payload.HideUtility = opts.HideUtility

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding map: %w", err)
	}

	page := strings.ReplaceAll(viewerHTML, "__CODEMAP_TITLE__", escapeHTMLText(title))
	page = strings.ReplaceAll(page, "__D3_BUNDLE__", d3Bundle)
	page = strings.ReplaceAll(page, "__CODEMAP_DATA__", escapeJSONForScript(string(data)))

	_, err = io.WriteString(w, page)

	return err
}

// escapeJSONForScript makes a JSON payload safe inside a <script> element. A "</"
// anywhere in the data — a component path, a file name, a line of source in a label
// — closes the element early and turns the rest of the map into markup. Escaping
// the "<" as a < keeps the JSON identical to a parser and inert to the HTML
// tokenizer.
func escapeJSONForScript(s string) string {
	r := strings.NewReplacer("<", `<`, ">", `>`, "&", `&`, " ", ` `, " ", ` `)

	return r.Replace(s)
}

func escapeHTMLText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

	return r.Replace(s)
}
