package cfml

import "strings"

type Tag struct {
	Name   string
	Detail string
	Doc    string
}

type Attribute struct {
	Name   string
	Detail string
}

type Function struct {
	Name      string
	Detail    string
	Signature string
	Doc       string
}

func Tags() []Tag {
	return []Tag{
		{"cfoutput", "Displays output, evaluating expressions within ## signs", "Displays output of variables, expressions, and results of CFML processing.\n\nExpressions are enclosed in `##` signs. Can also loop over query results."},
		{"cfquery", "Executes a SQL query against a datasource", "Executes SQL statements against a datasource.\n\nReturns a query result set that can be used with `cfoutput` or `cfloop`."},
		{"cfset", "Sets a variable value", "Sets a variable to a value.\n\n```cfml\n<cfset myVar = \"hello\">\n```"},
		{"cfif", "Conditional processing", "Evaluates a boolean expression and conditionally executes the enclosed block.\n\nUse with `cfelseif` and `cfelse` for branching."},
		{"cfloop", "Loops over a collection, list, query, or range", "Iterates over a query, list, array, structure, or numeric range.\n\nSupports `query`, `list`, `from/to/step`, `collection`, and `condition` loop types."},
		{"cfinclude", "Includes a CFML template", "Includes and processes another CFML template at the current location.\n\n```cfml\n<cfinclude template=\"header.cfm\">\n```"},
		{"cffunction", "Defines a function", "Defines a CFML function within a component or template.\n\nUse `cfargument` to define parameters and `cfreturn` to return a value."},
		{"cfargument", "Defines a function argument", "Declares a parameter for a `cffunction`.\n\nSpecify `name`, `type`, `required`, and `default` attributes."},
		{"cfreturn", "Returns a value from a function", "Returns a value from the enclosing `cffunction`.\n\n```cfml\n<cfreturn myResult>\n```"},
		{"cfcomponent", "Defines a CFC component", "Defines a ColdFusion Component (CFC).\n\nComponents encapsulate functions and properties, and support inheritance via `extends`."},
	}
}

func TagAttributes() map[string][]Attribute {
	return map[string][]Attribute{
		"cfquery": {
			{"name", "Name for the query result set"},
			{"datasource", "Name of the datasource"},
			{"maxrows", "Maximum number of rows to return"},
			{"cachedwithin", "Timespan for cached query"},
			{"result", "Name of variable for query result metadata"},
		},
		"cfoutput": {
			{"query", "Name of the query to loop over"},
			{"group", "Column to group output by"},
			{"groupcasesensitive", "Whether grouping is case-sensitive"},
			{"startrow", "First row to display"},
			{"maxrows", "Maximum number of rows to display"},
			{"encodefor", "Encoding type for output"},
		},
		"cfloop": {
			{"query", "Query to loop over"},
			{"list", "List to loop over"},
			{"index", "Variable name for current index"},
			{"item", "Variable name for current item"},
			{"from", "Start value for index loop"},
			{"to", "End value for index loop"},
			{"step", "Step value for index loop"},
			{"collection", "Structure to loop over"},
			{"condition", "Condition for while loop"},
			{"delimiters", "Delimiter characters for list loop"},
		},
		"cfinclude": {
			{"template", "Path to the template to include"},
		},
		"cffunction": {
			{"name", "Function name"},
			{"returntype", "Return type of the function"},
			{"access", "Access level (public, private, remote, package)"},
			{"output", "Whether the function can generate output"},
			{"hint", "Description of the function"},
		},
		"cfargument": {
			{"name", "Argument name"},
			{"type", "Data type of the argument"},
			{"required", "Whether the argument is required"},
			{"default", "Default value"},
			{"hint", "Description of the argument"},
		},
		"cfcomponent": {
			{"extends", "Component to extend"},
			{"implements", "Interfaces to implement"},
			{"output", "Whether the component can generate output"},
			{"hint", "Description of the component"},
			{"accessors", "Whether to generate accessor methods"},
		},
		"cfset":    {},
		"cfif":     {},
		"cfreturn": {},
	}
}

func Functions() []Function {
	return []Function{
		{"ArrayAppend", "Appends an element to an array", "ArrayAppend(array, value)", "Appends a value to the end of an array. Returns `true` on success.\n\n```cfml\nArrayAppend(myArray, \"newItem\")\n```"},
		{"ArrayLen", "Returns the length of an array", "ArrayLen(array)", "Returns the number of elements in an array.\n\n```cfml\nlen = ArrayLen(myArray)\n```"},
		{"Len", "Returns the length of a string or structure", "Len(value)", "Returns the number of characters in a string, or the number of keys in a structure.\n\n```cfml\nLen(\"hello\") // returns 5\n```"},
		{"Trim", "Removes leading and trailing whitespace", "Trim(string)", "Removes leading and trailing spaces and control characters from a string.\n\n```cfml\nTrim(\"  hello  \") // returns \"hello\"\n```"},
		{"LCase", "Converts a string to lowercase", "LCase(string)", "Converts all characters in a string to lowercase.\n\n```cfml\nLCase(\"HELLO\") // returns \"hello\"\n```"},
		{"UCase", "Converts a string to uppercase", "UCase(string)", "Converts all characters in a string to uppercase.\n\n```cfml\nUCase(\"hello\") // returns \"HELLO\"\n```"},
		{"Find", "Finds the first occurrence of a substring", "Find(substring, string)", "Returns the position of the first occurrence of a substring. Case-sensitive. Returns `0` if not found.\n\n```cfml\nFind(\"lo\", \"hello\") // returns 4\n```"},
		{"Replace", "Replaces occurrences of a substring", "Replace(string, sub1, sub2)", "Replaces the first occurrence of `sub1` with `sub2`. Use `ReplaceNoCase` for case-insensitive.\n\n```cfml\nReplace(\"hello\", \"l\", \"r\") // returns \"herlo\"\n```"},
		{"StructNew", "Creates a new structure", "StructNew()", "Creates and returns a new, empty structure.\n\n```cfml\nmyStruct = StructNew()\n```"},
		{"QueryExecute", "Executes a SQL query", "QueryExecute(sql, params, options)", "Executes a SQL query with optional parameterized values and query options.\n\n```cfml\nresult = QueryExecute(\n  \"SELECT * FROM users WHERE id = :id\",\n  {id: 1},\n  {datasource: \"myDS\"}\n)\n```"},
	}
}

var tagMap map[string]Tag
var funcMap map[string]Function

func init() {
	tagMap = make(map[string]Tag)
	for _, t := range Tags() {
		tagMap[strings.ToLower(t.Name)] = t
	}
	funcMap = make(map[string]Function)
	for _, f := range Functions() {
		funcMap[strings.ToLower(f.Name)] = f
	}
}

func LookupTag(name string) (Tag, bool) {
	t, ok := tagMap[strings.ToLower(name)]
	return t, ok
}

func LookupFunction(name string) (Function, bool) {
	f, ok := funcMap[strings.ToLower(name)]
	return f, ok
}
