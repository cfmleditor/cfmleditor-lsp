component {
	function onMissingMethod(string missingMethodName, struct missingMethodArguments) {
		return invoke(variables.javaObj, missingMethodName, missingMethodArguments);
	}
}
