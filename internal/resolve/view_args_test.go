package resolve

import "testing"

// TestAViewsArgsAreWhatItsRendersPass: ContentBox renders its table partials
// with `view( view : "_components/content/TableCreationInfo", args : {
// content : content } )` from each listing, and its admin bar from an
// interceptor. args.X in the view is what every render passing X gives it,
// typed where the render is made; a render in another module, or one passing
// no X, says nothing, and one passing args other than as a literal leaves it
// untyped.
func TestAViewsArgsAreWhatItsRendersPass(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"model/Content.cfc": `component { function getTitle(){} }`,
		"model/Author.cfc":  `component { function getBio(){} }`,
		"model/Svc.cfc":     `component { model.Content function get( id ){} }`,
		"admin/handlers/content.cfc": `component {
	property name="svc" inject="model.Svc";
	function init(){ variables.svc = new model.Svc(); }
	function index( event, rc, prc ){
		return view( view = "_components/info", args = { content : variables.svc.get( 1 ), showAll : true } );
	}
	function bare( event, rc, prc ){
		return view( view = "_components/info" );
	}
	function elsewhere( event, rc, prc ){
		return view( view = "_components/info", module = "other", args = { content : new model.Author() } );
	}
}`,
		"admin/interceptors/Bar.cfc": `component {
	function postRender( event ){
		var author = new model.Author();
		return view( view = "_components/info", args = { author : author ?: javacast( "null", "" ), content : new model.Content() } );
	}
}`,
		"admin/views/list.cfm":                 `#view( view : "_components/info", args : { showAll : false } )#`,
		"admin/views/_components/info.cfm":     `<cfoutput>#args.content.getTitle()# #args.content.getBio()# #args.author.getBio()#</cfoutput>`,
		"admin/views/_components/computed.cfm": `<cfoutput>#args.content.getTitle()#</cfoutput>`,
		"admin/handlers/computed.cfc": `component {
	function show( event ){ var a = { content : 1 }; return view( view = "_components/computed", args = a ); }
}`,
	})

	expectReasons(t, reasonsIn(t, dir, "admin/views/_components/info.cfm"), map[string]string{
		"args.content.getTitle": "",
		"args.content.getBio":   "method 'getBio' not found in Content",
		"args.author.getBio":    "",
	})
	expectReasons(t, reasonsIn(t, dir, "admin/views/_components/computed.cfm"), map[string]string{
		"args.content.getTitle": "variable 'args.content' has no component ref",
	})
}
