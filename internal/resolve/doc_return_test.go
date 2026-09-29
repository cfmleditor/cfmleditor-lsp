package resolve

import "testing"

// TestADocumentedReturnCountsWhereItNamesAFile: ColdBox declares few return
// types and documents most — RequestService's getContext() says `@return
// coldbox.system.web.context.RequestContext` — so with ColdBox checked out
// every chain through one stopped at "no component return type". A
// documented path that names no file is left alone, as if the doc said
// nothing, rather than reported as a missing component; and a declared type
// always outranks the doc. One naming an interface types the call as the
// interface, which declares less than the object behind it: getCache() is
// documented as an IColdBoxProvider, and callers use getOrSet(), which every
// provider has. A method the interface lacks is dynamic, not missing.
func TestADocumentedReturnCountsWhereItNamesAFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"lib/RequestContext.cfc": `component { function getValue( name ) {} }`,
		"lib/Other.cfc":          `component { function other() {} }`,
		"lib/IProvider.cfc":      "interface {\n\tfunction get( key );\n}",
		"lib/Service.cfc": `component {
	/**
	 * The current request.
	 *
	 * @return lib.RequestContext
	 */
	function getContext() {}

	/**
	 * @return lib.gone.Nowhere
	 */
	function wrongDoc() {}

	/**
	 * @return lib.RequestContext
	 */
	lib.Other function declared() {}

	/**
	 * @return lib.IProvider
	 */
	function getCache() {}
}`,
		"Page.cfc": `component {
	function f() {
		var s = new lib.Service();
		s.getContext().getValue( "x" );
		s.getContext().notAMethod();
		s.wrongDoc().anything();
		s.declared().other();
		s.getCache().getOrSet( "k" );
		s.getCache().get( "k" );
		s.getCache().getOrSet( "k" ).anything();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Page.cfc"), map[string]string{
		"s.getContext.getValue":        "",
		"s.getContext.notAMethod":      "method 'notAMethod' not found in RequestContext",
		"s.wrongDoc.anything":          "method 'wrongDoc' in lib.Service has no component return type (chain to 'anything')",
		"s.declared.other":             "",
		"s.getCache.getOrSet":          "",
		"s.getCache.get":               "",
		"s.getCache.getOrSet.anything": "",
	})
}
