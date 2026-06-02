# After Action Review

Continuous improvement log. Each session ends with a brief review: what went well, what didn't, what to change. This is the POOGI (Process Of Ongoing Improvement) record for this project.

## 2026-05-26 — Project setup and MVP implementation

**What went well:**
- Thorough research phase before coding — format detection, JPEG/PNG tradeoffs, and ImageOptim internals were well understood before writing a line of code
- MVP came together cleanly in one pass — all 15 tests passed on first run
- Benchmark script immediately revealed the lossy-vs-lossless insight, which reframed the whole project narrative
- Safe-by-default design (skip threshold, backup, dry-run) was validated by real-world testing

**What didn't go well:**
- Suggested `cp` to `/usr/local/bin/` instead of `go install` — wrong idiom for a Go project
- Initially framed imgcrush JPEG results as "better than ImageOptim" without understanding the lossy-vs-lossless distinction — benchmark corrected this
- Research agents got blocked on web permissions, requiring manual WebFetch calls
- Some Wikimedia image downloads failed due to rate limiting; had to generate test images programmatically

**What we'll do differently:**
- Always use `go install` for Go projects — never suggest manual binary copying
- When comparing compression tools, always specify whether the operation is lossy or lossless — the distinction is fundamental
- Generate reproducible test images programmatically rather than relying on external downloads

## 2026-05-27 — Metadata preservation research and design

**What went well:**
- Thorough library survey covered 7 pure-Go libraries with a clear capability matrix — the landscape is now fully mapped
- User challenge on the DROP field list caught a real error: most "unsafe" EXIF tags are TIFF-specific, not present in JPEG EXIF, or unchanged by JPEG-to-JPEG re-encoding. The corrected list is much smaller and more honest.
- The Six Hats analysis produced a clear, actionable conclusion: ship phase 1 (opaque blob copy), defer the library decision
- Research was properly documented in-place (RESEARCH.md, SPEC.md) rather than left in conversation context

**What didn't go well:**
- Initially generated a 16-tag DROP list by uncritically accepting ExifTool's "Unsafe" classification, which is about TIFF files, not JPEG EXIF. Had to be corrected by the user.
- Attempted a "Pro-Con-Cloud" analysis without knowing Alan Barnard's actual method — hallucinated a generic pro/con/uncertainty framework instead of admitting ignorance upfront
- The metadata field research agent took a long time (~7 minutes) due to extensive web searching

**What we'll do differently:**
- When classifying EXIF tags, always check whether the tag is actually present in real-world JPEG files, not just whether the spec defines it
- When asked to apply a named framework or method, verify you know it correctly before attempting — admit uncertainty rather than improvise
- Consider the TIFF vs JPEG EXIF distinction whenever referencing ExifTool documentation — ExifTool's classifications are TIFF-centric
