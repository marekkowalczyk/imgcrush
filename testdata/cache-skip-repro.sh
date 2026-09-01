#!/bin/bash
#
# cache-skip-repro.sh — time warm-cache skips on the screenshot batch.
#
# Reproduces the "cached skip is slow" scenario (#1): crush once to populate
# the cache, then time a second pass that should be almost all `cached` skips.
#
# Usage:
#   ./testdata/cache-skip-repro.sh
#
# Prerequisites:
#   - go run testdata/gen_test_images.go   # creates testdata/png/cache-skip/
#   - go build -o imgcrush .

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
IMGCRUSH="$PROJECT_DIR/imgcrush"
BATCH="$SCRIPT_DIR/png/cache-skip"
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

if [ ! -x "$IMGCRUSH" ]; then
	echo "error: build first: go build -o imgcrush ." >&2
	exit 1
fi

shopt -s nullglob
files=("$BATCH"/screenshot-*.png)
if [ ${#files[@]} -eq 0 ]; then
	echo "error: no batch images. Run: go run testdata/gen_test_images.go" >&2
	exit 1
fi

cp "${files[@]}" "$WORKDIR/"
n=${#files[@]}
echo "Batch: $n PNGs from $BATCH"
echo "Work:  $WORKDIR"
echo

echo "=== Pass 1 (cold: crush + fill cache) ==="
/usr/bin/time -p "$IMGCRUSH" --no-backup "$WORKDIR"/*.png
echo

echo "=== Pass 2 (warm: expect cached skips) ==="
/usr/bin/time -p "$IMGCRUSH" --no-backup "$WORKDIR"/*.png
echo
echo "Pass 2 should be near-instant if cache hits are cheap; multi-second"
echo "per-file skips reproduce https://github.com/marekkowalczyk/imgcrush/issues/1"
