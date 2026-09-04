<cfscript>
	var myTestVariable;
	//this is a comment

	function helloWorld() {

		lSParseCurrency("$1.03");
		myNumber = isNumeric(server.date);
		return myNumber;

	}

	function testData(
		myData
	) {

		var lsp = arguments.myData && " - worked!";
		return lsp;

	}
</cfscript>
