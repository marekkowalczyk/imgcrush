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
- `--png-colors <1-256>` — max palette size for lossy PNG (default: 256)
- `--threshold <0-100>` — skip if size gain is below this percent (default: 10)
- `--lossy-png` / `--no-lossy-png` — force or forbid automatic lossy PNG
  quantization (mutually exclusive)
- `--no-cache` — disable the incremental skip cache
- `--no-backup` — skip `.bak` creation in in-place mode

### PNG omakase (default)

For each PNG, imgcrush classifies the image and tournaments candidates:

- ≤256 unique RGBA colors → exact paletted encode (pixel-lossless remap)
- 257–2048 unique colors and binary alpha only → try lossy 256-color palette
  (`octreequant`); skip auto-lossy when any pixel has partial alpha
- otherwise → truecolor only
- always also encode truecolor; keep the smallest candidate that passes
  never-grow + `--threshold`

`--no-lossy-png` disables the 257–2048 quantization path but still allows
exact palettes. `--lossy-png` forces a palette attempt even when the
classifier would skip.

### CLI argument order

Flags may appear before or after filenames. A bare `--` ends flag parsing;
following tokens are filenames even if they start with `-`.

### Incremental skip cache

After a successful write, or a skip for already-optimal / minimal-gain,
imgcrush records a cascade of markers so later runs can skip work without
re-reading file contents when possible:

1. **L0 (inode)** — `dev|ino|size|mtime` + settings (Stat only). Survives
   rename on the same volume; works on iCloud placeholders without download.
2. **L0b (xattr)** — on-file settlement fingerprint (Darwin/Linux,
   best-effort). Readable without materializing content.
3. **L2 (content)** — SHA-256 of file bytes + settings. Checked after a
   deliberate read; covers copies and cross-volume moves. On hit, L0/xattr
   are backfilled so the next visit is Stat-only.

Markers live under the user cache dir (`os.UserCacheDir()/imgcrush`).
Unknown files always proceed to crush (may download from iCloud).
`--force` bypasses cache reads; `--no-cache` disables read and write.
Dry-run does not write cache entries.

### Safe-by-default behavior

The MVP must be safe to run casually on real files without surprises:

- **Skip well-compressed files.** If re-encoding would not reduce file size
  by at least `--threshold` percent (default 10%), skip the file and report
  it as "skipped (minimal gain)". Override with `--force`.
- **Never silently degrade.** Running imgcrush twice on the same JPEG must
  not keep degrading quality. The skip-if-minimal-gain rule handles this:
  after the first compression, subsequent runs see below-threshold gain and skip.
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

- Print each per-file summary as that file finishes (live, completion order)
- On a TTY, show a single updating `imgcrush: N/total` progress line on stderr
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

### Priority 1: Remaining PNG improvements

Palette omakase (exact + lossy tournament) shipped in v1.2.0. Still open:

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
