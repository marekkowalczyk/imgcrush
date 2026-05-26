# imgcrush — Technical Research

## 0. ImageOptim: What We're Replacing

### Overview

ImageOptim (v1.9.3) is a free, open-source macOS GUI app by Kornel Lesiński.
9.8k GitHub stars. Written in Objective-C + C + HTML. GPL-2.0 licensed.
Requires macOS 11+. The repo has 197 open issues and 1,162 commits.

### Architecture: a bundle of external C tools

ImageOptim is fundamentally a GUI wrapper that orchestrates multiple
standalone command-line tools. It runs several of them on each file and
keeps the smallest result:

| Tool | Format | Type | What it does |
|------|--------|------|--------------|
| **MozJPEG** | JPEG | Lossy/lossless | Mozilla's improved JPEG encoder, trellis quantization, optimized Huffman |
| **jpegtran** | JPEG | Lossless | Lossless JPEG optimization (Huffman tables, progressive conversion) |
| **jpegoptim** | JPEG | Lossless/lossy | Strip metadata, optimize Huffman, optional quality limit |
| **Guetzli** | JPEG | Lossy | Google's perceptual JPEG encoder (slow, high quality) |
| **OxiPNG** | PNG | Lossless | Rust-based PNG optimizer (filter optimization, bit-depth reduction) |
| **Zopfli** | PNG | Lossless | Google's exhaustive deflate compressor (slow, best lossless) |
| **AdvPNG** | PNG | Lossless | Recompress PNG with better deflate |
| **PNGCrush** | PNG | Lossless | Brute-force filter/compression strategy search |
| **PNGOUT** | PNG | Lossless | Extremely aggressive PNG lossless compression |
| **pngquant** | PNG | Lossy | Color quantization to 256-color palette (via ImageAlpha) |
| **Gifsicle** | GIF | Lossless | GIF optimizer |
| **SVGO** | SVG | Lossless | SVG markup optimizer |
| **svgcleaner** | SVG | Lossless | SVG cleanup tool |

Building ImageOptim from source requires Xcode AND Rust (via rustup) because
OxiPNG is written in Rust.

### GUI behavior

- Drag-and-drop or "+" button to add files
- **Overwrites originals in-place** — no output directory option
- Moves originals to Trash (recoverable) before overwriting
- Status icons: gray spinner (processing), green checkmark (saved),
  green X (already optimal)
- Preferences: toggle metadata stripping, toggle lossy mode, but it's
  largely all-or-nothing — no granular quality control per format
- Strips EXIF, GPS, color profiles by default; preservable in preferences

### CLI story: three approaches, all problematic

**1. `open -a ImageOptim .` (non-blocking)**
Launches GUI, asynchronously optimizes files. Returns immediately — no way
to know when it's done. Adds to running instance if already open.

**2. `/Applications/ImageOptim.app/Contents/MacOS/ImageOptim *.png` (blocking)**
Launches hidden GUI, processes synchronously, blocks until done. BUT: "you
must not specify any flags (arguments must not start with -)". File/directory
args only. No configuration from the command line. Each call spawns a
separate instance.

**3. `imageoptim-cli` npm package (v3.1.9, Nov 2023)**
By Jamie Mason. 3.5k stars. Written in TypeScript + AppleScript. Distributed
as a self-contained binary (no Node.js required since v3). Automates
ImageOptim, ImageAlpha, and JPEGmini by simulating GUI interactions via
AppleScript — clicking buttons, sending keystrokes. Supports glob patterns,
quality settings for ImageAlpha, and stats reporting.

Limitations:
- Requires the GUI apps to be installed separately
- JPEGmini needs macOS accessibility/GUI scripting permissions enabled
- AppleScript automation is fragile and breaks across macOS versions
- No output directory, no skip-unchanged, no stdin/stdout
- Intensive CPU; fans run at full power
- No real exit codes for per-file success/failure
- Cannot be used in headless environments (CI servers, SSH sessions)

### What ImageOptim gets right

- Excellent compression: runs multiple tools and picks the best result
- Typical PNG lossless: 20-70% reduction; with pngquant lossy: 60-80%+
- Typical JPEG lossless: 10-30%; with MozJPEG lossy (q~80): 40-70%
- Free and open source
- Simple drag-and-drop for non-technical users

### What ImageOptim gets wrong (and imgcrush must fix)

- **No real CLI.** The three approaches above are all workarounds.
- **No output directory.** Overwrites in-place, period.
- **No Unix composability.** No stdout, no reliable exit codes, no piping.
- **No per-file control.** Quality, metadata, format — all set globally.
- **Fragile automation.** AppleScript breaks across macOS versions.
- **Can't run headless.** Useless in CI, SSH, Docker, scripts.
- **Opaque.** No per-file reporting of what was saved and why.

### jpegoptim (also installed on this system, v1.4.6)

Separate from ImageOptim. Standalone C tool for JPEG optimization.
Installed via Homebrew (2020). Notable features that are good Unix design
and worth learning from:

```
jpegoptim [options] <filenames>
  -d <path>       output directory (default: overwrite originals)
  -m <quality>    max quality (disables lossless mode)
  -n              dry run (--noaction)
  -T <threshold>  skip if gain below threshold %
  -f              force optimization
  -p              preserve timestamps
  -P              preserve permissions
  -q              quiet mode
  -t              print totals
  -s              strip all markers
  --strip-exif    strip EXIF only
  --strip-icc     strip ICC only
  --stdout        output to stdout
  --stdin         read from stdin
  --all-progressive   force progressive output
```

jpegoptim is a good model for imgcrush's CLI design: clear flags, threshold
support, dry-run, output directory, granular metadata control, stdin/stdout.

---

## 1. Initial Library Survey (Grok research)

Go's standard library provides basic compression via `image/jpeg` (quality
parameter, 1-100) and `image/png` (`BestCompression` level). JPEG quality
reduction (80-90) gives the biggest wins; PNG stdlib compression is limited.

### Libraries with C/external dependencies

| Library | Best For | Backend |
|---------|----------|---------|
| **h2non/bimg** | PNG + JPEG + WebP (best overall) | libvips (C) |
| **aprimadi/imagecomp** | PNG + JPEG | pngquant + mozjpeg |
| **yusukebe/go-pngquant** | PNG only | pngquant wrapper |

### Pure Go

| Library | Notes |
|---------|-------|
| **disintegration/imaging** | General image processing, popular, easy to use |

**Decision:** We chose the pure-Go path (no C dependencies) for easy
distribution as a single binary. The sections below detail what that means
in practice.

---

## 1. Image Format Detection

**Use Go stdlib.** `image.DecodeConfig` reads only the header, returns a format
string (`"jpeg"` or `"png"`), and requires zero dependencies:

```go
import (
    "image"
    _ "image/jpeg"
    _ "image/png"
)

f, _ := os.Open(path)
_, format, err := image.DecodeConfig(f)
// format is "jpeg" or "png"
```

Magic bytes for reference: JPEG = `FF D8 FF`, PNG = `89 50 4E 47 0D 0A 1A 0A`.

No external library needed.

---

## 2. JPEG Compression in Pure Go

### Key findings

- **Decode + re-encode is always lossy.** Even at quality=100, you get generation
  loss and files typically grow 2-3x (original was likely encoded at q=75-85).
- **Metadata is destroyed.** `jpeg.Decode` discards EXIF, ICC profiles, XMP —
  everything except pixel data. No way to carry it in `image.Image`.
- **Go's encoder is 15-30% larger than mozjpeg** at equivalent visual quality.
  No progressive JPEG, no trellis quantization, no optimized Huffman tables.
- **Visually lossless range:** quality 85-92 in Go's encoder. Default q=85
  is a good tradeoff for photographic content.
- **No pure-Go alternative matches mozjpeg.** `pixiv/go-libjpeg` is better
  but uses CGo.

### Implications for imgcrush MVP

For MVP, the pure-Go approach is honest and workable:
- Re-encode at configurable quality (default 80-85)
- Clearly document that metadata is stripped and there's generation loss
- The "skip if file would grow" safety net handles the q=100 trap
- Post-MVP: improve pure-Go compression (no shelling out — that's the
  whole point of imgcrush)

---

## 3. PNG Compression in Pure Go

### Key finding

**No single pure-Go library replicates pngquant or oxipng.** But there are
composable pieces.

### Useful pure-Go libraries

| Library | Technique | Status |
|---------|-----------|--------|
| **klauspost/compress** | Drop-in `compress/flate` replacement, ~2x faster, slightly smaller output at max level | Actively maintained (2026) |
| **esimov/colorquant** | Color quantization with dithering (Floyd-Steinberg, etc.) — closest to pngquant | Stable, v1.0.0 (2017) |
| **delthas/octreequant** | Fast octree color quantization — simpler API, faster than colorquant | Active (2024) |
| **soniakeys/quant** | Median-cut quantization (same algorithm family as pngquant) | Stable, v1.0.0 (2018) |

### What doesn't exist in pure Go

- No zopfli deflate port
- No oxipng equivalent (filter optimization + bit-depth reduction + chunk stripping)
- No PNG filter strategy optimizer library

### Recommended strategy for imgcrush

**MVP (lossless only):**
- Use stdlib `image/png` with `BestCompression`
- Strip ancillary PNG chunks (tEXt, iTXt, tIME, etc.) — simple chunk-level processing

**Post-MVP enhancements (still pure Go):**
- Swap deflate backend to `klauspost/compress/flate` for better compression
- Add filter optimization: try all 5 PNG filter types per row, pick smallest
- Add lossy mode: color quantization via `colorquant` or `octreequant` to
  reduce to 256-color paletted PNG (pngquant-like behavior)

---

## 4. Real-World Testing: imgcrush vs ImageOptim (2026-05-26)

### Test: 4 already-compressed JPEGs (~100-200 KB each)

**imgcrush (v0.1.0, Go stdlib, q=85):**
```
  skip  Black-Dachshund.jpg (already optimal)
  skip  marek-kowalczyk-kask.jpg (already optimal)
  skip  marek-kowalczyk-none.jpg (already optimal)
  skip  marek-kowalczyk-pileczki.jpg (already optimal)
  0 compressed, 4 skipped, 0 errors. Saved 0 B.
```

**ImageOptim (with JPEGmini Pro):**
```
  Black-Dachshund.jpg          204 KB -> 172 KB  (15.4%)
  marek-kowalczyk-kask.jpg     148 KB -> 112 KB  (24.1%)
  marek-kowalczyk-none.jpg     103 KB ->  74 KB  (28.5%)
  marek-kowalczyk-pileczki.jpg  99 KB ->  75 KB  (23.8%)
  TOTAL 553 KB -> 434 KB saving 120 KB (21.7%)
```

### Analysis

1. **The safety net works correctly.** imgcrush refuses to produce larger
   files. Go's stdlib re-encodes at q=85 produce output equal to or larger
   than the originals — so "already optimal" is the right call.

2. **The gap is exactly as predicted.** ImageOptim (via JPEGmini Pro, a
   commercial perceptual encoder) saves 15-28% on files that are already
   well-compressed. This matches the research finding that Go's encoder is
   15-30% less efficient than specialized tools (section 2).

3. **imgcrush's sweet spot is unoptimized images.** Test suite results on
   unoptimized files: 45-78% savings on JPEGs, 23-75% on PNGs. These are
   files straight from cameras, design tools, or generated at high quality.

4. **For already-optimized JPEGs, pure Go can't compete today.** The user
   could lower `--quality` (e.g., 70) to force savings, but that's a
   visible quality reduction — a fundamentally different tradeoff than what
   JPEGmini does perceptually.

5. **JPEGmini Pro is a commercial tool ($100+).** ImageOptim's advantage
   in this test comes from proprietary technology, not from the open-source
   tools it bundles. With just mozjpeg (no JPEGmini), the gap would be
   smaller.

### What this means for the roadmap

- The JPEG compression gap is the biggest weakness for imgcrush.
- Pure Go has no perceptual JPEG encoder today. Closing this gap requires
  either: (a) a pure-Go port of mozjpeg-style optimizations (progressive
  encoding, optimized Huffman tables, trellis quantization), or (b) waiting
  for the Go ecosystem to produce one.
- In the meantime, imgcrush remains the right tool for batch-compressing
  unoptimized images, and honest about its limits on pre-optimized ones.

### Benchmark: test suite images, imgcrush vs ImageOptim lossless (2026-05-26)

Run via `testdata/benchmark.sh` on the project's test images.
ImageOptim run without JPEGmini (lossless JPEG only).

```
FILE                             IMGCRUSH    IMGOPTIM    NOTES
detailed-golden-gate.jpg          17.5%       15.9%     imgcrush wins (lossy vs lossless)
large-cat-photo.jpg               77.8%       10.3%     imgcrush wins (lossy vs lossless)
small-sunrise.jpg                 45.5%        7.2%     imgcrush wins (lossy vs lossless)
large-gradient.png                75.3%       75.6%     tied
simple-few-colors.png             23.3%       80.3%     ImageOptim wins (palette optimization)
transparency-demo.png                0%          0%     tied (both skip)
transparent-logo.png              13.4%       68.7%     ImageOptim wins (palette optimization)
TOTAL                             71.4%       13.5%
```

**Critical insight: the JPEG comparison is apples vs oranges.**

- **imgcrush** re-encodes at q=85 — this is *lossy*. The 77.8% savings on
  `large-cat-photo.jpg` comes from reducing quality, not from smarter
  encoding. Metadata is also stripped.
- **ImageOptim** (without JPEGmini) does *lossless* JPEG optimization —
  it optimizes Huffman tables and converts to progressive. Identical pixels,
  just smaller encoding. Its 10.3% savings preserves exact visual quality.

These are fundamentally different operations. imgcrush gets bigger numbers
because it's doing a more aggressive (destructive) thing.

**PNG comparison is a fair fight — and ImageOptim wins on small/paletted PNGs.**

- On `large-gradient.png` (complex, many colors): tied at ~75%.
- On `simple-few-colors.png` (4 colors): ImageOptim 80% vs imgcrush 23%.
  ImageOptim's pngquant/oxipng detects the low color count and converts
  to an optimized palette. Go's stdlib just re-deflates.
- On `transparent-logo.png` (alpha + few colors): ImageOptim 69% vs 13%.
  Same story — palette optimization + better filter strategies.

**Conclusions:**
1. imgcrush is effective on unoptimized images but the savings come from
   lossy re-encoding, not from superior compression.
2. The PNG gap on palette-friendly images is large and real. This is the
   highest-value backlog item: color quantization + palette optimization.
3. For lossless JPEG optimization, imgcrush currently has nothing to offer.
   This is a hard gap with no pure-Go solution today.

---

## 6. Design Implications Summary

| Concern | Decision |
|---------|----------|
| Format detection | stdlib `image.DecodeConfig` — no deps |
| JPEG compression | stdlib `image/jpeg` re-encode at q=85 default |
| PNG compression | stdlib `image/png` with `BestCompression` |
| Metadata | Stripped on re-encode (document clearly) |
| Skip-if-larger | Essential safety net, especially for already-optimized files |
| External tools | None — pure Go only, no shelling out, ever |
| Better PNG | Post-MVP: klauspost/compress, filter optimization, color quantization |
