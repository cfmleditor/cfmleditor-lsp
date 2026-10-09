#!/usr/bin/env bash
# Fetches the CFLint the release bundles beside clif, for one platform, and
# checks it against the checksum pinned here.
#
#   scripts/fetch-cflint.sh <goos> <goarch> <dir>
#
# Writes <dir>/cflint (cflint.exe on Windows). The release workflow packs it
# into clif-with-cflint-<goos>-<goarch>, which every package manager and
# install script installs, so an installed clif lints without downloading
# anything. internal/cflint looks beside the running clif after PATH.
#
# To move to a newer CFLint, change CFLINT_VERSION, fallbackVersion in
# internal/cflint/cflint.go (a test holds the two equal) and the five
# checksums: each
# is the sha256 of that release's .tar.gz (.zip on Windows), which GitHub lists
# as the asset's digest:
#   gh api repos/cfmleditor/CFLint/releases/tags/<version> --jq '.assets[] | "\(.name) \(.digest)"'
set -euo pipefail

CFLINT_VERSION=1.5.17

goos="${1:?usage: fetch-cflint.sh <goos> <goarch> <dir>}"
goarch="${2:?usage: fetch-cflint.sh <goos> <goarch> <dir>}"
dir="${3:?usage: fetch-cflint.sh <goos> <goarch> <dir>}"

case "${goos}/${goarch}" in
darwin/arm64) asset=cflint-macos-aarch64.tar.gz sum=75a8a3d0cdb28b39aa2906145e88925189f8a252ad2edd118bf2ea5582c89092 ;;
darwin/amd64) asset=cflint-macos-amd64.tar.gz sum=2457382fdc0f4b18e3e561f26580fd224a62f0d0649240a05dc3716b56626b2b ;;
linux/arm64) asset=cflint-linux-aarch64.tar.gz sum=1fb7a8f060d87c8dad52854f50b04edc97eb0a1b67821639ff8037ebd40eec3b ;;
linux/amd64) asset=cflint-linux-amd64.tar.gz sum=0813981b8e6f500a61d81b022e45556d0cee679ad0183f39331afe933c9c455c ;;
windows/amd64) asset=cflint-windows-amd64.zip sum=5a55fe060570fde20b370380cfa6de4cd88c3387fafced63b25e1ee3789539e8 ;;
*)
	echo "fetch-cflint: no CFLint build for ${goos}/${goarch}" >&2
	exit 1
	;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL --retry 3 -o "${tmp}/${asset}" \
	"https://github.com/cfmleditor/CFLint/releases/download/${CFLINT_VERSION}/${asset}"

if command -v sha256sum >/dev/null 2>&1; then
	got="$(sha256sum "${tmp}/${asset}" | cut -d' ' -f1)"
else
	got="$(shasum -a 256 "${tmp}/${asset}" | cut -d' ' -f1)"
fi

if [ "${got}" != "${sum}" ]; then
	echo "fetch-cflint: ${asset} ${CFLINT_VERSION} is ${got}, pinned ${sum}" >&2
	exit 1
fi

mkdir -p "${dir}"
case "${asset}" in
*.zip)
	unzip -q -o "${tmp}/${asset}" cflint.exe -d "${dir}"
	;;
*)
	tar -xzf "${tmp}/${asset}" -C "${dir}" cflint
	chmod 755 "${dir}/cflint"
	;;
esac

echo "CFLint ${CFLINT_VERSION} for ${goos}/${goarch} in ${dir}"
