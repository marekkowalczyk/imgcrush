# Next Session

- **v1.1.0 released** — concurrent processing, backup-from-memory fix, and `--outdir` collision guard shipped. Tag pushed, GitHub Actions release succeeded, binaries verified on GitHub Releases.
- **Homebrew tap — deferred.** `marekkowalczyk/homebrew-imgcrush` doesn't exist yet. When picked up: create the repo, add a `Formula/imgcrush.rb` pinned to the current release with SHA256 checksums, and consider wiring goreleaser's `brews` section so future tags auto-update the formula (needs a GitHub token with repo access to the tap, stored as a secret).
- **Check Go Report Card** — should now show A+ after v1.0.1 fixes. If not, hit "Refresh now" at https://goreportcard.com/report/github.com/marekkowalczyk/imgcrush
- **Post to r/golang and Go forum** — next step in the publication plan (see dev/publication.taskpaper)
- **Metadata preservation (phase 1)** — raw byte-level splicing of JPEG/PNG metadata segments, `--keep-metadata` flag. See dev/RESEARCH.md section 5.
