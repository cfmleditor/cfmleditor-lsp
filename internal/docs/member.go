package docs

import "strings"

// MemberFunction represents a function callable via dot notation on a type.
type MemberFunction struct {
	Name  string // e.g. "append"
	Entry *Entry // the underlying built-in entry (e.g. arrayAppend)
}

// typePrefixes maps CFML type names to their function name prefixes.
var typePrefixes = []string{
	"array", "string", "struct", "query", "list", "image", "spreadsheet",
}

var memberFuncs map[string][]MemberFunction // type -> member functions

func init() {
	buildMemberFuncs()
}

func buildMemberFuncs() {
	memberFuncs = make(map[string][]MemberFunction)

	for i := range entries {
		e := &entries[i]
		if e.Type != "function" {
			continue
		}

		lower := strings.ToLower(e.Name)
		for _, prefix := range typePrefixes {
			if strings.HasPrefix(lower, prefix) && len(lower) > len(prefix) {
				memberName := strings.ToLower(e.Name[len(prefix):len(prefix)+1]) + e.Name[len(prefix)+1:]
				memberFuncs[prefix] = append(memberFuncs[prefix], MemberFunction{
					Name:  memberName,
					Entry: e,
				})

				break
			}
		}
	}
}

// AllMemberFunctions returns member functions for all types (used when type is unknown).
func AllMemberFunctions() []MemberFunction {
	var out []MemberFunction
	for _, mfs := range memberFuncs {
		out = append(out, mfs...)
	}

	return out
}

// MemberFunctionsForType returns member functions for a specific type.
func MemberFunctionsForType(typeName string) []MemberFunction {
	return memberFuncs[strings.ToLower(typeName)]
}
