//go:build cflint_embed

package cflint

import _ "embed" // for the CFLint a release build carries

// embeddedCFLint is CFLint fallbackVersion's release asset for this platform,
// as published: a .tar.gz, or a .zip on Windows. The release workflow writes it
// with scripts/fetch-cflint.sh, which checks it against a pinned checksum, and
// builds with -tags cflint_embed. A plain go build leaves it out (embed_off.go)
// and downloads CFLint on first use instead.
//
//go:embed embedded/cflint.archive
var embeddedCFLint []byte
