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
