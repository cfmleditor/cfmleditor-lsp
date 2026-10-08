#!/bin/sh
# Install one clif release into a directory you own: no root, no package
# manager, the same binary on every machine that pins the same version.
#
#   scripts/install.sh [version] [dir]
#
# version   0.5.0, v0.5.0 or latest. Left out, the first of clif.version and
#           scripts/clif.version in the working directory names it, so a
#           project, its developers and its CI install one version from one
#           file; with neither, latest.
# dir       where clif goes: $CLIF_INSTALL_DIR, else ~/.local/bin.
#
# CLIF_DOWNLOAD_URL replaces https://github.com/cfmleditor/clif/releases, for
# a mirror laid out the same way (<url>/download/v<version>/<asset>, and
# <url>/latest/download/<asset>).
#
# The archive is checked against the release's checksums.txt when the release
# has one. Releases from before the rename to clif publish cfmleditor-lsp
# archives only; one of those is installed as clif.
set -eu

version=${1:-}
dir=${2:-${CLIF_INSTALL_DIR:-$HOME/.local/bin}}
releases=${CLIF_DOWNLOAD_URL:-https://github.com/cfmleditor/clif/releases}

if [ -z "$version" ]; then
	for f in clif.version scripts/clif.version; do
		if [ -f "$f" ]; then
			version=$(tr -d ' \t\r\n' <"$f")
			break
		fi
	done
fi
version=${version:-latest}

case "$(uname -s)" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*)
	echo "clif install: $(uname -s) is not supported by this script; on Windows use scripts/install.ps1" >&2
	exit 1
	;;
esac

case "$(uname -m)" in
arm64 | aarch64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*)
	echo "clif install: no release for $(uname -m)" >&2
	exit 1
	;;
esac

if [ "$version" = latest ]; then
	base=$releases/latest/download
else
	base=$releases/download/v${version#v}
fi

fetch() { # url file
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL -o "$2" "$1"
	else
		wget -q -O "$2" "$1"
	fi
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	else
		shasum -a 256 "$1" | cut -d' ' -f1
	fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

name=clif
if ! fetch "$base/clif-$os-$arch.tar.gz" "$tmp/archive.tar.gz" 2>/dev/null; then
	name=cfmleditor-lsp
	fetch "$base/cfmleditor-lsp-$os-$arch.tar.gz" "$tmp/archive.tar.gz" || {
		echo "clif install: no $os/$arch archive for $version at $base" >&2
		exit 1
	}
fi

asset=$name-$os-$arch.tar.gz
if fetch "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
	want=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1 }' "$tmp/checksums.txt")
	got=$(sha256 "$tmp/archive.tar.gz")
	if [ -z "$want" ] || [ "$want" != "$got" ]; then
		echo "clif install: $asset does not match the release's checksums.txt (want ${want:-nothing}, got $got)" >&2
		exit 1
	fi
else
	echo "clif install: $version has no checksums.txt, so $asset was not verified" >&2
fi

tar -xzf "$tmp/archive.tar.gz" -C "$tmp" "$name"
mkdir -p "$dir"
mv "$tmp/$name" "$dir/clif"
chmod 755 "$dir/clif"

echo "Installed $("$dir/clif" version) to $dir/clif"
case ":$PATH:" in
*":$dir:"*) ;;
*) echo "clif install: $dir is not on PATH; add it to use clif by name" >&2 ;;
esac
