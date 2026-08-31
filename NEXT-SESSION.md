# Next Session

## Completed last session

- [x] ~~PNG omakase palette path (logo-like images)~~
- [x] ~~Interspersed flags (flags after files; `--` only for dash-filenames)~~
- [x] ~~Content-hash skip cache with stable `cacheAlgoVersion`~~
- [x] ~~Live per-file results + TTY `\r` batch progress~~
- [x] ~~v1.3.1 tagged and released~~
- [x] ~~README product elevator statement~~

## Immediate

- Pick next compression work: **klauspost/compress deflate** or **per-row PNG filter optimization** (SPEC Priority 1). Nothing blocking; tree is clean and pushed.

## Follow-ups opened by this session

- None beyond the Priority 1 PNG items already on the backlog.

## Carried over

- **Homebrew tap — deferred.** Create `marekkowalczyk/homebrew-imgcrush` with a formula pinned to the current release; optionally wire goreleaser `brews`.
- **Go Report Card** — refresh after the v1.3.1 surface if desired.
- **Publication** — post to r/golang and the Go forum (see `dev/publication.taskpaper`).
- **Metadata preservation (phase 1)** — opaque JPEG/PNG segment splice + `--keep-metadata` (`dev/RESEARCH.md` §5).

## Current state

- CLI shows `1.3.1`; skip-cache algorithm epoch remains `1.3.0`.
- `go test ./...` passes (measured at close).
- README includes product elevator statement as vision North Star.

## Process notes

- Do not put CLI release version into the skip-cache fingerprint; bump `cacheAlgoVersion` only when crush behavior changes (also in CLAUDE.md).
- When the user says "performance," confirm bytes vs latency before proposing work.
- Prefer Unix-normal flag parsing (flags before/after files; `--` only when needed) over inventing invocation quirks.
