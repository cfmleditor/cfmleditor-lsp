#!/usr/bin/env bash
set -euo pipefail

# Fetches Lucee JSON into its own staging directory. See the note in
# fetch-docs-cfdocs.sh — docs/data/ is assembled from all staging dirs by
# scripts/assemble-docs.sh, never written directly from here.
SRC="docs/src/lucee"
ETAG_FILE="docs/.etag"
URL="https://docs.lucee.org/lucee-docs.zip"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# An unreachable host or a response without an ETag header must not kill the
# script via set -e/pipefail on this pipeline — fall through to the download,
# which reports a real error if the host is genuinely unavailable.
remote_etag=$(curl -sI "$URL" 2>/dev/null | grep -i "^etag:" | tr -d '\r' | awk '{print $2}' || true)

if [ -n "$remote_etag" ] && [ -f "$ETAG_FILE" ] && [ -d "$SRC" ]; then
    local_etag=$(cat "$ETAG_FILE")
    if [ "$remote_etag" = "$local_etag" ]; then
        echo "Lucee docs up to date (etag: $local_etag)"
        exit 0
    fi
    echo "Lucee docs changed, re-downloading..."
fi

echo "Downloading lucee-docs.zip..."
if ! curl -fsSL "$URL" -o "$TMP/lucee-docs.zip"; then
    echo "error: could not download $URL" >&2
    exit 1
fi

echo "Extracting..."
unzip -qo "$TMP/lucee-docs.zip" "lucee-docs-json-zipped.zip" -d "$TMP"
unzip -qo "$TMP/lucee-docs-json-zipped.zip" "lucee-docs-json.zip" -d "$TMP"
unzip -qo "$TMP/lucee-docs-json.zip" "*.json" -d "$TMP/staged"

# Only replace the staged copy once the download and extraction have both
# succeeded, so a failed fetch leaves the previous one intact.
rm -rf "$SRC"
mkdir -p "$(dirname "$SRC")"
mv "$TMP/staged" "$SRC"

mkdir -p "$(dirname "$ETAG_FILE")"
echo "$remote_etag" > "$ETAG_FILE"

echo "Extracted $(find "$SRC" -name '*.json' | wc -l | tr -d ' ') JSON files to $SRC/"
