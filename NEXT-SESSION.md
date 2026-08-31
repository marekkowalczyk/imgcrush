# Next Session

- **v1.3.1 released** — omakase PNG (1.2.0), interspersed flags + skip
  cache, live per-file output, TTY `\r` batch progress, stable
  `cacheAlgoVersion`. Tag and GitHub Actions release should be on
  GitHub Releases.
- **Next PNG work** — klauspost/compress deflate swap; per-row PNG filter
  optimization (remaining SPEC Priority 1).
- **Homebrew tap — deferred.** `marekkowalczyk/homebrew-imgcrush` doesn't exist yet. When picked up: create the repo, add a `Formula/imgcrush.rb` pinned to the current release with SHA256 checksums, and consider wiring goreleaser's `brews` section so future tags auto-update the formula (needs a GitHub token with repo access to the tap, stored as a secret).
- **Check Go Report Card** — refresh after v1.3.1 if needed.
- **Post to r/golang and Go forum** — next step in the publication plan (see dev/publication.taskpaper)
- **Metadata preservation (phase 1)** — raw byte-level splicing of JPEG/PNG metadata segments, `--keep-metadata` flag. See dev/RESEARCH.md section 5.
