package resolve

import "testing"

// TestPrcResponseIsColdBoxsResponse: ColdBox keeps the request's Response in
// prc.response, so a handler's prc.response is one — ColdBox's own RestHandler
// included, which extends a bare "EventHandler". A handler that assigns
// prc.response itself, or a component that is not a handler, gets nothing.
func TestPrcResponseIsColdBoxsResponse(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"coldbox/system/EventHandler.cfc":         `component {}`,
		"coldbox/system/web/context/Response.cfc": `component { function setFormat( f ){ return this; } }`,
		"coldbox/system/RestHandler.cfc":          `component extends="EventHandler" { function aroundHandler( event, rc, prc ){ arguments.prc.response.setFormat( "json" ); } }`,
		"handlers/Main.cfc":                       `component extends="coldbox.system.EventHandler" { function index( event, rc, prc ){ prc.response.setFormat( "json" ); prc.response.nope(); } }`,
		"handlers/Own.cfc":                        `component extends="coldbox.system.EventHandler" { function index( event, rc, prc ){ prc.response = {}; prc.response.setFormat( "json" ); } }`,
		"models/Plain.cfc":                        `component { function f( prc ){ prc.response.setFormat( "json" ); } }`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "handlers/Main.cfc"), map[string]string{
		"prc.response.setFormat": "",
		"prc.response.nope":      "method 'nope' not found in Response",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "coldbox/system/RestHandler.cfc"), map[string]string{
		"arguments.prc.response.setFormat": "",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "handlers/Own.cfc"), map[string]string{
		"prc.response.setFormat": "variable 'prc.response' has no component ref",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "models/Plain.cfc"), map[string]string{
		"prc.response.setFormat": "variable 'prc.response' has no component ref",
	})
}
