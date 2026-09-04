package path

import (
	"strings"

	"go.lsp.dev/uri"
)

// uriScheme is the only scheme this codebase maps onto the filesystem.
const uriScheme = "file://"

// ToURI converts a filesystem path into a document URI fit to send to an LSP
// client. Use it for every URI that crosses the wire: a Location, a
// DocumentLink target, a window/showDocument parameter.
//
// The obvious `"file://" + path` is wrong in ways that only show up off a plain
// POSIX path, because there it happens to produce the right answer:
//
//	/app/Application.cfc      → file:///app/Application.cfc          (same)
//	/My Documents/App.cfc     → file:///My Documents/App.cfc         (space not encoded)
//	C:\Users\q\Application.cfc → file://C:\Users\q\Application.cfc    (two slashes,
//	                                                                  backslashes,
//	                                                                  bare drive colon)
//
// The last one is what a Windows client is handed, and it cannot open it. The
// canonical form is file:///c%3A/Users/q/Application.cfc.
func ToURI(path string) uri.URI {
	return uri.File(path)
}

// ToURIFor is [ToURI] for an explicitly chosen platform rather than the host's.
//
// It exists so the Windows encoding can be tested from a POSIX machine, which
// is the only place this ever gets tested: CI runs Linux, and uri.File keys off
// runtime.GOOS, so on Linux it percent-escapes a Windows path's backslashes
// (%5C) instead of converting them to slashes.
func ToURIFor(platform uri.Platform, path string) uri.URI {
	return uri.FileFor(platform, path)
}

// FromURI converts a document URI received from a client into a filesystem
// path, tolerating both the canonical form and the raw form some clients send.
//
// It replaces `strings.TrimPrefix(u, "file://")`, which is correct only for an
// unescaped POSIX URI:
//
//	file:///app/App.cfc              → /app/App.cfc                (same)
//	file:///My%20Documents/App.cfc   → /My%20Documents/App.cfc      (escape left in)
//	file:///c%3A/Users/q/App.cfc     → /c%3A/Users/q/App.cfc        (not a path at all)
//
// The escapes have to be decoded, but decoding alone is not enough either: a
// client that sends the raw `file://C:\Users\q\App.cfc` parses as a URI whose
// *authority* is the whole path and whose path is empty, so asking the URI for
// its filesystem path returns "". Those clients exist, so the trim is kept as
// the fallback for exactly that case rather than the primary route.
//
// Anything that is not a file URI is returned unchanged, which is what the trim
// did too: a bare path stays a bare path, and an untitled: document keeps its
// scheme instead of being mangled into a path that does not exist.
func FromURI(rawURI string) string {
	if !strings.HasPrefix(rawURI, uriScheme) {
		return rawURI
	}

	if p := uri.URI(rawURI).FsPath(); p != "" {
		return p
	}

	return strings.TrimPrefix(rawURI, uriScheme)
}

// FromURIFor is [FromURI] for an explicitly chosen platform rather than the
// host's, for the same reason as [ToURIFor].
func FromURIFor(platform uri.Platform, rawURI string) string {
	if !strings.HasPrefix(rawURI, uriScheme) {
		return rawURI
	}

	if p := uri.FsPathFor(uri.URI(rawURI), platform, false); p != "" {
		return p
	}

	return strings.TrimPrefix(rawURI, uriScheme)
}
