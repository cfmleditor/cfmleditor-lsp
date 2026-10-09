//go:build !cflint_embed

package cflint

// embeddedCFLint is empty in a build without -tags cflint_embed; see
// embedded.go. A var, so a test can give it an archive.
var embeddedCFLint []byte
