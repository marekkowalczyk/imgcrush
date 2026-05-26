#!/bin/bash
#
# benchmark.sh — compare imgcrush vs imageoptim on test files
#
# Runs both tools on copies of the test images and reports a side-by-side
# comparison. ImageOptim results serve as the gold standard/benchmark,
# with caveats:
#   - ImageOptim shells out to 6+ C/Rust tools (mozjpeg, oxipng, etc.)
#   - With -j flag, it uses JPEGmini Pro (commercial, $100+)
#   - It requires the GUI app installed and is macOS-only
#
# Usage:
#   ./testdata/benchmark.sh              # imageoptim without JPEGmini
#   ./testdata/benchmark.sh -j           # imageoptim with JPEGmini Pro
#
# Prerequisites:
#   - imgcrush binary built (go build -o imgcrush .)
#   - imageoptim-cli installed (brew install imageoptim-cli)
#   - ImageOptim.app installed

set -euo pipefail

# --- Helpers ---

fmt_size() {
    local b=$1
    if [ "$b" -lt 1024 ]; then
        echo "${b} B"
    elif [ "$b" -lt 1048576 ]; then
        echo "$(echo "scale=1; $b / 1024" | bc) KB"
    else
        echo "$(echo "scale=1; $b / 1048576" | bc) MB"
    fi
}

pct_saved() {
    local orig=$1 new=$2
    if [ "$orig" -eq 0 ]; then
        echo "0.0"
    else
        echo "scale=1; ($orig - $new) * 100 / $orig" | bc
    fi
}

# --- Setup ---

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
IMGCRUSH="$PROJECT_DIR/imgcrush"
JPEGMINI_FLAG="${1:-}"

if [ ! -x "$IMGCRUSH" ]; then
    echo "error: imgcrush binary not found. Run: go build -o imgcrush ." >&2
    exit 1
fi

if ! command -v imageoptim >/dev/null 2>&1; then
    echo "error: imageoptim-cli not found. Run: brew install imageoptim-cli" >&2
    exit 1
fi

# Collect test files (JPEG and PNG only)
TEST_FILES=$(find "$SCRIPT_DIR/jpeg" "$SCRIPT_DIR/png" -type f \( -name '*.jpg' -o -name '*.png' \) | sort)

if [ -z "$TEST_FILES" ]; then
    echo "error: no test images found" >&2
    exit 1
fi

# Create temp dirs for each tool
DIR_CRUSH=$(mktemp -d)
DIR_OPTIM=$(mktemp -d)
trap 'rm -rf "$DIR_CRUSH" "$DIR_OPTIM"' EXIT

# Copy test files and record original sizes to a temp file
SIZES_FILE=$(mktemp)
trap 'rm -rf "$DIR_CRUSH" "$DIR_OPTIM" "$SIZES_FILE"' EXIT

NAMES=""
echo "$TEST_FILES" | while IFS= read -r f; do
    name=$(basename "$f")
    cp "$f" "$DIR_CRUSH/$name"
    cp "$f" "$DIR_OPTIM/$name"
    echo "$name $(stat -f%z "$f")" >> "$SIZES_FILE"
done

# Build names list from sizes file
NAMES=$(awk '{print $1}' "$SIZES_FILE")

# --- Run imgcrush ---

echo "Running imgcrush (q=85, png-level=3)..."
"$IMGCRUSH" --no-backup --force "$DIR_CRUSH"/* 2>&1 | grep -v "^imgcrush:" || true
echo

# --- Run imageoptim ---

OPTIM_ARGS=""
if [ "$JPEGMINI_FLAG" = "-j" ]; then
    OPTIM_ARGS="--jpegmini"
    echo "Running imageoptim (with JPEGmini Pro)..."
else
    echo "Running imageoptim (lossless, no JPEGmini)..."
fi
(cd "$DIR_OPTIM" && imageoptim $OPTIM_ARGS '**/*.jpg' '**/*.png' '*.jpg' '*.png') 2>&1 || true
echo

# --- Compare results ---

SEP="-----------------------------------------------------------------------------------------"
printf "\n%s\n" "$SEP"
printf "%-30s %10s %12s %8s %12s %8s\n" \
    "FILE" "ORIGINAL" "IMGCRUSH" "SAVED" "IMGOPTIM" "SAVED"
printf "%s\n" "$SEP"

TOTAL_ORIG=0
TOTAL_CRUSH=0
TOTAL_OPTIM=0

echo "$NAMES" | while IFS= read -r name; do
    [ -z "$name" ] && continue

    orig=$(grep "^$name " "$SIZES_FILE" | awk '{print $2}')

    crush=$(stat -f%z "$DIR_CRUSH/$name" 2>/dev/null || echo "$orig")
    optim=$(stat -f%z "$DIR_OPTIM/$name" 2>/dev/null || echo "$orig")

    crush_pct=$(pct_saved "$orig" "$crush")
    optim_pct=$(pct_saved "$orig" "$optim")

    printf "%-30s %10s %12s %7s%% %12s %7s%%\n" \
        "$name" \
        "$(fmt_size "$orig")" \
        "$(fmt_size "$crush")" \
        "$crush_pct" \
        "$(fmt_size "$optim")" \
        "$optim_pct"

    # Accumulate totals in a file (subshell workaround)
    echo "$orig $crush $optim" >> "${SIZES_FILE}.totals"
done

# Sum totals
if [ -f "${SIZES_FILE}.totals" ]; then
    TOTAL_ORIG=$(awk '{s+=$1} END{print s}' "${SIZES_FILE}.totals")
    TOTAL_CRUSH=$(awk '{s+=$2} END{print s}' "${SIZES_FILE}.totals")
    TOTAL_OPTIM=$(awk '{s+=$3} END{print s}' "${SIZES_FILE}.totals")
    rm -f "${SIZES_FILE}.totals"
fi

printf "%s\n" "$SEP"

total_crush_pct=$(pct_saved "$TOTAL_ORIG" "$TOTAL_CRUSH")
total_optim_pct=$(pct_saved "$TOTAL_ORIG" "$TOTAL_OPTIM")

printf "%-30s %10s %12s %7s%% %12s %7s%%\n" \
    "TOTAL" \
    "$(fmt_size $TOTAL_ORIG)" \
    "$(fmt_size $TOTAL_CRUSH)" \
    "$total_crush_pct" \
    "$(fmt_size $TOTAL_OPTIM)" \
    "$total_optim_pct"

echo
echo "imgcrush: pure Go, stdlib encoder, q=85, --force"
if [ "$JPEGMINI_FLAG" = "-j" ]; then
    echo "imageoptim: mozjpeg + oxipng + zopfli + JPEGmini Pro (commercial)"
else
    echo "imageoptim: mozjpeg + oxipng + zopfli (lossless JPEG, no JPEGmini)"
fi
echo
echo "Date: $(date +%Y-%m-%d)"
echo "imgcrush version: $("$IMGCRUSH" -v 2>&1)"
echo "imageoptim version: $(imageoptim -V 2>&1 || echo 'unknown')"
