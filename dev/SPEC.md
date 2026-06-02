# imgcrush — Feature Spec

## Vision

imgcrush is a free, clean, pure-Go drop-in replacement for ImageOptim.

### What we're replacing and why

ImageOptim on macOS is a Rube Goldberg machine:
- The "CLI" (`imageoptim-cli`) is an npm package that launches the GUI app
  via AppleScript, feeds files through osascript, and polls for completion.
  It's fragile and breaks across macOS updates.
- Under the hood, the GUI shells out to 6+ separate C tools (mozjpeg,
  jpegtran, jpegoptim, pngquant, oxipng, zopfli, advpng), runs them all
  in parallel, and picks the smallest result.
- It overwrites in-place with no output directory option.
- No real Unix integration: no stdout, no exit codes you can trust, no
  composability with pipes or scripts.

### What imgcrush is

- **One binary.** `go build` and you're done. No brew dependencies, no
  npm, no GUI, no AppleScript, no C toolchain.
- **Pure Go.** No shelling out to external tools — ever. That's the core
  principle. If pure Go can't do it yet, we wait or we write it ourselves.
- **A proper Unix citizen.** Flags, exit codes, stderr for errors, stdout
  for output, composable with other tools. Works in scripts, CI, Makefiles.
- **Safe by default.** Backups, dry-run, skip-if-minimal-gain. You should
  be able to run it casually on real files without fear.
- **Honest about tradeoffs.** Pure Go JPEG encoding is 15-30% less
  efficient than mozjpeg. We say so. We improve over time. We don't hide
  behind wrappers.

## MVP

### Core behavior

- Accept one or more file paths as arguments: `imgcrush photo.jpg logo.png`
- Detect format (JPEG/PNG) from file content, not just extension
- Compress and write output

### Output modes (pick one via flag)

- **In-place** (default): overwrite the original file
- **Suffix**: write to `photo.crushed.jpg` (flag: `--suffix <string>`)
- **Output directory**: write to a specified dir, preserving filenames
  (flag: `--outdir <path>`)

### Compression controls

- `--quality <1-100>` — JPEG quality (default: 85, the "visually lossless"
  sweet spot for Go's encoder — see dev/RESEARCH.md)
- `--png-level <0-3>` — PNG compression level mapped to Go's
  `png.DefaultCompression`, `png.BestSpeed`, `png.BestCompression`,
  `png.NoCompression` (default: best compression)

### Safe-by-default behavior

The MVP must be safe to run casually on real files without surprises:

- **Skip well-compressed files.** If re-encoding would not reduce file size
  by at least 10%, skip the file and report it as "skipped (minimal gain)".
  Override with `--force`.
- **Never silently degrade.** Running imgcrush twice on the same JPEG must
  not keep degrading quality. The skip-if-minimal-gain rule handles this:
  after the first compression, subsequent runs see <10% gain and skip.
- **Backup before overwrite.** In in-place mode, create a `.bak` copy of
  the original before writing. Flag `--no-backup` to skip.
- **Refuse non-image files.** Exit with an error, don't silently produce
  garbage.
- **`--dry-run` flag.** Report what would happen without writing anything.

### Metadata transparency

Re-encoding through Go's stdlib strips all metadata (EXIF, ICC profiles,
XMP). This is inherent to the decode/re-encode pipeline — `image.Image`
carries only pixel data. The tool must:
- State this clearly in `--help` output
- Print a one-line warning when processing files (suppressible with `--quiet`)

### Output and feedback

- Print per-file summary: filename, original size, new size, % reduction
- Print total summary at the end: files processed, total saved, files skipped
- `--quiet` flag to suppress all output (exit code only)

### Standard flags

- `--help` / `-h`
- `--version` / `-v`

### Exit codes

- 0: success (all files compressed or skipped)
- 1: error (bad args, unreadable files, write failures)

---

## Backlog (post-MVP)

Improvements that close the gap with ImageOptim, all staying pure Go.
Prioritized by benchmark findings (see dev/RESEARCH.md section 4).

### Priority 1: Close the PNG gap (biggest real gap from benchmark)

Benchmark showed ImageOptim gets 80% on few-color PNGs where imgcrush
gets 23%. The gap is palette optimization + better filter strategies.

- **Color quantization** — lossy PNG mode via colorquant/octreequant
  (pngquant-like, `--lossy-png` flag). Highest-value single improvement.
- **PNG filter optimization** — try all 5 filter types per row, pick best
- **klauspost/compress deflate** — swap stdlib deflate for smaller output

### Priority 2: Honest framing of JPEG compression

Benchmark revealed imgcrush JPEG savings come from lossy re-encoding,
not smarter compression. ImageOptim (without JPEGmini) does lossless
JPEG optimization — different operation entirely. imgcrush currently
has nothing for lossless JPEG.

- **Lossless JPEG Huffman optimization** — no pure-Go solution exists
  today. Research needed: is a pure-Go jpegtran-style optimizer feasible?
- **Progressive JPEG encoding** — Go stdlib only does baseline. Progressive
  is typically 5-10% smaller for large images.

### Better Unix citizenship
- **Glob patterns**: `imgcrush "*.jpg"`
- **Directory mode**: `imgcrush ./photos/` processes all images in a directory
- **Recursive walking**: `--recursive` / `-r` flag for nested directories
- **Stdin/stdout**: pipe support for single-image workflows
- **JSON output**: `--json` for scripting and automation

### More features
- **Config file**: `imgcrush.conf` for default quality, output mode, etc.
- **Parallel processing**: compress multiple files concurrently (`--jobs <n>`)
- **Resize**: `--max-width`, `--max-height`, `--scale` flags
- **WebP output**: `--format webp` to convert and compress to WebP
- **Metadata preservation (phase 1)**: `--keep-metadata` to preserve
  EXIF, ICC profiles, XMP, and IPTC data through compression. Uses raw
  byte-level segment/chunk splicing — no third-party library needed.
  Copy metadata segments as opaque blobs from original into output.
  Some EXIF fields (compression tags, Software, DateTime) will be stale
  but non-breaking. See dev/RESEARCH.md section 5 for full design rationale.
- **Metadata preservation (phase 2)**: Parse the EXIF APP1 segment to
  drop invalid tags (compression-structural, thumbnail IFD, MakerNote,
  ImageUniqueID) and update others (Software, DateTime). Requires a
  minimal EXIF parser/writer. See dev/RESEARCH.md section 5 field analysis.
- **Granular metadata stripping**: `--strip-gps`, `--strip-exif`,
  `--strip-private`, `--keep-icc` — per-field control for privacy and
  selective stripping. Requires parsing individual EXIF/XMP fields.
  See dev/RESEARCH.md section 5 privacy analysis.
- **Preserve timestamps**: `--keep-mtime` to preserve modification time
- **Progress bar**: for large batches
