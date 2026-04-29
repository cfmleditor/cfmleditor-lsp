#!/usr/bin/env bash
set -euo pipefail

DEST="docs/data"
ETAG_FILE="docs/.etag"
URL="https://docs.lucee.org/lucee-docs.zip"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

remote_etag=$(curl -sI "$URL" | grep -i "^etag:" | tr -d '\r' | awk '{print $2}')

if [ -f "$ETAG_FILE" ] && [ -d "$DEST" ]; then
    local_etag=$(cat "$ETAG_FILE")
    if [ "$remote_etag" = "$local_etag" ]; then
        echo "Lucee docs up to date (etag: $local_etag)"
        exit 0
    fi
    echo "Lucee docs changed, re-downloading..."
    rm -rf "$DEST"
fi

echo "Downloading lucee-docs.zip..."
curl -sL "$URL" -o "$TMP/lucee-docs.zip"

echo "Extracting..."
unzip -qo "$TMP/lucee-docs.zip" "lucee-docs-json-zipped.zip" -d "$TMP"
unzip -qo "$TMP/lucee-docs-json-zipped.zip" "lucee-docs-json.zip" -d "$TMP"

mkdir -p "$DEST"
unzip -qo "$TMP/lucee-docs-json.zip" "*.json" -d "$DEST"

mkdir -p "$(dirname "$ETAG_FILE")"
echo "$remote_etag" > "$ETAG_FILE"

echo "Extracted $(ls "$DEST"/*.json | wc -l | tr -d ' ') JSON files to $DEST/"
