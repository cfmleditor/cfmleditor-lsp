package server

import (
	"reflect"
	"strings"
	"testing"
)

// A tree-sitter tree is C memory the Go collector does not account for, so a
// tree that outlives the call that made it is a leak the runtime cannot see and
// a heap profile will not show. Every one this server makes — formatting,
// folding, scanning — is a local closed before its function returns, and this
// asserts that the Server never grows somewhere to keep one.
//
// It is worth pinning because caching them is the obvious next optimisation and
// looks free: folding re-parses a script component's whole body on every
// request, and holding the tree between requests would remove that. It is not
// free, it is a deliberate trade of untracked native memory in a long-lived
// daemon shared by every editor session, and it should be a decision rather
// than something that arrives inside a performance patch.
//
// Reachability, not just direct fields: a map, slice, array, pointer or nested
// struct that can hold a tree fails too, since that is the shape a cache would
// actually take.
func TestServerHoldsNoTreeSitterTree(t *testing.T) {
	const treeType = "tree_sitter.Tree"

	seen := map[reflect.Type]bool{}

	var walk func(rt reflect.Type, path string)

	walk = func(rt reflect.Type, path string) {
		if rt == nil || seen[rt] {
			return
		}

		seen[rt] = true

		if strings.HasSuffix(rt.String(), treeType) {
			t.Errorf("the Server can hold a tree-sitter tree at %s (%s) — trees are C memory the collector does not track, and must not outlive the call that made them", path, rt)

			return
		}

		switch rt.Kind() { //nolint:exhaustive // only the kinds that can carry another type
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
			walk(rt.Elem(), path+"[]")
		case reflect.Map:
			walk(rt.Key(), path+"{key}")
			walk(rt.Elem(), path+"{}")
		case reflect.Struct:
			for i := range rt.NumField() {
				f := rt.Field(i)
				walk(f.Type, path+"."+f.Name)
			}
		case reflect.Func, reflect.Interface:
			// A ParseFunc returns a tree and is meant to: the formatter closes
			// what it is handed. Only storage is the concern here, and an
			// interface's dynamic type is not knowable statically.
		}
	}

	walk(reflect.TypeOf(Server{}), "Server")
}
