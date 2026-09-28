package docs

import "strings"

// undocumentedFunctions are functions Lucee defines that neither docs source
// lists, so a call to one reads as a call to a function nobody declared.
// Taken from Lucee's own function library (core/src/main/java/resource/fld/
// core-base.fld): every function there that LookupFunction does not know.
// All but the last two are marked <status>hidden</status>, which is why
// docs.lucee.org leaves them out; Lucee's administrator calls struct() 137
// times. createPageContext and sessionTouch are newer than the docs.
//
// Hand-maintained, not generated: the docs pipeline reads the published
// docs, and these are exactly what it cannot see. Keep it to functions the
// engine defines — a framework's helpers belong in configuration.
var undocumentedFunctions = func() map[string]bool {
	names := []string{
		// hidden
		"_createComponent", "_dump", "_getStaticScope", "_getSuperStaticScope",
		"_internalRequest", "_jsonArray", "_jsonStruct", "_literalArray",
		"_literalOrderedStruct", "_literalStruct", "_valueList",
		"__arrayDuplicate", "__dateTimeDuplicate", "__queryDuplicate",
		"__setDay", "__setHour", "__setMilliSecond", "__setMinute",
		"__setMonth", "__setSecond", "___setYear",
		"compileToBytecode", "directoryEvery", "dumpStruct",
		"evaluateComponent", "evaluateJava", "importJavaSettings",
		"intergralContext", "luceeAIGetNameForDefault", "luceeExtension",
		"luceeStructInfo", "luceeValueRef", "luceeVersionsDetail",
		"luceeVersionsDetailMvn", "luceeVersionsDetailS3", "luceeVersionsList",
		"luceeVersionsListMvn", "luceeVersionsListS3", "restoreToSource",
		"stringHasPrefix", "stringHasSuffix", "stringIsEmpty", "struct",
		"systemExitClean", "systemExitHas", "systemExitScan",
		// newer than the docs
		"createPageContext", "sessionTouch",
	}

	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[strings.ToLower(n)] = true
	}

	return m
}()

// IsBuiltinFunction reports whether name is a function the engine provides:
// a documented one, a tag called as a function, or one of Lucee's
// undocumented functions. The unresolved report and the code map both ask
// it, so the two cannot disagree about what needs no definition.
func IsBuiltinFunction(name string) bool {
	if _, ok := LookupFunction(name); ok {
		return true
	}

	if IsTagFunction(name) {
		return true
	}

	return undocumentedFunctions[strings.ToLower(name)]
}
