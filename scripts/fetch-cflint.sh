#!/usr/bin/env bash
# Fetches the CFLint a release build of clif embeds, for one platform, and
# checks it against the checksum pinned here.
#
#   scripts/fetch-cflint.sh <goos> <goarch> [<file>]
#
# Exits 3 for a platform CFLint publishes no build for. `make build` builds
# without CFLint after any failure, with a warning; the release workflow fails.
#
# Writes CFLint's release asset as published (a .tar.gz, or a .zip on Windows)
# to <file>, by default internal/cflint/embedded/cflint.archive, where a build
# with -tags cflint_embed embeds it (internal/cflint/embedded.go). clif unpacks
# it into its cache on first use, so a released clif lints with no download.
#
# To move to a newer CFLint, change CFLINT_VERSION, fallbackVersion in
# internal/cflint/cflint.go (a test holds the two equal) and the five
# checksums. Each is the sha256 of that release's asset, which GitHub lists as
# its digest:
#   gh api repos/cfmleditor/CFLint/releases/tags/<version> --jq '.assets[] | "\(.name) \(.digest)"'
set -euo pipefail

CFLINT_VERSION=1.5.17

goos="${1:?usage: fetch-cflint.sh <goos> <goarch> [<file>]}"
goarch="${2:?usage: fetch-cflint.sh <goos> <goarch> [<file>]}"
out="${3:-$(dirname "$0")/../internal/cflint/embedded/cflint.archive}"

case "${goos}/${goarch}" in
darwin/arm64) asset=cflint-macos-aarch64.tar.gz sum=75a8a3d0cdb28b39aa2906145e88925189f8a252ad2edd118bf2ea5582c89092 ;;
darwin/amd64) asset=cflint-macos-amd64.tar.gz sum=2457382fdc0f4b18e3e561f26580fd224a62f0d0649240a05dc3716b56626b2b ;;
linux/arm64) asset=cflint-linux-aarch64.tar.gz sum=1fb7a8f060d87c8dad52854f50b04edc97eb0a1b67821639ff8037ebd40eec3b ;;
linux/amd64) asset=cflint-linux-amd64.tar.gz sum=0813981b8e6f500a61d81b022e45556d0cee679ad0183f39331afe933c9c455c ;;
windows/amd64) asset=cflint-windows-amd64.zip sum=5a55fe060570fde20b370380cfa6de4cd88c3387fafced63b25e1ee3789539e8 ;;
*)
	echo "fetch-cflint: no CFLint build for ${goos}/${goarch}" >&2
	exit 3
	;;
esac

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

# Already there and the pinned one: nothing to download.
if [ -f "${out}" ] && [ "$(sha256 "${out}")" = "${sum}" ]; then
	echo "CFLint ${CFLINT_VERSION} for ${goos}/${goarch} already in ${out}"
	exit 0
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL --retry 3 -o "${tmp}/${asset}" \
	"https://github.com/cfmleditor/CFLint/releases/download/${CFLINT_VERSION}/${asset}"

got="$(sha256 "${tmp}/${asset}")"

if [ "${got}" != "${sum}" ]; then
	echo "fetch-cflint: ${asset} ${CFLINT_VERSION} is ${got}, pinned ${sum}" >&2
	exit 1
fi

mkdir -p "$(dirname "${out}")"
mv "${tmp}/${asset}" "${out}"

echo "CFLint ${CFLINT_VERSION} for ${goos}/${goarch} in ${out}"
