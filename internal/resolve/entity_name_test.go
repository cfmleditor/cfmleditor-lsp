package resolve

import "testing"

// TestAnEntityNameIsTheEntityNotTheFileBesideTheCaller: FW/1's qBall writes
// entityNew( "question" ) in services/question.cfc, whose own file a path
// lookup finds; the name is the persistent beans/question.cfc's. With an
// application of each beside the other, each takes its own nearest entity.
func TestAnEntityNameIsTheEntityNotTheFileBesideTheCaller(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{}

	for _, app := range []string{"appA", "appB"} {
		files[app+"/model/beans/question.cfc"] = `component persistent="true" accessors="true" { property name="` + app + `Field"; }`
		files[app+"/model/services/question.cfc"] = `component {
	function post(){
		var q = entityNew( "question" );
		q.set` + app + `Field( 1 );
		q.nope();
		return q;
	}
}`
	}

	writeFiles(t, dir, files)

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "appA/model/services/question.cfc"), map[string]string{
		"q.setappAField": "",
		"q.nope":         "method 'nope' not found in question",
	})
	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "appB/model/services/question.cfc"), map[string]string{
		"q.setappBField": "",
	})
}
