# Next Session

- **Tag and release v1.1.0** — concurrent processing, backup-from-memory fix, and `--outdir` collision guard are committed (f61b9cf) and tested. Verify `.goreleaser.yml` / release workflow still applies cleanly, then `git tag v1.1.0 && git push --tags`.
- **Check Go Report Card** — should now show A+ after v1.0.1 fixes. If not, hit "Refresh now" at https://goreportcard.com/report/github.com/marekkowalczyk/imgcrush
- **Post to r/golang and Go forum** — next step in the publication plan (see dev/publication.taskpaper)
- **Metadata preservation (phase 1)** — raw byte-level splicing of JPEG/PNG metadata segments, `--keep-metadata` flag. See dev/RESEARCH.md section 5.
