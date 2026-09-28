package language

/*
#include <stddef.h>
#include <stdlib.h>

// Declared here rather than taken from tree_sitter/api.h, whose include path
// belongs to go-tree-sitter. Both symbols are defined in go-tree-sitter's C
// objects, which are linked into the same binary.
extern void ts_set_allocator(
	void *(*new_malloc)(size_t),
	void *(*new_calloc)(size_t, size_t),
	void *(*new_realloc)(void *, size_t),
	void (*new_free)(void *));

extern void (*ts_current_free)(void *);

static void use_native_allocator(void) {
	ts_set_allocator(NULL, NULL, NULL, NULL);
}

static int native_allocator(void) {
	return ts_current_free == free;
}
*/
import "C"

// go-tree-sitter's init installs an allocator that sends every malloc, calloc,
// realloc and free tree-sitter makes back into Go through a cgo callback, so a
// caller can substitute its own. Nothing here does, and the callback is not
// free: a parse allocates a node at a time, and on a folding request the
// callbacks were 12% of the whole request. Passing NULL restores tree-sitter's
// own defaults, which call libc directly.
//
// Both allocators are libc underneath, so memory allocated before this runs
// (the grammars' languages, say) is freed correctly after it. This package's
// init runs after go-tree-sitter's, since it imports it, so the reset is the
// one that stays.
func init() {
	C.use_native_allocator()
}

// nativeAllocator reports whether tree-sitter is calling libc directly, for
// the test that pins it; a test file cannot use cgo itself.
func nativeAllocator() bool {
	return C.native_allocator() != 0
}
