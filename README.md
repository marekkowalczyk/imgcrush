# imgcrush

A free, clean, pure-Go drop-in replacement for [ImageOptim](https://imageoptim.com).

One binary. No dependencies. No GUI. No AppleScript. No C toolchain.
A proper Unix tool for compressing JPEG and PNG images.

## Why

ImageOptim is great at compression, but its architecture is a mess:

- The "CLI" is an npm package that launches the GUI via AppleScript
  and polls for completion. It breaks across macOS updates.
- Under the hood, the GUI shells out to 6+ separate C/Rust tools
  (mozjpeg, jpegtran, pngquant, oxipng, zopfli, and more).
- No output directory option. No reliable exit codes. No stdout.
  No way to use it in scripts, CI, or over SSH.

imgcrush takes a different approach:

- **Pure Go.** No shelling out to external tools -- ever. Single
  binary, built with `go build`.
- **A proper Unix citizen.** Flags, exit codes, stderr for errors,
  composable with pipes and scripts.
- **Safe by default.** Backups before overwrite, dry-run mode,
  skips files that are already well-compressed.
- **Honest about tradeoffs.** Pure Go encoding is less efficient
  than mozjpeg or pngquant. We say so and improve over time.

## Status

**Work in progress.** The MVP is under active development.

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

# Dry run -- see what would happen without writing anything
imgcrush --dry-run *.jpg

# Set JPEG quality (default: 85)
imgcrush --quality 75 photo.jpg

# Write compressed files to a separate directory
imgcrush --outdir ./compressed/ photo.jpg logo.png

# Write with a suffix instead of overwriting
imgcrush --suffix .min photo.jpg  # produces photo.min.jpg

# Force compression even if the gain is small
imgcrush --force photo.jpg

# Suppress all output (exit code only)
imgcrush --quiet photo.jpg
```

## Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--quality <1-100>` | JPEG quality | 85 |
| `--png-level <0-3>` | PNG compression level | 3 (best) |
| `--outdir <path>` | Write output to a directory | (in-place) |
| `--suffix <string>` | Append suffix to output filenames | (none) |
| `--dry-run` | Report what would happen, don't write | false |
| `--force` | Compress even if gain is below 10% | false |
| `--no-backup` | Skip creating .bak files in-place mode | false |
| `--quiet` | Suppress all output | false |
| `--help`, `-h` | Show help | |
| `--version`, `-v` | Show version | |

## How it works

imgcrush detects image format from file content (magic bytes, not
file extension), decodes the image, and re-encodes it with
compression:

- **JPEG**: Re-encodes at the specified quality level using Go's
  `image/jpeg` encoder. Default quality of 85 is the "visually
  lossless" sweet spot.
- **PNG**: Re-encodes with Go's `image/png` at maximum compression.

### What you should know

- **Metadata is stripped.** Re-encoding through Go's image pipeline
  discards EXIF, ICC color profiles, and XMP data. This is inherent
  to the approach, not a bug. Back up originals if you need metadata.
- **JPEG compression is lossy.** Each decode/re-encode cycle loses
  some quality. imgcrush mitigates this: if re-encoding wouldn't save
  at least 10%, the file is skipped (override with `--force`).
- **Backups by default.** In-place mode creates `.bak` copies before
  overwriting. Use `--no-backup` if you don't want them.

### Tradeoffs vs ImageOptim

imgcrush prioritizes simplicity and correctness over raw compression
ratio:

| | ImageOptim | imgcrush |
|---|---|---|
| Dependencies | 6+ C/Rust tools, npm, GUI app | None (single binary) |
| CLI | AppleScript wrapper (fragile) | Native flags, exit codes, stdout |
| Output options | In-place only | In-place, suffix, output directory |
| Runs headless | No | Yes |
| JPEG compression | Excellent (mozjpeg) | Good (Go stdlib, ~15-30% larger) |
| PNG compression | Excellent (oxipng + zopfli) | Decent (Go stdlib, improving) |
| Safety | Overwrites, no dry-run | Backups, dry-run, skip threshold |

## Roadmap

The backlog focuses on closing the compression gap and improving
Unix integration, all in pure Go:

- Better PNG compression via `klauspost/compress` and filter optimization
- Lossy PNG mode (color quantization, pngquant-style)
- Directory and recursive mode
- Parallel processing
- Stdin/stdout pipe support
- EXIF/ICC preservation
- WebP output

See [SPEC.md](SPEC.md) for the full feature spec.

## License

[MIT](LICENSE)
