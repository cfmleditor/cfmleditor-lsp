package resolve

import "testing"

// TestAStructOfClosuresAnswersItsMembers: DI/1's declare() returns a struct
// whose members are closures that return it again, so
// declare( "x" ).instanceOf( "y" ).asSingleton() is a chain of members. A name
// the struct does not hold is still reported, and a member returning anything
// else ends what can be checked.
func TestAStructOfClosuresAnswersItsMembers(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"ioc.cfc": `component {
	public any function declare( string beanName ) {
		var declaration = { beanName : beanName, built : false };
		var beanFactory = this;
		structAppend( declaration, {
			instanceOf : function( string dottedPath ) {
				if ( declaration.built ) throw "done";
				declaration.built = true;
				return declaration;
			},
			asSingleton : function() {
				return declaration;
			},
			done : function() {
				return beanFactory;
			}
		} );
		return declaration;
	}

	public any function plain() {
		var s = { a : 1 };
		return s;
	}
}`,
		"Main.cfc": `component {
	function setup() {
		var bf = new ioc();
		bf.declare( "a" ).instanceOf( "x.y" ).asSingleton();
		bf.declare( "b" ).instanceOf( "x.y" ).done().whatever();
		bf.declare( "c" ).notAMember();
		bf.plain().nothing();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Main.cfc"), map[string]string{
		"bf.declare.instanceOf":               "",
		"bf.declare.instanceOf.asSingleton":   "",
		"bf.declare.instanceOf.done.whatever": "",
		"bf.declare.notAMember":               "method 'notAMember' is not a member of the struct 'declare' in ioc returns",
		"bf.plain.nothing":                    "method 'plain' in ioc has no component return type (chain to 'nothing')",
	})
}

// TestAReturnOfABuiltInsValueIsDynamic: TestBox's getPageContextResponse()
// returns getPageContext().getResponse(), or a struct standing in for it, so
// a call on what it returns is a call on what the engine hands back. A
// function with any other kind of return is still reported.
func TestAReturnOfABuiltInsValueIsDynamic(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"Reporter.cfc": `component {
	function getPageContextResponse() {
		if ( !getFunctionList().keyExists( "getPageContext" ) ) {
			return {
				"setContentType" : function() {}
			};
		}
		return getPageContext().getResponse();
	}

	function getHeld() {
		if ( x ) {
			return getPageContext();
		}
		return variables.held;
	}

	private function getResponse() {
		return server.keyExists( "lucee" ) ? getPageContext().getResponse() : getPageContext()
			.getResponse()
			.getResponse();
	}

	private function getEither() {
		return x ? getPageContext() : variables.held;
	}

	function run() {
		getPageContextResponse().setContentType( "text/html" );
		getHeld().anything();
		getResponse().isCommitted();
		getEither().anything();
	}
}`,
	})

	expectReasons(t, reasonsWith(t, &Resolver{}, dir, "Reporter.cfc"), map[string]string{
		"getPageContextResponse.setContentType": "",
		"getHeld.anything":                      "method 'getHeld' has no component return type (chain to 'anything')",
		"getResponse.isCommitted":               "",
		"getEither.anything":                    "method 'getEither' has no component return type (chain to 'anything')",
	})
}
