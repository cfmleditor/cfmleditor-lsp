#!/usr/bin/env bash
set -euo pipefail

# Assembles the combined docs/data/ consumed by scripts/generate_docs.go from
# the per-source staging directories populated by the fetch-docs-*.sh scripts.
#
# Sources are applied in order and later ones win on filename collision. Lucee
# is applied last, preserving the historical behaviour of unzipping Lucee over
# an already-populated cfdocs directory.
#
# docs/data/ is rebuilt from scratch each time so a file removed upstream does
# not linger, and so a partially-wiped state can never survive into a build.
#
# Note: plain strings rather than arrays for the "missing" list — expanding an
# empty array under `set -u` is an error on bash < 4.4, i.e. macOS's stock 3.2.

DEST="docs/data"
SOURCES="docs/src/cfdocs docs/src/lucee"

missing=""
found=0

for src in $SOURCES; do
    if [ -d "$src" ]; then
        found=$((found + 1))
    else
        missing="$missing $src"
    fi
done

if [ "$found" -eq 0 ]; then
    echo "error: no doc sources staged —$missing" >&2
    echo "       run \`make docs\` to fetch them" >&2
    exit 1
fi

if [ -n "$missing" ]; then
    echo "warning: doc source(s) not staged, generated docs will be incomplete:$missing" >&2
fi

rm -rf "$DEST"
mkdir -p "$DEST"

for src in $SOURCES; do
    [ -d "$src" ] || continue

    # generate_docs.go globs docs/data/*.json, so only top-level files count.
    count=$(find "$src" -maxdepth 1 -name '*.json' -exec cp {} "$DEST/" \; -print | wc -l | tr -d ' ')
    echo "  $src → $count JSON files"
done

echo "Assembled $(find "$DEST" -maxdepth 1 -name '*.json' | wc -l | tr -d ' ') JSON files in $DEST/"
