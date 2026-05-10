#!/usr/bin/env bash
set -euo pipefail

DEST="docs/data"
REPO="https://github.com/cfmleditor/cfdocs.git"
SHA_FILE="docs/.sha-cfdocs"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

remote_sha=$(git ls-remote "$REPO" refs/heads/master | cut -f1)

if [ -f "$SHA_FILE" ] && [ -d "$DEST" ]; then
    local_sha=$(cat "$SHA_FILE")
    if [ "$remote_sha" = "$local_sha" ]; then
        echo "cfdocs up to date (sha: ${local_sha:0:8})"
        exit 0
    fi
    echo "cfdocs changed, re-downloading..."
    rm -rf "$DEST"
fi

echo "Downloading cfdocs data/en..."
git clone --depth 1 --filter=blob:none --sparse "$REPO" "$TMP/cfdocs"
git -C "$TMP/cfdocs" sparse-checkout set data/en

mkdir -p "$DEST"
cp "$TMP/cfdocs/data/en/"*.json "$DEST/"

mkdir -p "$(dirname "$SHA_FILE")"
echo "$remote_sha" > "$SHA_FILE"

echo "Copied $(ls "$DEST"/*.json | wc -l | tr -d ' ') JSON files to $DEST/"
