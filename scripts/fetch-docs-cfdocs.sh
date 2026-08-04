#!/usr/bin/env bash
set -euo pipefail

# Fetches cfdocs JSON into its own staging directory. The combined docs/data/
# consumed by generate_docs.go is assembled from all staging dirs by
# scripts/assemble-docs.sh — never write to docs/data directly from here, or
# this script's cache marker and rm -rf would clobber the other source's files.
SRC="docs/src/cfdocs"
REPO="https://github.com/cfmleditor/cfdocs.git"
SHA_FILE="docs/.sha-cfdocs"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

remote_sha=$(git ls-remote "$REPO" refs/heads/master | cut -f1)

if [ -f "$SHA_FILE" ] && [ -d "$SRC" ]; then
    local_sha=$(cat "$SHA_FILE")
    if [ "$remote_sha" = "$local_sha" ]; then
        echo "cfdocs up to date (sha: ${local_sha:0:8})"
        exit 0
    fi
    echo "cfdocs changed, re-downloading..."
fi

echo "Downloading cfdocs data/en..."
git clone --depth 1 --filter=blob:none --sparse "$REPO" "$TMP/cfdocs"
git -C "$TMP/cfdocs" sparse-checkout set data/en

# Only replace the staged copy once the download has succeeded, so a failed
# fetch leaves the previous one intact instead of destroying it.
rm -rf "$SRC"
mkdir -p "$SRC"
cp "$TMP/cfdocs/data/en/"*.json "$SRC/"

mkdir -p "$(dirname "$SHA_FILE")"
echo "$remote_sha" > "$SHA_FILE"

echo "Copied $(find "$SRC" -name '*.json' | wc -l | tr -d ' ') JSON files to $SRC/"
