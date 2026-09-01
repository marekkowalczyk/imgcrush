# imgcrush

[![Go Report Card](https://goreportcard.com/badge/github.com/marekkowalczyk/imgcrush)](https://goreportcard.com/report/github.com/marekkowalczyk/imgcrush)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A quick, non-hacky command-line tool for compressing JPEG and PNG images.
One binary, pure Go (no CGo, no shelling out).

```
$ imgcrush --dry-run *.jpg

imgcrush: metadata (EXIF, ICC, XMP) will be stripped
  dry   photo.jpg  2.4 MB -> 1.1 MB (54.2%)
  dry   hero.jpg   890.3 KB -> 412.7 KB (53.6%)
  skip  thumb.jpg (minimal gain: 3.2%)

  2 compressed, 1 skipped, 0 errors. Saved 1.8 MB.
```

## Product elevator statement

For developers and anyone who shrinks JPEG and PNG images from scripts,
CI, or SSH — people who need compression they can trust in automation —
existing GUI-first tools are excellent at saving bytes but awkward or
fragile as command-line citizens. imgcrush is a single pure-Go binary
that compresses images with safe defaults (backups, dry-run, skip-if-
minimal-gain) and honest Unix behavior (flags, exit codes, stdout/
stderr). You get reliable, scriptable image crushing without installing
a C toolchain or shelling out to a stack of external tools. Unlike
ImageOptim-class wrappers, it is one binary you can drop into any
workflow and reason about — trading peak compression ratio for
simplicity and predictability.

## Why this exists

I needed a command-line image compressor that works inside automated
workflows — CI pipelines, Makefiles, SSH sessions, Claude Code. Something
I could call from a script and trust to behave: flags, exit codes, stderr
for errors, stdout for output.

[ImageOptim](https://imageoptim.com) is excellent at compression, but its
CLI story is a mess: an npm package that launches the GUI via AppleScript
and polls for completion. It breaks across macOS updates, can't run
headless, and doesn't compose with anything.

imgcrush takes a different approach: a single Go binary that does one
thing — shrink images — and tries to be a good Unix citizen about it.

**It is not as good as ImageOptim at compression.** Sometimes by a long
shot. ImageOptim bundles mozjpeg, oxipng, pngquant, and other
purpose-built C/Rust tools that have been optimized for years. imgcrush
uses Go's standard library encoders plus a small pure-Go quantizer for
logo-like PNGs — decent but not state-of-the-art. What you get in return
is simplicity: no C toolchain, no shelling out, no fragile dependency
chain. It just works.

Your mileage may vary. If you need top-of-the-line compression, use
ImageOptim (or mozjpeg/oxipng directly). If you want something quick and
reliable that you can drop into any workflow, here you are.

### Philosophy

This tool is built in the spirit of the Unix tradition of small, sharp,
composable tools — the kind described in Brian P. Hogan's
[Small, Sharp Software Tools](https://pragprog.com/titles/bhcldev/small-sharp-software-tools/)
and Ricardo Gerardi's
[Powerful Command-Line Applications in Go](https://pragprog.com/titles/rggo/powerful-command-line-applications-in-go/).

## Install

Requires Go 1.21 or later.

```bash
go install github.com/marekkowalczyk/imgcrush@latest
```

Or from a local clone:

```bash
git clone https://github.com/marekkowalczyk/imgcrush.git
cd imgcrush
go install .
```

## Usage

```bash
# Compress files in-place (creates .bak backups)
imgcrush photo.jpg logo.png

# Dry run — see what would happen without writing anything
imgcrush --dry-run *.jpg

# Set JPEG quality (default: 85)
imgcrush --quality 75 photo.jpg

# Write compressed files to a separate directory
imgcrush --outdir ./compressed/ photo.jpg logo.png

# Write with a suffix instead of overwriting
imgcrush --suffix .min photo.jpg  # produces photo.min.jpg

# Force compression even if the gain is small
imgcrush --force photo.jpg

# Flags may appear after filenames (shell-loop friendly)
imgcrush photo.jpg --no-backup

# Disable automatic lossy PNG quantization (exact ≤256 palettes still allowed)
imgcrush --no-lossy-png icons/*.png

# Re-check everything ignoring the skip cache
imgcrush --no-cache *.png

# Suppress all output (exit code only)
imgcrush --quiet photo.jpg
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--quality <1-100>` | JPEG quality | 85 |
| `--png-level <0-3>` | PNG compression level | 3 (best) |
| `--png-colors <1-256>` | Max palette size for lossy PNG | 256 |
| `--threshold <0-100>` | Skip if size gain is below this percent | 10 |
| `--outdir <path>` | Write output to a directory | (in-place) |
| `--suffix <string>` | Append suffix to output filenames | (none) |
| `--dry-run` | Report what would happen, don't write | false |
| `--force` | Compress even if gain is below `--threshold` (also bypasses cache read) | false |
| `--lossy-png` | Force palette quantization for PNG | false |
| `--no-lossy-png` | Disable automatic lossy PNG quantization | false |
| `--no-backup` | Skip creating .bak files in in-place mode | false |
| `--no-cache` | Disable incremental skip cache | false |
| `--quiet` | Suppress all output | false |
| `--help`, `-h` | Show help | |
| `--version`, `-v` | Show version | |

Flags may appear before or after filenames. Use `--` before a path that
starts with `-`.

## How it works

imgcrush detects image format from file content (magic bytes, not file
extension), decodes the image, and re-encodes it with compression:

- **JPEG**: Re-encodes at the specified quality level using Go's
  `image/jpeg` encoder. Default quality of 85 is a reasonable
  size-vs-quality sweet spot.
- **PNG (omakase)**: Classifies each image and runs a small tournament:
  - ≤256 unique colors → exact paletted PNG (pixel-lossless remap)
  - 257–2048 unique colors, binary alpha only → try lossy 256-color palette
  - otherwise → truecolor at max compression
  - keeps the smallest candidate that passes the size-gain threshold

### Incremental cache

After a file is crushed (or skipped as already optimal / minimal gain),
imgcrush records settlement markers so later runs can skip work:

- **Inode (Stat-only)** — same file after rename or iCloud eviction skips
  without reading contents
- **On-file xattr** (macOS/Linux, best-effort) — survives when the central
  cache is cleared
- **Content hash** — same bytes at a new path skip encode after one read,
  then the inode/xattr layers make the next visit free

Markers live under the user cache directory
(`~/Library/Caches/imgcrush` on macOS, `~/.cache/imgcrush` on Linux).
Unknown files are always crushed (including iCloud downloads when needed).
Use `--force` to re-process anyway, or `--no-cache` to disable the cache.

### Live per-file output

Multi-file runs print each `ok`/`skip` line as that file finishes (not
only at the end), so long batches do not look hung. On a terminal, a
single updating `imgcrush: N/total` line on stderr shows batch position
between results (no scrolling litany). Suppressed by `--quiet`.

### What you should know

- **Metadata is stripped.** Re-encoding through Go's image pipeline
  discards EXIF, ICC color profiles, and XMP data. This is inherent to
  the approach. Back up originals if you need metadata.
- **JPEG compression is lossy.** Each decode/re-encode cycle loses some
  quality. imgcrush mitigates this: if re-encoding wouldn't save at
  least `--threshold` percent (default 10%), the file is skipped
  (override with `--force`).
- **PNG logos may be quantized.** Omakase can reduce logo-like PNGs to a
  palette. Use `--no-lossy-png` to forbid lossy quantization (exact
  palettes for ≤256-color images still run).
- **Backups by default.** In-place mode creates `.bak` copies before
  overwriting. Use `--no-backup` if you don't want them.

### Tradeoffs vs ImageOptim

| | ImageOptim | imgcrush |
|---|---|---|
| Dependencies | 6+ C/Rust tools, npm, GUI app | Pure Go module; single binary |
| CLI | AppleScript wrapper (fragile) | Native flags, exit codes, stdout |
| Output options | In-place only | In-place, suffix, output directory |
| Runs headless | No | Yes |
| JPEG approach | Lossless optimization (mozjpeg) | Lossy re-encoding (Go stdlib) |
| PNG compression | Excellent (oxipng + pngquant) | Omakase palette + Go stdlib |
| Safety | Overwrites, no dry-run | Backups, dry-run, skip threshold |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All files processed successfully |
| 1 | One or more files failed, or invalid arguments |

## Testing

```bash
go test ./...
```

## License

[MIT](LICENSE)

Created by [Marek Kowalczyk](https://orcid.org/0009-0008-3874-6736).
