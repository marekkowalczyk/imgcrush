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

## 5. Metadata Preservation: Pure-Go Library Research (2026-05-26)

### The problem

Go's `image.Decode` → `image.Encode` pipeline discards all metadata:
EXIF, ICC profiles, XMP, IPTC — everything except pixel data. To offer
a `--keep-metadata` flag, imgcrush needs to extract metadata from the
original file before compression and re-inject it into the output.

### How JPEG stores metadata

JPEG files store metadata in APP marker segments between SOI (Start of
Image, `FF D8`) and SOS (Start of Scan):

| Marker | Header | Content |
|--------|--------|---------|
| APP1 (`FFE1`) | `Exif\x00\x00` | EXIF data in TIFF format (camera, GPS, orientation, thumbnails) |
| APP1 (`FFE1`) | `http://ns.adobe.com/xap/1.0/\x00` | XMP (XML-based metadata) |
| APP2 (`FFE2`) | 16-byte ICC header | ICC color profile (critical for color accuracy) |
| APP13 (`FFED`) | `Photoshop 3.0\x00` | IPTC metadata (captions, keywords, copyright) |

All metadata segments appear before SOS. Each APP segment is limited
to 64 KB (but ICC/XMP can span multiple segments).

### How PNG stores metadata

PNG uses typed chunks (4-char code + length + data + CRC32):

| Chunk | Content |
|-------|---------|
| `eXIf` | EXIF data (same binary format as JPEG APP1, minus the wrapper) |
| `iCCP` | ICC color profile (zlib-compressed) |
| `iTXt` | UTF-8 text; XMP stored with keyword `XML:com.adobe.xmp` |
| `tEXt` | Plain-text key-value pairs (Title, Author, etc.) |
| `zTXt` | Compressed text (zlib-compressed tEXt) |

### Pure-Go library landscape

| Library | EXIF R | EXIF W | XMP R | XMP W | ICC R | ICC W | IPTC R | Maintained |
|---------|--------|--------|-------|-------|-------|-------|--------|------------|
| **dsoprea/go-exif v3** + go-jpeg-image-structure v2 + go-png-image-structure v2 | Yes | **Yes** | Yes | No | No | No | Yes | Dormant (2023) |
| **bep/imagemeta** | Yes | No | Yes | No | No | No | Yes | **Active** (used by Hugo) |
| **trimmer-io/go-xmp** | No | No | Yes | **Yes** | No | No | No | Dormant (2021) |
| **mandykoh/prism** | No | No | No | No | Yes | No | No | Active |
| **rwcarlsen/goexif** | Yes | No | No | No | No | No | No | Abandoned (2019) |
| **evanoberholster/imagemeta** | Yes | No | Yes | No | No | No | No | Active |
| **sfomuseum/go-exif-update** | No | **Yes** | No | No | No | No | No | Low (wraps dsoprea) |

**Key finding:** dsoprea's ecosystem is the only pure-Go option that can
write EXIF back into JPEG/PNG files. But it's dormant, has a complex API,
and doesn't handle ICC or XMP writing. No single library covers all
metadata types for both reading and writing.

The Go stdlib has no metadata support and no plans to add it (proposal
golang/go#33457 was frozen).

### Design decision: raw byte-level splicing (no dependencies)

Instead of depending on third-party libraries for metadata round-tripping,
imgcrush will do raw byte-level segment/chunk copying:

**For JPEG:**
1. Scan the original file for APP marker segments (APP1, APP2, APP13)
2. Save each metadata segment as an opaque byte blob
3. After `jpeg.Encode` produces the compressed pixel data, construct the
   output by writing: SOI → saved metadata segments → compressed data
   (from SOF onward)

**For PNG:**
1. Parse the original file's chunk stream
2. Save metadata chunks (`eXIf`, `iCCP`, `iTXt`, `tEXt`, `zTXt`) as
   opaque byte blobs
3. After `png.Encode`, parse the new PNG's chunks, insert saved metadata
   chunks after IHDR, recompute CRCs

**Why this approach:**
- **Zero dependencies.** Requires only knowledge of JPEG/PNG binary
  structure — well-documented, stable formats.
- **Aligns with project principles.** Pure Go, no external libraries,
  no maintenance risk from dormant third-party code.
- **Simplest correct solution.** For "preserve everything unchanged,"
  there's no need to parse individual EXIF tags — just copy raw bytes.
- **Metadata that was modified by compression.** Some EXIF fields become
  inaccurate after re-encoding (e.g., compression type, bits per sample,
  image dimensions if resized). These specific tags should be stripped or
  updated rather than blindly copied. The implementation should maintain
  a list of "invalidated by re-encoding" tags.

**What this doesn't cover (future work):**
- Granular per-field control (`--strip-gps`, `--strip-exif`, `--keep-icc`).
  These require parsing individual EXIF/XMP fields, which would warrant
  adding dsoprea/go-exif or similar as a dependency. Defer until needed.
- For metadata display/reporting, `bep/imagemeta` is the best option
  (actively maintained, tested against exiftool).

### Field-by-field analysis: what to DROP, KEEP, or UPDATE (2026-05-27)

After lossy re-encoding (decode to pixels, re-encode at target quality),
some metadata fields become lies. This section catalogs every relevant
field. The raw byte-level splicing approach copies entire EXIF/XMP/ICC/IPTC
segments as opaque blobs, so **for the MVP, field-level filtering applies
only to the EXIF APP1 segment** (which must be partially parsed to remove
invalid tags). XMP, ICC, and IPTC blocks are copied verbatim in phase 1.

#### IFD0 encoding tags — mostly NOT PRESENT or SAFE TO KEEP

ExifTool classifies many IFD0 structural tags as "Unsafe," but this is
about TIFF files, not JPEG EXIF. In real-world JPEG files from cameras,
most of these tags are either absent from IFD0 or unchanged by
JPEG→JPEG re-encoding:

**Not typically present in JPEG EXIF IFD0** (TIFF-specific):
StripOffsets (0x0111), RowsPerStrip (0x0116), StripByteCounts (0x0117),
JPEGQTables (0x0205), JPEGDCTables (0x0206), JPEGACTables (0x0207).
These are TIFF strip-based storage tags. If encountered, drop them —
but in practice they won't be there.

**Present but unchanged by JPEG→JPEG re-encoding** (safe to KEEP):

| Tag | ID | IFD | Why keep |
|-----|----|-----|----------|
| Compression | 0x0103 | IFD0 | Stays "JPEG" for JPEG→JPEG. Rarely in IFD0 anyway (usually IFD1 only). |
| BitsPerSample | 0x0102 | IFD0 | Stays 8. Go's encoder outputs 8-bit. |
| SamplesPerPixel | 0x0115 | IFD0 | Stays 3. Unchanged. |
| PhotometricInterpretation | 0x0106 | IFD0 | Stays YCbCr for JPEG. Unchanged. |
| YCbCrPositioning | 0x0213 | IFD0 | Informational in EXIF — JPEG markers control actual positioning, not this tag. Written by almost all cameras. Safe to keep. |
| YCbCrCoefficients | 0x0211 | IFD0 | Standard Rec.601 coefficients. Unchanged. |
| ReferenceBlackWhite | 0x0214 | IFD0 | Standard reference values. Unchanged. |

**Genuinely problematic** (DROP if present in IFD0):

| Tag | ID | IFD | Why drop |
|-----|----|-----|----------|
| JPEGInterchangeFormat | 0x0201 | IFD0 | Absolute byte offset pointer. Invalid in new file. But almost always in IFD1 only — covered by thumbnail drop below. |
| JPEGInterchangeFormatLength | 0x0202 | IFD0 | Paired with above. Same caveat. |
| YCbCrSubSampling | 0x0212 | IFD0 | Go's JPEG encoder may use different chroma subsampling than the original. Drop if present to avoid mismatch. |

**Bottom line:** The original DROP list of 16 tags was overly aggressive.
Most are either not present in JPEG EXIF, only in IFD1 (dropped with
thumbnail), or unchanged by re-encoding. The actual risk is small.

#### Thumbnail (IFD1) — DROP entirely

IFD1 contains a thumbnail JPEG (~160×120) plus its own structural tags
(Compression, ImageWidth, JPEGInterchangeFormat, etc.). The thumbnail
depicts the original encoding; offset pointers are invalid in the new
file. Drop the entire IFD1. Thumbnail regeneration is possible but
unnecessary for MVP.

Reference: jpegtran copies thumbnails verbatim with `-copy all`, but
jpegtran is lossless. Lossy re-encoding could produce visible
differences between thumbnail and main image.

#### Image identity — DROP

| Tag | ID | IFD | Why drop |
|-----|----|-----|----------|
| ImageUniqueID | 0xA420 | ExifIFD | 128-bit identifier for the original image. After lossy re-encoding, pixel data has changed — this is a different image. |

#### MakerNote — DROP (MVP)

| Tag | ID | IFD | Why drop |
|-----|----|-----|----------|
| MakerNote | 0x927C | ExifIFD | Opaque binary blob with manufacturer-specific internal structure. Many brands (Nikon, Panasonic, Leica) use absolute byte offsets that break when the EXIF block is relocated. ExifTool has 20+ years of brand-specific fixup code for this. A pure-Go tool cannot safely relocate MakerNotes without per-brand logic. Low value for imgcrush's use case. |

MakerNote is the highest-risk metadata to copy and the single most
common cause of corrupted metadata in image processing tools.

#### Software identification — UPDATE

| Tag | ID | IFD | Action |
|-----|----|-----|--------|
| Software | 0x0131 | IFD0 | Set to `"imgcrush <version>"`. Standard practice. |
| ProcessingSoftware | 0x000B | IFD0 | Set to `"imgcrush <version>"`. |

#### Modification timestamp — UPDATE

| Tag | ID | IFD | Action |
|-----|----|-----|--------|
| DateTime | 0x0132 | IFD0 | Set to current time. This is the file modification date. |
| SubSecTime | 0x9290 | ExifIFD | Update to match new DateTime. |
| OffsetTime | 0x9010 | ExifIFD | Update timezone for new DateTime. |

#### Image dimensions — KEEP (update if resizing is added)

| Tag | ID | IFD | Notes |
|-----|----|-----|-------|
| ImageWidth | 0x0100 | IFD0 | imgcrush does not resize; dimensions unchanged. |
| ImageLength | 0x0101 | IFD0 | Same. |
| PixelXDimension | 0xA002 | ExifIFD | Same. |
| PixelYDimension | 0xA003 | ExifIFD | Same. |

If resizing is added later, these must be updated to match the output.

#### Orientation — KEEP (special case)

| Tag | ID | IFD | Notes |
|-----|----|-----|-------|
| Orientation | 0x0112 | IFD0 | Go's `image/jpeg` Decode does NOT apply the Orientation tag — pixels are returned as stored. Since imgcrush does not rotate, the original Orientation value remains correct. If auto-orient is ever added (rotate pixels to match tag), update to 1 (Normal). |

#### Camera/shooting data — KEEP all

All ExifIFD shooting tags describe the original capture, unrelated to
encoding: Make (0x010F), Model (0x0110), ExposureTime (0x829A),
FNumber (0x829D), ISOSpeedRatings (0x8827), FocalLength (0x920A),
Flash (0x9209), LensMake (0xA433), LensModel (0xA434), WhiteBalance
(0xA403), SceneCaptureType (0xA406), ExifVersion (0x9000), and all
other ExifIFD tags not listed above.

#### GPS data — KEEP all

All GPS IFD tags (GPSLatitude 0x0002, GPSLongitude 0x0004, GPSAltitude
0x0006, GPSTimeStamp 0x0007, GPSDateStamp 0x001D, etc.) describe where
the photo was taken, not how it was encoded.

**Privacy note:** GPS is the highest privacy risk in metadata. imgcrush
should offer `--strip-gps` (future), but default to keeping it — a
compression tool should not silently strip data the user didn't ask to
remove.

#### Original timestamps — KEEP

| Tag | ID | IFD | Notes |
|-----|----|-----|-------|
| DateTimeOriginal | 0x9003 | ExifIFD | When the photo was taken. Never modify. |
| SubSecTimeOriginal | 0x9291 | ExifIFD | Fractional seconds. |
| DateTimeDigitized | 0x9004 | ExifIFD | When the image was digitized. Still true. |
| SubSecTimeDigitized | 0x9292 | ExifIFD | Fractional seconds. |
| OffsetTimeOriginal | 0x9011 | ExifIFD | Timezone for original. |
| OffsetTimeDigitized | 0x9012 | ExifIFD | Timezone for digitized. |

#### Copyright/authorship — KEEP all

Artist (0x013B), Copyright (0x8298), ImageDescription (0x010E),
UserComment (0x9286). Stripping copyright is legally problematic in
some jurisdictions. All descriptive, unrelated to encoding.

#### Color space — KEEP all

ColorSpace (0xA001), Gamma (0xA500), WhitePoint (0x013E),
PrimaryChromaticities (0x013F), TransferFunction (0x012D). Go's
decoder does not perform color space conversion — output pixels are
in the same color space as input. These remain correct.

#### ICC profile (APP2) — KEEP

The ICC profile describes the color space of the pixel data. Since Go
does not perform color management, the output remains in the same color
space. Dropping the ICC profile would cause wide-gamut images (AdobeRGB,
Display P3) to be misinterpreted as sRGB, producing wrong colors.

Copy the entire APP2 ICC segment verbatim.

#### XMP packet (APP1 XMP) — KEEP (copy verbatim for MVP)

Most XMP properties are descriptive (dc:title, dc:creator, dc:rights,
photoshop:DateCreated, Iptc4xmpCore:*) and remain valid.

Fields that should ideally be updated (phase 2, requires XML parsing):
- `xmp:ModifyDate` — set to current time
- `xmp:MetadataDate` — set to current time
- `xmp:CreatorTool` — set to `"imgcrush <version>"`
- `tiff:Software` — set to `"imgcrush <version>"`
- `xmpMM:InstanceID` — regenerate (new file instance)

For MVP, copy the entire XMP block unchanged. The stale fields are
non-critical — no application will break from an outdated ModifyDate
in XMP when the EXIF DateTime is correct.

#### IPTC (APP13) — KEEP (copy verbatim)

IPTC-IIM contains purely descriptive/editorial metadata: caption,
keywords, byline, credit, copyright, city, country, category. None
describe image encoding. Safe to copy unchanged.

### Implementation implications for the splicing approach

The raw byte-level splicing design (copy entire APP segments as blobs)
conflicts with the need to drop/update individual EXIF tags within the
APP1 EXIF segment. Resolution:

**Phase 1 (MVP):** Copy APP1/EXIF, APP1/XMP, APP2/ICC, and APP13/IPTC
segments verbatim from the original into the re-encoded output. Accept
that some EXIF fields (compression tags, Software, DateTime) will be
stale. This is the same behavior as jpegtran `-copy all` — and it's
what most users expect from a `--keep-metadata` flag.

**Phase 2:** Parse the EXIF APP1 segment for targeted changes:
1. Drop entire IFD1 (thumbnail) — broken offset pointers, stale pixels
2. Drop MakerNote (0x927C) — broken internal offsets after relocation
3. Drop ImageUniqueID (0xA420) — identity changed by re-encoding
4. Drop YCbCrSubSampling (0x0212) if present — may not match encoder
5. Update Software (0x0131) → `"imgcrush <version>"`
6. Update DateTime (0x0132) → current time
7. Keep everything else in IFD0 and ExifIFD — most encoding tags are
   either not present in JPEG EXIF or unchanged by JPEG→JPEG re-encode
8. Re-serialize the EXIF APP1 segment

This requires a minimal EXIF parser/writer — either a small in-house
implementation (TIFF IFD structure is well-documented) or vendoring
dsoprea/go-exif.

**Phase 3:** Parse XMP (XML) to update xmp:ModifyDate, xmp:CreatorTool,
etc. Standard Go `encoding/xml` suffices for targeted edits.

### Privacy-sensitive fields (future `--strip-private` flag)

These fields are technically correct after re-encoding but pose
privacy risks when images are shared publicly:

| Risk | Fields |
|------|--------|
| **High** | GPS Latitude/Longitude (0x0002/0x0004), XMP face regions (mwg-rs:*) |
| **Medium** | GPS Altitude (0x0006), BodySerialNumber (0xA431), CameraOwnerName (0xA430), MakerNote internals |
| **Low** | LensSerialNumber (0xA435), Artist (0x013B) |

Planned future flags:
- `--strip-metadata` / `-s` — strip all metadata
- `--strip-gps` — strip only GPS data
- `--strip-private` — strip GPS + serials + owner name + face regions
- `--keep-icc` — when combined with `--strip-metadata`, preserve ICC
  profile for color accuracy

### Reference: how other tools handle metadata

**exiftool `-TagsFromFile`**: Copies all "safe" writable tags by default.
Skips tags marked "Unsafe" (dimensions, compression, strip offsets,
ICC profile). With `-unsafe` flag copies structural tags too. MakerNotes
copied as monolithic blob with internal offset fixup logic.

**jpegtran `-copy all`**: Copies all APP markers verbatim, byte-for-byte.
Does not update any tags. Acceptable because jpegtran is lossless —
pixel data is identical.

### Six Hats analysis: writing pure-Go metadata libraries (2026-05-27)

Question: should imgcrush write its own pure-Go libraries to fill the
gaps in the Go metadata ecosystem (no library covers read+write for
EXIF, XMP, ICC, and IPTC)?

#### White Hat (facts)

- No pure-Go library exists that can both read and write EXIF, XMP,
  ICC, and IPTC across JPEG and PNG.
- dsoprea/go-exif is the closest — handles EXIF read/write but is
  dormant since 2023, no ICC/XMP write.
- The EXIF spec (CIPA DC-008) is public and stable — hasn't changed
  fundamentally in years.
- JPEG and PNG binary formats are well-documented and simple at the
  segment/chunk level.
- MakerNote handling requires per-brand logic; exiftool maintains
  ~150+ brand-specific modules built over 20+ years.
- imgcrush's actual need is narrow: splice opaque segments, drop IFD1,
  drop MakerNote, update ~3 tags.
- Go's stdlib has no plans to add metadata support (golang/go#33457
  frozen).

#### Red Hat (feelings)

- It feels like the right thing to do — filling a real gap in the Go
  ecosystem, owning your stack.
- But there's a nagging feeling of "this is how side projects die" —
  the library becomes the project and the tool never ships the feature.
- The pure-Go philosophy is emotionally core to imgcrush — depending on
  a dormant third-party library feels wrong.
- There's excitement in the craftsmanship of understanding a binary
  format deeply, but also dread at the long tail of broken files from
  obscure cameras.

#### Black Hat (risks)

- **Scope creep.** "Just a small EXIF writer" becomes a full metadata
  suite once real-world files hit it. Edge cases multiply: endianness,
  offset chains, IFD linking, non-conforming files from hundreds of
  camera models.
- **Maintenance burden.** A library has users, issues, expectations
  of stability. You're now maintaining two projects.
- **Quality risk.** A half-baked metadata writer that corrupts files is
  worse than no metadata writer. Silently breaking someone's photo
  archive is a serious failure mode.
- **Opportunity cost.** Time spent on metadata libraries is time not
  spent on imgcrush's bigger gaps (PNG palette optimization, progressive
  JPEG).
- **Testing surface.** You'd need a corpus of images from dozens of
  camera brands to validate.
- **MakerNote is a trap.** You either handle it (unbounded complexity)
  or don't (users complain about lost data).

#### Yellow Hat (benefits)

- **Full ownership of the dependency chain.** No dormant upstream, no
  surprise breakage, no API you can't change.
- **Exactly the right abstraction.** A general library tries to do
  everything; an imgcrush-internal package does exactly what's needed.
- **The Go ecosystem benefits.** If the internal package is
  well-designed, it could be extracted later and fill a genuine gap.
- **Deep understanding.** Building the parser means you truly understand
  what imgcrush is doing with metadata — no black-box surprises.
- **The narrow scope is genuinely tractable.** "Parse TIFF IFDs, drop
  entries by tag ID, rewrite offsets, serialize" is a few hundred lines
  of Go.
- **Phase 1 needs no library at all.** Opaque blob copying is pure byte
  work. The library is only needed for phase 2, so there's time to get
  it right.

#### Green Hat (alternatives)

- **Don't write a library at all.** Phase 1 (opaque blob copy) covers
  most users. Ship it, see if anyone actually asks for phase 2.
- **Fork dsoprea/go-exif.** It works, it's dormant not broken. Vendor
  it, strip what you don't need, fix what you do. Less work than
  starting from scratch; the hard offset-fixup logic is already written.
- **Hybrid approach.** Write a minimal internal "EXIF surgery" module
  (just IFD parsing, tag drop, tag update, reserialize) — not a general
  library, not extracted, not published. If it grows useful, reconsider.
- **Shell out to exiftool as an optional enhancer.** Violates the
  pure-Go principle for the core tool, but could be offered as
  `imgcrush --use-exiftool` for users who want perfect metadata
  handling. Honest about why.
- **Collaborate.** Contribute write support to bep/imagemeta (actively
  maintained, high quality). The maintainer may welcome it or reject it,
  but worth asking.
- **Wait.** The Go ecosystem is active. Someone may write this library.
  imgcrush ships phase 1 now, revisits in a year.

#### Blue Hat (process and summary)

The hats converge on a clear path:

1. **Ship phase 1 now** — opaque blob splicing, no library needed. This
   delivers the feature users want (`--keep-metadata`) with minimal risk.

2. **Don't write a general-purpose library.** The Black Hat risks are
   real and the Yellow Hat benefits come mostly from the narrow scope,
   not from building something general.

3. **For phase 2, choose between:** (a) a minimal internal EXIF surgery
   module — just enough to drop IFD1/MakerNote and update
   Software/DateTime, or (b) forking/vendoring dsoprea. Decision can
   wait until phase 1 is shipped and user feedback reveals whether
   phase 2 is even needed.

4. **Explicitly don't touch MakerNote.** Drop it, document why, move on.
   This is the single biggest scope trap.

5. **Consider reaching out to bep/imagemeta** about write support before
   building anything. A five-minute conversation could save months.

---

## 6. Design Implications Summary

| Concern | Decision |
|---------|----------|
| Format detection | stdlib `image.DecodeConfig` — no deps |
| JPEG compression | stdlib `image/jpeg` re-encode at q=85 default |
| PNG compression | stdlib `image/png` with `BestCompression` |
| Metadata | Stripped on re-encode; preserve via raw byte-level splicing (see section 5) |
| Skip-if-larger | Essential safety net, especially for already-optimized files |
| External tools | None — pure Go only, no shelling out, ever |
| Better PNG | Post-MVP: klauspost/compress, filter optimization, color quantization |
