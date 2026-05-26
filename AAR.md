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
